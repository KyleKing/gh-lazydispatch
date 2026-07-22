#!/usr/bin/env bash
# Live integration test for the dispatch path.
#
# Creates a throwaway GitHub repository, pushes a workflow_dispatch workflow
# that echoes its inputs, drives the TUI headlessly against it (go test -tags
# live), and verifies that the values the TUI sent are the values the workflow
# received. The scratch repository and the temp directory are always destroyed
# on exit unless KEEP=1.
#
# This consumes GitHub Actions minutes and creates a real repository. See
# docs/live-testing.md.

set -euo pipefail

readonly REPO_PREFIX="gh-ld-livetest-"
readonly WORKFLOW_FILE="livetest.yml"
readonly BRANCH="main"

ASSUME_YES="${GH_LD_LIVE_TEST:-0}"
KEEP="${KEEP:-0}"

SCRATCH_REPO=""
TMP_DIR=""
REPO_CREATED=0

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly PROJECT_ROOT

log() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
die() {
    printf '\n\033[31mERROR: %s\033[0m\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'EOF'
Usage: scripts/live-test.sh [--yes] [--keep] [--help]

  --yes    Confirm creating and deleting a real GitHub repository.
           Equivalent to GH_LD_LIVE_TEST=1.
  --keep   Leave the scratch repository and temp directory in place.
           Equivalent to KEEP=1.

This test creates a private repository named gh-ld-livetest-<epoch>-<rand>
under your account, dispatches a workflow in it, and then deletes it. It
consumes GitHub Actions minutes.
EOF
}

for arg in "$@"; do
    case "$arg" in
        --yes) ASSUME_YES=1 ;;
        --keep) KEEP=1 ;;
        -h | --help)
            usage
            exit 0
            ;;
        *) die "unknown argument: $arg (try --help)" ;;
    esac
done

if [[ $ASSUME_YES != "1" ]]; then
    cat <<EOF
This is a LIVE test. Running it will:

  CREATE   a private GitHub repository named ${REPO_PREFIX}<epoch>-<rand>
           under the account that 'gh auth status' reports
  PUSH     a workflow_dispatch workflow that echoes its inputs
  DISPATCH that workflow for real, consuming GitHub Actions minutes
  DOWNLOAD the run logs to verify the inputs round-tripped
  DELETE   the scratch repository and a temp working directory

It never touches an existing repository and never writes outside of a
mktemp -d working directory.

Re-run with --yes (or GH_LD_LIVE_TEST=1) to proceed.
EOF
    exit 1
fi

# --- cleanup -----------------------------------------------------------------

cleanup() {
    local status=$?

    if [[ $KEEP == "1" ]]; then
        log "KEEP=1: leaving scratch resources in place"
        [[ -n $SCRATCH_REPO ]] && info "repo: $SCRATCH_REPO"
        [[ -n $TMP_DIR ]] && info "dir:  $TMP_DIR"
        return $status
    fi

    # Refuse to delete anything that is not demonstrably ours.
    if [[ $REPO_CREATED == "1" && -n $SCRATCH_REPO ]]; then
        local name="${SCRATCH_REPO##*/}"
        if [[ $name != "${REPO_PREFIX}"* ]]; then
            printf '\n\033[31mREFUSING to delete %s: name lacks the %s prefix\033[0m\n' \
                "$SCRATCH_REPO" "$REPO_PREFIX" >&2
        else
            log "Deleting scratch repo $SCRATCH_REPO"
            if ! gh repo delete "$SCRATCH_REPO" --yes; then
                printf '\n\033[31mLEAKED REPOSITORY: %s could not be deleted. Remove it manually.\033[0m\n' \
                    "$SCRATCH_REPO" >&2
            fi
        fi
    fi

    if [[ -n $TMP_DIR && -d $TMP_DIR ]]; then
        case "$TMP_DIR" in
            /tmp/* | /var/folders/* | "${TMPDIR:-/nonexistent}"*)
                cd /
                rm -rf "$TMP_DIR"
                ;;
            *) printf '\n\033[31mREFUSING to remove %s: not under a temp directory\033[0m\n' "$TMP_DIR" >&2 ;;
        esac
    fi

    return $status
}
trap cleanup EXIT

# --- preflight ---------------------------------------------------------------

log "Preflight"

for bin in gh git go; do
    command -v "$bin" >/dev/null 2>&1 || die "$bin is not on PATH"
done

gh auth status >/dev/null 2>&1 || die "gh is not authenticated; run 'gh auth login'"

OWNER="$(gh api user --jq .login)"
[[ -n $OWNER ]] || die "could not determine the authenticated GitHub account"
info "account: $OWNER"

# Classic OAuth tokens advertise their scopes; fine-grained PATs do not, so for
# those the only honest check is to create and delete a probe repository before
# committing to the expensive part of the run.
TOKEN_SCOPES="$(gh auth status 2>&1 | sed -n 's/.*Token scopes: //p' | tr -d "'" || true)"

if [[ -n $TOKEN_SCOPES ]]; then
    info "token scopes: $TOKEN_SCOPES"
    case ",${TOKEN_SCOPES// /}," in
        *,delete_repo,*) ;;
        *) die "token lacks the 'delete_repo' scope; a failed run would leak a repository.
       Run: gh auth refresh -h github.com -s delete_repo" ;;
    esac
else
    info "token scopes are not introspectable (fine-grained PAT); probing delete permission"
    PROBE_REPO="${OWNER}/${REPO_PREFIX}probe-$(date +%s)-$RANDOM"
    gh repo create "$PROBE_REPO" --private >/dev/null ||
        die "cannot create repositories with this token (needs Administration: write)"
    if ! gh repo delete "$PROBE_REPO" --yes >/dev/null 2>&1; then
        printf '\n\033[31mLEAKED REPOSITORY: %s\033[0m\n' "$PROBE_REPO" >&2
        die "token cannot delete repositories; aborting before creating the real scratch repo.
       Classic tokens need the 'delete_repo' scope; fine-grained PATs need Administration: write."
    fi
    info "delete permission confirmed"
fi

log "Building the binary under test from source"
TMP_DIR="$(mktemp -d)"
readonly BIN="$TMP_DIR/gh-lazydispatch"
(cd "$PROJECT_ROOT" && go build -o "$BIN" ./cmd/gh-lazydispatch)
info "$("$BIN" --version)"

# --- scratch repo ------------------------------------------------------------

SCRATCH_NAME="${REPO_PREFIX}$(date +%s)-$RANDOM"
SCRATCH_REPO="${OWNER}/${SCRATCH_NAME}"
CLONE_DIR="$TMP_DIR/$SCRATCH_NAME"

log "Creating scratch repo $SCRATCH_REPO"
gh repo create "$SCRATCH_REPO" --private --add-readme >/dev/null
REPO_CREATED=1

gh repo clone "$SCRATCH_REPO" "$CLONE_DIR" -- --quiet
cd "$CLONE_DIR"

# Every destructive step from here on assumes we are inside the temp tree.
[[ $PWD == "$TMP_DIR"/* ]] || die "cwd $PWD escaped the temp directory $TMP_DIR"

git checkout -q -B "$BRANCH"

mkdir -p .github/workflows
cat >".github/workflows/$WORKFLOW_FILE" <<'EOF'
name: LiveTest Echo
on:
  workflow_dispatch:
    inputs:
      level:
        description: Log level for the echoed message
        type: choice
        default: info
        options:
          - debug
          - info
          - warn
      message:
        description: Message echoed back by the job
        type: string
        default: hello
      verbose:
        description: Echo the full input payload
        type: boolean
        default: false
jobs:
  echo:
    runs-on: ubuntu-latest
    steps:
      - name: Echo dispatch inputs
        env:
          LEVEL: ${{ inputs.level }}
          MESSAGE: ${{ inputs.message }}
          VERBOSE: ${{ inputs.verbose }}
        run: |
          echo "LIVETEST_LEVEL=$LEVEL"
          echo "LIVETEST_MESSAGE=$MESSAGE"
          echo "LIVETEST_VERBOSE=$VERBOSE"
EOF

git add .github/workflows/"$WORKFLOW_FILE"
git -c user.name="gh-lazydispatch live test" \
    -c user.email="live-test@localhost" \
    commit -q -m "test: add live dispatch workflow"
git push -q origin "$BRANCH"

log "Enabling Actions on the scratch repo"
gh api -X PUT "repos/$SCRATCH_REPO/actions/permissions" \
    -f enabled=true -f allowed_actions=all >/dev/null ||
    die "could not enable GitHub Actions on $SCRATCH_REPO (org policy may forbid it)"

if [[ "$(gh api "repos/$SCRATCH_REPO/actions/permissions" --jq .enabled)" != "true" ]]; then
    die "GitHub Actions is not enabled on $SCRATCH_REPO; a dispatch cannot run"
fi

log "Waiting for GitHub to register the workflow"
registered=0
for _ in $(seq 1 30); do
    if gh workflow list --repo "$SCRATCH_REPO" --json path --jq '.[].path' 2>/dev/null |
        grep -q "$WORKFLOW_FILE"; then
        registered=1
        break
    fi
    sleep 2
done
[[ $registered == 1 ]] || die "workflow $WORKFLOW_FILE never appeared in $SCRATCH_REPO"

# --- drive the TUI -----------------------------------------------------------

NONCE="$(date +%s)-$RANDOM"
RESULT_FILE="$TMP_DIR/result.env"

log "Driving the TUI headlessly (go test -tags live)"
(
    cd "$PROJECT_ROOT"
    GH_LD_LIVE_REPO="$SCRATCH_REPO" \
        GH_LD_LIVE_DIR="$CLONE_DIR" \
        GH_LD_LIVE_WORKFLOW="$WORKFLOW_FILE" \
        GH_LD_LIVE_BRANCH="$BRANCH" \
        GH_LD_LIVE_NONCE="$NONCE" \
        GH_LD_LIVE_RESULT="$RESULT_FILE" \
        XDG_CACHE_HOME="$TMP_DIR/cache" \
        go test -tags live -count=1 -v -timeout 10m -run TestLiveDispatch ./internal/app/
)

[[ -f $RESULT_FILE ]] || die "the live test did not report a dispatched run"
# shellcheck disable=SC1090
source "$RESULT_FILE"
: "${run_id:?run_id missing from result file}"

log "Confirming the run via gh run list"
gh run list --repo "$SCRATCH_REPO" --workflow "$WORKFLOW_FILE" \
    --json databaseId,event,headBranch,status,conclusion,url

gh run list --repo "$SCRATCH_REPO" --workflow "$WORKFLOW_FILE" --json databaseId --jq '.[].databaseId' |
    grep -qx "$run_id" || die "run $run_id is not in gh run list output"

# --- verify the workflow received what the TUI sent --------------------------

log "Waiting for run $run_id to finish"
gh run watch "$run_id" --repo "$SCRATCH_REPO" --exit-status >/dev/null ||
    die "run $run_id did not succeed; inspect ${run_url:-}"

log "Verifying echoed inputs in the run logs"
LOGS="$TMP_DIR/run.log"
gh run view "$run_id" --repo "$SCRATCH_REPO" --log >"$LOGS"

failures=0
# shellcheck disable=SC2154 # input_* come from the sourced result file
for pair in "LEVEL=$input_level" "MESSAGE=$input_message" "VERBOSE=$input_verbose"; do
    if grep -q "LIVETEST_$pair" "$LOGS"; then
        info "ok: LIVETEST_$pair"
    else
        printf '\033[31m    MISSING: LIVETEST_%s\033[0m\n' "$pair" >&2
        failures=$((failures + 1))
    fi
done

[[ $failures == 0 ]] || die "$failures input(s) did not round-trip through the workflow"

log "PASS"
info "repo: https://github.com/$SCRATCH_REPO"
info "run:  ${run_url:-}"
