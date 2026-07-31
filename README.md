# gh-lazydispatch

![demo](https://raw.githubusercontent.com/KyleKing/gh-lazydispatch/main/.github/assets/demo.gif)

Pick a GitHub Actions workflow, fill in its `workflow_dispatch` inputs, and run it without leaving the terminal. Chain several workflows into one sequence and read the logs as they stream back.

## Install

```bash
# GitHub CLI extension
gh extension install kyleking/gh-lazydispatch
# Go
go install github.com/kyleking/gh-lazydispatch/cmd/gh-lazydispatch@latest
```

## Quick start

From a git repository that has workflows:

```bash
cd your-project
gh lazydispatch
```

It finds every workflow with a `workflow_dispatch` trigger and lists them. `tab` moves between panes, `enter` runs the highlighted workflow, and `?` opens the keymap.

## What it does not do

- Send `repository_dispatch` events. It reads `workflow_dispatch` triggers only, so use gh-dispatch for the other kind
- Run from a script. The only flags are `-h` and `-v`, so use `gh workflow run` in CI
- Run Actions locally. That is what act is for
- Edit or create workflow files. It reads them and dispatches them
- Work outside a repository. It discovers workflows from the checkout you are standing in

Full docs: [./docs](./docs)

## License

MIT
