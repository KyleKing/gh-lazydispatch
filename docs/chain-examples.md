# Advanced Chain Examples

## Commitizen Bump with CI Gate

This example demonstrates a Chain that conditionally runs `commitizen bump` only after verifying all CI checks pass for the HEAD commit on main.

### Architecture

The Chain uses two workflows:
1. **ci-gate.yml** - Verifies CI status before proceeding
2. **version-bump.yml** - Runs commitizen bump and commits changes

### Workflow: `.github/workflows/ci-gate.yml`

```yaml
name: CI Gate

on:
  workflow_dispatch:
    inputs:
      ref:
        description: 'Git ref to check (branch/tag/commit)'
        required: true
        default: 'main'
      required_checks:
        description: 'Comma-separated list of required check names (leave empty for all)'
        required: false
        default: ''

jobs:
  verify-ci-status:
    runs-on: ubuntu-latest
    steps:
      - name: Check CI Status
        uses: actions/github-script@v7
        with:
          script: |
            const ref = '${{ inputs.ref }}';
            const requiredChecks = '${{ inputs.required_checks }}'
              .split(',')
              .map(s => s.trim())
              .filter(s => s.length > 0);

            // Get the commit SHA for the ref
            const { data: refData } = await github.rest.git.getRef({
              owner: context.repo.owner,
              repo: context.repo.repo,
              ref: `heads/${ref.replace('refs/heads/', '')}`
            });
            const sha = refData.object.sha;

            console.log(`Checking CI status for commit: ${sha}`);

            // Get all check runs for this commit
            const { data: checkRuns } = await github.rest.checks.listForRef({
              owner: context.repo.owner,
              repo: context.repo.repo,
              ref: sha,
              per_page: 100
            });

            // Get all status checks (for older CI systems)
            const { data: statuses } = await github.rest.repos.getCombinedStatusForRef({
              owner: context.repo.owner,
              repo: context.repo.repo,
              ref: sha
            });

            // Combine all checks
            const allChecks = [
              ...checkRuns.check_runs.map(check => ({
                name: check.name,
                status: check.status,
                conclusion: check.conclusion,
                type: 'check_run'
              })),
              ...statuses.statuses.map(status => ({
                name: status.context,
                status: status.state === 'pending' ? 'in_progress' : 'completed',
                conclusion: status.state,
                type: 'status'
              }))
            ];

            // Filter to required checks if specified
            const checksToVerify = requiredChecks.length > 0
              ? allChecks.filter(c => requiredChecks.includes(c.name))
              : allChecks;

            if (checksToVerify.length === 0) {
              core.setFailed('No CI checks found for this commit');
              return;
            }

            console.log(`Found ${checksToVerify.length} checks to verify:`);
            checksToVerify.forEach(check => {
              console.log(`  - ${check.name}: ${check.status} / ${check.conclusion}`);
            });

            // Check if all required checks have completed
            const incompleteChecks = checksToVerify.filter(c =>
              c.status !== 'completed' && c.conclusion !== 'success'
            );

            if (incompleteChecks.length > 0) {
              const checkList = incompleteChecks.map(c => c.name).join(', ');
              core.setFailed(`CI checks incomplete or pending: ${checkList}`);
              core.setOutput('incomplete_checks', checkList);
              return;
            }

            // Check if any checks failed
            const failedChecks = checksToVerify.filter(c =>
              c.conclusion !== 'success' && c.conclusion !== 'skipped'
            );

            if (failedChecks.length > 0) {
              const checkList = failedChecks.map(c =>
                `${c.name} (${c.conclusion})`
              ).join(', ');
              core.setFailed(`CI checks failed: ${checkList}`);
              core.setOutput('failed_checks', checkList);
              return;
            }

            console.log('✓ All CI checks passed');
            core.setOutput('sha', sha);
            core.setOutput('checks_verified', checksToVerify.length);

      - name: Summary
        if: always()
        run: |
          echo "### CI Gate Results" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "**Ref:** ${{ inputs.ref }}" >> $GITHUB_STEP_SUMMARY
          echo "**Status:** ${{ job.status }}" >> $GITHUB_STEP_SUMMARY
```

### Workflow: `.github/workflows/version-bump.yml`

```yaml
name: Version Bump

on:
  workflow_dispatch:
    inputs:
      bump_type:
        description: 'Version bump type (leave empty for auto)'
        required: false
        type: choice
        options:
          - ''
          - 'patch'
          - 'minor'
          - 'major'
        default: ''
      prerelease:
        description: 'Prerelease identifier (e.g., alpha, beta, rc)'
        required: false
        default: ''
      push:
        description: 'Push changes and tags to remote'
        required: true
        type: boolean
        default: true

jobs:
  bump-version:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Setup Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'

      - name: Install commitizen
        run: pip install commitizen

      - name: Configure git
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"

      - name: Bump version
        id: bump
        run: |
          ARGS=()
          if [ -n "${{ inputs.bump_type }}" ]; then
            ARGS+=(--increment "${{ inputs.bump_type }}")
          fi
          if [ -n "${{ inputs.prerelease }}" ]; then
            ARGS+=(--prerelease "${{ inputs.prerelease }}")
          fi

          cz bump "${ARGS[@]}" --yes || {
            echo "No version bump needed"
            echo "bumped=false" >> $GITHUB_OUTPUT
            exit 0
          }

          echo "bumped=true" >> $GITHUB_OUTPUT
          echo "new_version=$(cz version --project)" >> $GITHUB_OUTPUT

      - name: Push changes
        if: steps.bump.outputs.bumped == 'true' && inputs.push
        run: |
          git push --follow-tags

      - name: Summary
        if: always()
        run: |
          echo "### Version Bump Results" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          if [ "${{ steps.bump.outputs.bumped }}" == "true" ]; then
            echo "**Status:** ✓ Version bumped" >> $GITHUB_STEP_SUMMARY
            echo "**New Version:** ${{ steps.bump.outputs.new_version }}" >> $GITHUB_STEP_SUMMARY
          else
            echo "**Status:** No version bump needed" >> $GITHUB_STEP_SUMMARY
          fi
```

### Chain Definition: `.github/lazydispatch.yml`

```yaml
version: 2
chains:
  release-bump:
    description: Bump version with commitizen after CI passes on main
    variables:
      - name: target_branch
        type: string
        description: Branch to verify CI status
        default: main
        required: true

      - name: required_checks
        type: string
        description: Required check names (comma-separated, empty for all)
        default: ''
        required: false

      - name: bump_type
        type: choice
        description: Version bump type (empty for auto-detect)
        options: ['', 'patch', 'minor', 'major']
        default: ''
        required: false

      - name: prerelease
        type: string
        description: Prerelease identifier (e.g., alpha, beta, rc)
        default: ''
        required: false

      - name: push
        type: boolean
        description: Push changes and tags to remote
        default: 'true'
        required: true

    steps:
      - workflow: ci-gate.yml
        wait_for: success
        on_failure: abort
        inputs:
          ref: '{{ var.target_branch }}'
          required_checks: '{{ var.required_checks }}'

      - workflow: version-bump.yml
        wait_for: success
        on_failure: abort
        inputs:
          bump_type: '{{ var.bump_type }}'
          prerelease: '{{ var.prerelease }}'
          push: '{{ var.push }}'
```

### Usage

1. **Basic usage** (auto-detect bump, verify all CI on main):
   ```
   lazydispatch
   > Select "release-bump" chain
   > Accept defaults
   ```

2. **Force minor bump with specific checks**:
   ```
   Variables:
   - target_branch: main
   - required_checks: "tests,lint,build"
   - bump_type: minor
   - prerelease: (empty)
   - push: true
   ```

3. **Create beta prerelease**:
   ```
   Variables:
   - target_branch: develop
   - required_checks: (empty - verify all)
   - bump_type: (empty - auto)
   - prerelease: beta
   - push: true
   ```

### Behavior

**Step 1: CI Gate**
- Fetches all check runs and statuses for HEAD of target branch
- Filters to required checks if specified
- Fails if any checks are incomplete or failed
- Outputs detailed check information to job summary

**Step 2: Version Bump**
- Only runs if CI gate passes (`wait_for: success`)
- Runs `commitizen bump` with specified options
- Commits and tags the version bump
- Pushes if `push: true`
- Gracefully handles "no bump needed" case

**Failure Handling**
- If CI gate fails: Chain aborts, no version bump attempted
- If version bump fails: Chain fails, can be rerun after fixing issues
- All failures include detailed error messages and logs

### Integration with Error Alerting

When the CI gate fails, users will see:
- Clear error message identifying which checks failed
- Direct link to the workflow run logs
- Link to the commit on GitHub showing CI status
- Option to view full check details locally
- Suggestion to rerun after CI passes

See "Chain Failure Alerting" section below for implementation details.
