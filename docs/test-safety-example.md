# Test Safety: Example Violations and How They're Caught

This document demonstrates how the test safety mechanisms prevent accidental mutation of GitHub resources.

## Example Violation

Here's what happens if you write a test that doesn't properly mock the executor:

```go
// ❌ UNSAFE TEST - Will panic at runtime
func TestMyFeature_UNSAFE(t *testing.T) {
    client := github.NewClient("owner/repo")

    // This will PANIC with safety violation!
    // Because it tries to run a real gh workflow run command during tests
    runID, err := runner.ExecuteAndGetRunID(runner.RunConfig{
        Workflow: "test.yml",
        Branch:   "main",
    }, client)

    // Test never gets here because it panics above
}
```

**Runtime error:**
```
panic: SAFETY VIOLATION: Attempted to run mutation command during test: gh workflow run test.yml --ref main
This could modify real GitHub resources!
Use exec.MockExecutor or runner.SetExecutor() in your test instead.
```

## Correct Approach

### Option 1: Using SetExecutor (Recommended for integration tests)

```go
// ✅ SAFE TEST - Properly mocked
func TestMyFeature_Safe(t *testing.T) {
    // Setup mock executor
    mockExec := exec.NewMockExecutor()
    mockExec.AddCommand("gh", []string{"workflow", "run", "test.yml", "--ref", "main"}, "", "", nil)
    runner.SetExecutor(mockExec)
    defer runner.SetExecutor(nil) // Clean up

    client := &mockGitHubClient{
        latestID: 12345,
    }

    // Now this is safe - uses the mock
    runID, err := runner.ExecuteAndGetRunID(runner.RunConfig{
        Workflow: "test.yml",
        Branch:   "main",
    }, client)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if runID != 12345 {
        t.Errorf("got runID %d, want 12345", runID)
    }
}
```

### Option 2: Using ...WithExecutor Functions (Recommended for unit tests)

```go
// ✅ SAFE TEST - Using explicit executor parameter
func TestMyFeature_UnitTest(t *testing.T) {
    mockExec := &mockCommandExecutor{} // Test-specific mock
    client := &mockGitHubClient{latestID: 12345}

    // Pass executor explicitly
    runID, err := runner.ExecuteAndGetRunIDWithExecutor(
        runner.RunConfig{
            Workflow: "test.yml",
            Branch:   "main",
        },
        client,
        mockExec,
    )

    // Assertions...
}
```

## Static Analysis Detection

The `./scripts/check-test-safety.sh` script will warn about patterns like:

```bash
$ ./scripts/check-test-safety.sh
WARNING: internal/mypackage/myfeature_test.go uses runner.Execute* but may not set up mocks
  Ensure you call runner.SetExecutor() or use ...WithExecutor() functions
```

## Mutation Commands Blocked in Tests

The following `gh` commands will panic if executed during tests:

- `gh workflow run` - Dispatch workflows
- `gh issue create/edit/close/delete` - Issue operations
- `gh pr create/merge/close/edit` - Pull request operations
- `gh run cancel/rerun` - Workflow run mutations
- `gh release create/delete` - Release management
- `gh repo create/delete` - Repository management
- `gh secret set/delete` - Secret management
- `gh variable set/delete` - Variable management
- `gh label create/delete` - Label management
- And more (see `internal/exec/executor.go:isMutationCommand`)

## Read-Only Commands Allowed in Tests

These commands are safe and won't panic:

- `gh api repos/owner/repo/actions/runs` - API read calls
- `gh run view 12345` - View workflow run
- `gh run list` - List workflow runs
- `gh run watch` - Watch workflow run
- `gh --version` - Version check
- `gh auth status` - Auth check
- Non-gh commands like `git`, `echo`, etc.

## Why Multiple Safety Layers?

1. **Runtime Check** - Catches violations immediately with clear error message
2. **Static Analysis** - Detects issues before runtime (CI integration)
3. **Test Design** - Patterns encourage proper mocking by default

This defense-in-depth approach ensures no real GitHub resources are accidentally modified during test runs.
