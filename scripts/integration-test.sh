#!/bin/bash
# Integration tests for SafeHeaders-Go modules

set -e

echo "SafeHeaders-Go Integration Tests"
echo "================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0
PASSED=0

# Test counter
test_count=0

# Helper function to run a test
run_test() {
    local name="$1"
    local command="$2"

    test_count=$((test_count + 1))
    echo -e "${YELLOW}Test $test_count: $name${NC}"

    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓ PASSED${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}  ✗ FAILED${NC}"
        echo "    Command: $command"
        FAILED=$((FAILED + 1))
    fi
    echo ""
}

# Ensure testdata exists
if [ ! -d "testdata" ]; then
    echo "Generating test data..."
    bash scripts/generate-testdata.sh
    echo ""
fi

# Test 1: JSON Parser Example
run_test "JSON Parser Example" \
    "cd examples/json-parser && go run main.go"

# Test 2: All modules have tests
run_test "All modules have test files" \
    'find . -name "*_test.go" -type f | wc -l | grep -qv "^0$"'

# Test 3: Modules can be imported
echo -e "${YELLOW}Test $test_count: Module imports${NC}"
cat > /tmp/test_import.go << 'EOF'
package main

import (
    _ "github.com/alikatgh/safeheaders-go/jsmn-go"
    _ "github.com/alikatgh/safeheaders-go/stb-image-go"
    _ "github.com/alikatgh/safeheaders-go/tinyxml2-go"
)

func main() {}
EOF

cd /tmp
if go run test_import.go > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓ PASSED${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e "${RED}  ✗ FAILED${NC}"
    FAILED=$((FAILED + 1))
fi
cd - > /dev/null
echo ""
test_count=$((test_count + 1))

# Test 4: Fuzz tests exist
run_test "Fuzz tests present" \
    'grep -r "func Fuzz" jsmn-go/ tinyxml2-go/ | wc -l | grep -qv "^0$"'

# Test 5: Benchmarks exist
run_test "Benchmarks present" \
    'find . -name "*_bench.go" -type f | wc -l | grep -qv "^0$"'

# Test 6: Test data files exist
run_test "Test data files present" \
    'test -f testdata/small.json && test -f testdata/small.xml'

# Test 7: Example binaries can be built
run_test "Example binaries build" \
    "make build-examples"

# Test 8: Docker image builds
if command -v docker &> /dev/null; then
    run_test "Docker image builds" \
        "docker build -t safeheaders-test -f Dockerfile . > /dev/null 2>&1"
else
    echo -e "${YELLOW}Test $test_count: Docker image builds${NC}"
    echo -e "  ${YELLOW}⊘ SKIPPED (Docker not available)${NC}"
    echo ""
    test_count=$((test_count + 1))
fi

# Test 9: CI simulation
run_test "CI checks pass" \
    "make fmt && make vet"

# Summary
echo "================================="
echo "Integration Test Results"
echo "================================="
echo ""
echo -e "Total tests: $test_count"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Some integration tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All integration tests passed!${NC}"
    exit 0
fi
