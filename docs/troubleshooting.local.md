# Troubleshooting (project-specific)

Entries specific to gh-lazydispatch. The template never renders this file, so it
survives `copier update` untouched.

## No workflows listed

No workflows are listed because the repository has none that declare a `workflow_dispatch` trigger, or because the working directory is not a git repository. Add the trigger to the workflow, or `cd` into the checkout first.

## Dispatch or log viewing fails on authentication

Both go through the `gh` CLI. Run `gh auth status`, then `gh auth login` if needed.

## A chain step fails immediately

The named workflow does not exist, or does not accept `workflow_dispatch` on the branch you dispatched from. The error names both the workflow and the branch.

## `brew install` fails

No cask has published to `kyleking/homebrew-tap` for gh-lazydispatch yet, even though `.goreleaser.yml` is wired to push one. Install through `gh extension install` or `go install` instead.
