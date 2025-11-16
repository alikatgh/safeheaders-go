# SafeHeaders-Go Scripts

This directory contains automation scripts for development, testing, and release management.

## Available Scripts

### 📊 `benchmark.sh`

Run comprehensive benchmarks for all modules with statistical analysis.

```bash
./scripts/benchmark.sh
```

**What it does:**
- Runs benchmarks for jsmn-go, stb-image-go, stb-truetype-go
- Executes 3 iterations for statistical accuracy
- Generates benchstat reports if available
- Saves results to `benchmarks/` directory

**Output:**
```
benchmarks/
├── jsmn-go-bench.txt       # Raw benchmark results
├── jsmn-go-stats.txt       # Statistical analysis
├── stb-image-go-bench.txt
├── stb-image-go-stats.txt
└── ...
```

**Usage tips:**
```bash
# Run benchmarks
./scripts/benchmark.sh

# Compare with baseline
benchstat benchmarks/baseline.txt benchmarks/jsmn-go-bench.txt

# Filter specific benchmarks
grep "BenchmarkParse" benchmarks/jsmn-go-bench.txt
```

---

### 🧪 `generate-testdata.sh` / `generate-testdata.go`

Generate test data files for benchmarking and testing.

```bash
./scripts/generate-testdata.sh
```

**What it generates:**
- `testdata/large.json` - 10MB JSON array with 50,000 objects
- `testdata/large.xml` - 5MB XML with 10,000 elements
- `testdata/nested.json` - Deeply nested JSON (100 levels)
- `testdata/primitives.json` - Array of 10,000 primitives

**Implementation:**
- Uses Go-based generator (`generate-testdata.go`) for performance
- ~5 seconds to generate all files (vs 10+ minutes with bash)
- Bash wrapper (`generate-testdata.sh`) for convenience

**Why this is needed:**
- Realistic benchmark data
- Test parser performance with large inputs
- Verify parallel parsing efficiency
- Test edge cases (deep nesting)

**Regenerate data:**
```bash
rm -rf testdata/*.json testdata/*.xml
./scripts/generate-testdata.sh

# Or run Go directly for more control:
go run scripts/generate-testdata.go
```

---

### ✅ `integration-test.sh`

Run comprehensive integration tests across all modules.

```bash
./scripts/integration-test.sh
```

**What it tests:**
1. JSON parser example runs successfully
2. All modules have test files
3. Modules can be imported without errors
4. Fuzz tests are present
5. Benchmarks are present
6. Test data files exist
7. Example binaries can be built
8. Docker image builds (if Docker available)
9. CI checks pass (fmt, vet)

**Exit codes:**
- `0` - All tests passed
- `1` - One or more tests failed

**Example output:**
```
SafeHeaders-Go Integration Tests
=================================

Test 1: JSON Parser Example
  ✓ PASSED

Test 2: All modules have test files
  ✓ PASSED

...

=================================
Integration Test Results
=================================

Total tests: 9
Passed: 9
Failed: 0

All integration tests passed!
```

---

### 🚀 `release.sh`

Automated release process with all checks.

```bash
./scripts/release.sh v0.5.1
```

**What it does:**
1. **Pre-flight checks:**
   - Verifies on main branch
   - Checks working directory is clean
   - Ensures tag doesn't already exist

2. **Testing:**
   - Runs full test suite
   - Runs linter
   - Verifies all checks pass

3. **Release:**
   - Updates CHANGELOG.md
   - Creates git tag
   - Pushes to remote
   - Triggers GitHub Actions release workflow

**Requirements:**
- Must be on `main` branch
- Working directory must be clean
- Version format: `vX.Y.Z` or `vX.Y.Z-suffix`

**Example:**
```bash
# Patch release
./scripts/release.sh v0.5.1

# Minor release
./scripts/release.sh v0.6.0

# Major release
./scripts/release.sh v1.0.0

# Pre-release
./scripts/release.sh v1.0.0-rc1
```

**What happens after:**
- GitHub Actions builds binaries (Linux, macOS, Windows)
- Creates GitHub release with changelog
- Publishes Docker image to GitHub Container Registry
- Attaches binaries to release

---

## Script Dependencies

### Required Tools

| Script | Required Tools | Installation |
|--------|---------------|-------------|
| `benchmark.sh` | `go`, `benchstat` (optional) | `go install golang.org/x/tools/cmd/benchstat@latest` |
| `generate-testdata.sh` | `go`, `bash` | - |
| `integration-test.sh` | `go`, `docker` (optional) | - |
| `release.sh` | `go`, `git` | - |

### Installing Optional Tools

```bash
# Install benchstat for statistical analysis
go install golang.org/x/tools/cmd/benchstat@latest

# Install other development tools
make install-tools
```

---

## Using Scripts in CI/CD

### GitHub Actions

Scripts are used in CI workflows:

```yaml
- name: Run Integration Tests
  run: ./scripts/integration-test.sh

- name: Run Benchmarks
  run: ./scripts/benchmark.sh

- name: Generate Test Data
  run: ./scripts/generate-testdata.sh
```

### Local Pre-commit

Add to `.git/hooks/pre-commit`:

```bash
#!/bin/bash
set -e

echo "Running integration tests..."
./scripts/integration-test.sh

echo "All checks passed!"
```

### Docker

Scripts work inside Docker containers:

```bash
# Run in dev container
docker-compose run dev ./scripts/benchmark.sh
```

---

## Development Workflow

### Typical Development Cycle

```bash
# 1. Make changes to code
vim jsmn-go/jsmn.go

# 2. Run tests
make test

# 3. Run integration tests
./scripts/integration-test.sh

# 4. Run benchmarks (if performance-sensitive)
./scripts/benchmark.sh

# 5. Commit changes
git commit -m "feat: improve parsing performance"

# 6. Release (when ready)
./scripts/release.sh v0.5.1
```

### Benchmarking Workflow

```bash
# 1. Establish baseline
./scripts/benchmark.sh
cp benchmarks/jsmn-go-bench.txt benchmarks/baseline.txt

# 2. Make performance improvements
vim jsmn-go/jsmn.go

# 3. Re-run benchmarks
./scripts/benchmark.sh

# 4. Compare with baseline
benchstat benchmarks/baseline.txt benchmarks/jsmn-go-bench.txt
```

### Testing Workflow

```bash
# 1. Generate fresh test data
./scripts/generate-testdata.sh

# 2. Run all tests
make test

# 3. Run integration tests
./scripts/integration-test.sh

# 4. Run fuzz tests (optional)
cd jsmn-go && go test -fuzz=FuzzParse -fuzztime=1m
```

---

## Troubleshooting

### "Permission denied" Error

```bash
chmod +x scripts/*.sh
```

### "benchstat: command not found"

```bash
go install golang.org/x/tools/cmd/benchstat@latest
```

### Integration tests fail on Docker

```bash
# Skip Docker tests if Docker is not available
# The script automatically skips if Docker isn't installed
which docker || echo "Docker not available, tests will be skipped"
```

### Release script fails

**Common issues:**
- Not on main branch: `git checkout main`
- Dirty working directory: `git status` and commit/stash changes
- Tag already exists: Use a different version number

---

## Contributing New Scripts

When adding new scripts:

1. **Make executable:**
   ```bash
   chmod +x scripts/new-script.sh
   ```

2. **Add shebang:**
   ```bash
   #!/bin/bash
   set -e  # Exit on error
   ```

3. **Add to this README**

4. **Add to Makefile** (if appropriate)

5. **Test in CI** (add to `.github/workflows/`)

---

## Script Best Practices

### Error Handling

```bash
#!/bin/bash
set -e  # Exit on error
set -u  # Error on undefined variables
set -o pipefail  # Catch errors in pipelines
```

### Colored Output

```bash
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'  # No Color

echo -e "${GREEN}Success!${NC}"
echo -e "${RED}Error!${NC}"
```

### Progress Indication

```bash
echo "Step 1/3: Running tests..."
# ...
echo "✓ Complete"
```

---

## See Also

- [Makefile](../Makefile) - Common development tasks
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines
- [.github/workflows/](../.github/workflows/) - CI/CD workflows

---

**Last Updated**: 2025-11-16
**Maintainer**: @alikatgh
