#!/usr/bin/env bash
# Script to check for unsafe test patterns that could mutate real GitHub resources
# This is a static analysis complement to the runtime safety checks in internal/exec

set -euo pipefail

# Find test files that call runner.Execute or runner.ExecuteAndGetRunID
# without first calling runner.SetExecutor
echo "Checking for unsafe test patterns..."

violations=0

# Check for direct RealExecutor usage in test files
if grep -r "exec\.NewRealExecutor()" --include="*_test.go" .; then
    echo "ERROR: Found direct RealExecutor usage in test files (above)"
    echo "Use exec.MockExecutor instead"
    ((violations++))
fi

# Check for runner.Execute calls in test files without SetExecutor
# This is a heuristic check - not perfect but catches common mistakes
for test_file in $(find . -name "*_test.go" -not -path "./vendor/*"); do
    if grep -q "runner\.Execute\|runner\.ExecuteAndGetRunID" "$test_file"; then
        if ! grep -q "runner\.SetExecutor\|runner\.ExecuteWithExecutor\|runner\.ExecuteAndGetRunIDWithExecutor" "$test_file"; then
            echo "WARNING: $test_file uses runner.Execute* but may not set up mocks"
            echo "  Ensure you call runner.SetExecutor() or use ...WithExecutor() functions"
            ((violations++))
        fi
    fi
done

if [ $violations -eq 0 ]; then
    echo "✓ No unsafe test patterns detected"
    exit 0
else
    echo ""
    echo "Found $violations potential issue(s)"
    echo "Note: Runtime safety checks in exec.RealExecutor will catch actual violations"
    exit 1
fi
