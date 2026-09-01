#!/usr/bin/env bash
# Script to check for unsafe test patterns that could mutate real GitHub resources
# This is a static analysis complement to the runtime safety checks in internal/exec

set -euo pipefail

# Find test files that call runner.Execute or runner.ExecuteAndGetRunID
# without first calling runner.SetExecutor
echo "Checking for unsafe test patterns..."

violations=0

# Check for direct RealExecutor usage in test files.
#
# A file that imports aragonite/ghcassette is exempt: the cassette puts a stand-in
# gh ahead of the real one on PATH, so a real executor reaches the recording and
# never GitHub. That is the whole point of recording against the real binary.
while IFS= read -r -d '' test_file; do
    if ! grep -q "exec\.NewRealExecutor()" "$test_file"; then
        continue
    fi

    if grep -q "aragonite/ghcassette" "$test_file"; then
        continue
    fi

    echo "ERROR: $test_file uses exec.NewRealExecutor() outside a cassette"
    echo "  Use exec.MockExecutor, or route gh through aragonite/ghcassette"
    violations=$((violations + 1))
done < <(find . -name "*_test.go" -not -path "./vendor/*" -print0)

# Check for runner.Execute calls in test files without SetExecutor
# This is a heuristic check - not perfect but catches common mistakes
# Process substitution rather than a pipe so the counter survives the loop
while IFS= read -r -d '' test_file; do
    if grep -q "runner\.Execute\|runner\.ExecuteAndGetRunID" "$test_file"; then
        if ! grep -q "runner\.SetExecutor\|runner\.ExecuteWithExecutor\|runner\.ExecuteAndGetRunIDWithExecutor" "$test_file"; then
            echo "WARNING: $test_file uses runner.Execute* but may not set up mocks"
            echo "  Ensure you call runner.SetExecutor() or use ...WithExecutor() functions"
            violations=$((violations + 1))
        fi
    fi
done < <(find . -name "*_test.go" -not -path "./vendor/*" -print0)

if [ $violations -eq 0 ]; then
    echo "✓ No unsafe test patterns detected"
    exit 0
else
    echo ""
    echo "Found $violations potential issue(s)"
    echo "Note: Runtime safety checks in exec.RealExecutor will catch actual violations"
    exit 1
fi
