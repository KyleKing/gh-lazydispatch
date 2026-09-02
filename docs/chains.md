# Workflow chains

![chains demo](https://raw.githubusercontent.com/KyleKing/gh-lazydispatch/main/.github/assets/chains-demo.gif)

A chain runs several workflows in sequence, each with its own wait condition and failure handling. Define chains in `.github/lazydispatch.yml`:

```yaml
version: 1
chains:
  deploy-all:
    description: Build, test, and deploy to all environments
    steps:
      - workflow: build.yml
        wait_for: success      # Wait for successful completion (default)
        on_failure: abort      # Stop chain on failure (default)
      - workflow: test.yml
        wait_for: completion   # Wait for any completion (success or failure)
        on_failure: continue   # Continue even if this step fails
      - workflow: deploy.yml
        wait_for: none         # Don't wait, dispatch immediately
        inputs:
          environment: production
          version: v1.0.0

  quick-test:
    description: Run tests with default settings
    steps:
      - workflow: test.yml
```

## Step options

| Option       | Values                           | Default    | Meaning                                        |
| ------------ | -------------------------------- | ---------- | ----------------------------------------------- |
| `source`     | `dispatch`, `existing`           | `dispatch` | Start a fresh run, or adopt one already going    |
| `wait_for`   | `success`, `completion`, `none`  | `success`  | When to move to the next step                   |
| `on_failure` | `abort`, `skip`, `continue`      | `abort`    | What to do when the step fails                   |
| `inputs`     | map                              | none       | Override the workflow's inputs                   |

`source: existing` attaches to the newest queued or in-progress run of the step's workflow on the branch, instead of dispatching a new one. It is for waiting on a build that is already going rather than starting a second one:

```yaml
steps:
  - workflow: docker-build.yml
    source: existing
    wait_for: success
  - workflow: deploy-prod.yml
```

Where nothing is running, the step fails and says so rather than dispatching instead: a step that sometimes starts a production build and sometimes adopts one is a step nobody can read. The confirmation modal names the run it will adopt, with its ID, age, and URL, once it resolves.

## Running one

Press `tab` to focus the right panel, `l` until the Chains tab is showing, then `j`/`k` to pick a chain and `enter` to run it. `C` runs a chain directly.

While a chain runs, the status bar reads `Chain: name (step/total)`. Press `v` after it finishes or fails to open the [log viewer](./interface.md#log-viewer), which starts filtered to errors when the chain failed.

Each step needs the named workflow to exist and to accept `workflow_dispatch` on the branch you dispatched from, otherwise the chain reports that step as failed.

[Chain examples](./chain-examples.md) has worked configurations.
