#!/bin/bash
# Benchmark comparison script for SafeHeaders-Go

set -e

echo "SafeHeaders-Go Benchmark Suite"
echo "==============================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Modules to benchmark
MODULES=("jsmn-go" "stb-image-go" "stb-truetype-go")

# Benchmark output directory
BENCH_DIR="benchmarks"
mkdir -p "$BENCH_DIR"

# Run benchmarks for each module
for module in "${MODULES[@]}"; do
    echo -e "${YELLOW}Benchmarking $module...${NC}"

    if [ ! -d "$module" ]; then
        echo -e "${RED}Module $module not found, skipping${NC}"
        continue
    fi

    cd "$module"

    # Run benchmark multiple times for accuracy
    echo "  Running benchmarks (3 iterations)..."
    go test -bench=. -benchmem -benchtime=3s -count=3 \
        > "../$BENCH_DIR/$module-bench.txt" 2>&1

    echo -e "${GREEN}  ✓ Complete${NC}"

    # Generate statistics if benchstat is available
    if command -v benchstat &> /dev/null; then
        echo "  Generating statistics..."
        benchstat "../$BENCH_DIR/$module-bench.txt" \
            > "../$BENCH_DIR/$module-stats.txt" 2>&1
        echo -e "${GREEN}  ✓ Stats generated${NC}"
    fi

    cd ..
    echo ""
done

echo -e "${GREEN}Benchmarks complete!${NC}"
echo "Results saved to $BENCH_DIR/"
echo ""

# Summary
echo "Summary:"
echo "--------"
for module in "${MODULES[@]}"; do
    if [ -f "$BENCH_DIR/$module-bench.txt" ]; then
        echo -e "${YELLOW}$module:${NC}"
        grep "^Benchmark" "$BENCH_DIR/$module-bench.txt" | head -3
        echo ""
    fi
done

echo ""
echo "For detailed analysis:"
echo "  cat $BENCH_DIR/<module>-stats.txt"
echo ""
echo "To compare with a baseline:"
echo "  benchstat baseline.txt current.txt"
