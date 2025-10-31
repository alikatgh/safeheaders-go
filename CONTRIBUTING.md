# Contributing to SafeHeaders-Go

Thank you for your interest in contributing to SafeHeaders-Go! This document provides guidelines and standards for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Architecture Guidelines](#architecture-guidelines)
- [Error Handling Standards](#error-handling-standards)
- [Documentation Requirements](#documentation-requirements)

---

## Code of Conduct

Be respectful, inclusive, and professional in all interactions. We welcome contributors of all skill levels and backgrounds.

---

## Getting Started

### What to Contribute

1. **Fix Bugs**: See [ISSUES.md](./ISSUES.md) for known issues
2. **Complete Modules**: Help finish alpha/beta modules (see maturity badges in README)
3. **Add New Ports**: Port new single-header C libraries (see wishlist in README)
4. **Improve Tests**: Increase test coverage, add fuzz tests
5. **Optimize Performance**: Improve parallel chunking, reduce allocations
6. **Documentation**: Add examples, tutorials, godoc improvements

### Before You Start

- Check [ISSUES.md](./ISSUES.md) to see if someone is already working on it
- Open an issue to discuss major changes before implementing
- For new module ports, ensure the original C library has a permissive license (MIT, BSD, Public Domain, zlib)

---

## Development Setup

### Prerequisites

- Go 1.23 or later
- Git
- golangci-lint (for linting)

### Clone and Setup

```bash
git clone https://github.com/alikatgh/safeheaders-go.git
cd safeheaders-go

# Run tests for all modules
go test ./...

# Run tests for specific module
cd jsmn-go && go test -v

# Run benchmarks
go test -bench . -benchmem

# Run linter
golangci-lint run --config ../.golangci.yml
```

### Workspace Structure

SafeHeaders-Go uses Go workspaces (`go.work`). Each module is independent:

```
safeheaders-go/
├── go.work              # Workspace file (references all modules)
├── jsmn-go/
│   ├── go.mod          # Independent module
│   ├── jsmn.go
│   ├── jsmn_test.go
│   └── jsmn_bench.go
└── [other modules]/
```

---

## Coding Standards

### General Principles

1. **Idiomatic Go**: Follow [Effective Go](https://go.dev/doc/effective_go) and [Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
2. **Pure Go**: No CGO dependencies (the whole point of the project!)
3. **Stdlib Only**: Avoid external dependencies unless absolutely necessary
4. **Concurrency-Safe**: All public APIs must be safe for concurrent use (document if not)
5. **Zero-Copy Where Possible**: Minimize allocations and copies

### Naming Conventions

```go
// Package names: lowercase, no underscores
package jsmngo  // Good
package jsmn_go // Bad

// Exported functions: PascalCase, verb-first
func Parse(data []byte) error           // Good
func LoadBatchConcurrent(...)           // Good
func data_parse(data []byte) error      // Bad

// Private functions: camelCase, descriptive
func findSplitPoints(data []byte) []int // Good
func defaultRasterizer(...) (...)       // Good

// Constants: PascalCase for types, SCREAMING_SNAKE for magic numbers
const Object TokenType = iota           // Good (type)
const MaxTokenSize = 1000000           // Good (config)
```

### Code Formatting

- **Use `gofmt`**: All code must be formatted with `gofmt`
- **Line Length**: Max 120 characters (configured in `.golangci.yml`)
- **Imports**: Grouped (stdlib, external, internal) with blank lines
- **Comments**: Full sentences with proper punctuation

```go
// Good comment formatting
// Parse tokenizes the JSON input and returns the number of tokens.
// It returns an error if the JSON is malformed.
func Parse(json []byte) (int, error) {
    // Implementation...
}
```

### Function Length

- **Prefer small functions**: Max ~50 lines
- **Cyclomatic complexity**: Max 15 (enforced by golangci-lint)
- **Extract helpers**: If a function is complex, break it into smaller private functions

---

## Testing Requirements

### Test Coverage

- **Minimum**: 75% coverage for production modules (🟢 Stable)
- **Target**: 80%+ coverage for all modules
- **Critical paths**: 100% coverage for concurrency, error handling

### Test Categories

Every module should have:

1. **Unit Tests** (`*_test.go`)
   - Happy path tests
   - Error path tests (invalid inputs, malformed data)
   - Edge cases (empty inputs, single-byte files, large files)
   - Boundary conditions

2. **Concurrency Tests**
   - Verify parallel results match single-threaded
   - Test for data races (use `go test -race`)
   - Context cancellation tests

3. **Benchmarks** (`*_bench.go`)
   - Single-threaded baseline
   - Parallel performance with different CPU counts
   - Use `b.ReportAllocs()` to track allocations
   - Use realistic data sizes

### Test Naming

```go
// Function tests
func TestParse(t *testing.T) {}
func TestParseParallel(t *testing.T) {}

// Error path tests
func TestParseErrors(t *testing.T) {
    testCases := []struct {
        name string
        json string
    }{
        {"Unclosed Object", `{"key": "value"`},
        {"Invalid Character", `{"key": @}`},
    }
    // Table-driven test...
}

// Benchmarks
func BenchmarkParse(b *testing.B) {}
func BenchmarkParseParallel(b *testing.B) {}
```

### Running Tests

```bash
# All tests
go test ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out
go tool cover -html=coverage.out

# Benchmarks
go test -bench . -benchmem -cpu=1,2,4,8
```

---

## Pull Request Process

### Before Submitting

1. **Run tests**: `go test ./...`
2. **Run linter**: `golangci-lint run --config ../.golangci.yml`
3. **Check coverage**: Ensure no significant drop
4. **Update docs**: Update README, godoc comments, ISSUES.md if needed
5. **Add tests**: New features must have tests

### PR Title Format

Use conventional commits format:

```
feat(jsmn): add smart boundary detection for parallel parsing
fix(stb-image): handle context cancellation correctly
docs(readme): add maturity badges for all modules
test(truetype): add fuzz tests for glyph cache
perf(miniz): reduce allocations in compression path
refactor(common): extract worker pool to internal package
```

### PR Description Template

```markdown
## Summary
Brief description of what this PR does.

## Changes
- Bullet list of changes
- With specific details

## Testing
- What tests were added/modified
- Manual testing performed

## Related Issues
Fixes #123
Related to #456

## Checklist
- [ ] Tests added/updated
- [ ] Benchmarks added/updated (for performance changes)
- [ ] Documentation updated
- [ ] No breaking changes (or documented in PR)
- [ ] Linter passes
```

### Review Process

1. **Automated Checks**: CI must pass (tests, linting, coverage)
2. **Code Review**: Maintainer will review within 1-2 weeks
3. **Address Feedback**: Make requested changes
4. **Merge**: Once approved, maintainer will merge

---

## Architecture Guidelines

### Concurrency Patterns

SafeHeaders-Go uses consistent patterns for parallelism:

#### Worker Pool Pattern

```go
func ProcessConcurrent(data []Item) ([]Result, error) {
    numWorkers := runtime.NumCPU()
    if len(data) < numWorkers {
        numWorkers = len(data)
    }

    // Job distribution
    jobs := make(chan int, len(data))
    for i := range data {
        jobs <- i
    }
    close(jobs)

    // Results collection
    results := make([]Result, len(data))
    errs := make(chan error, numWorkers)
    var wg sync.WaitGroup

    // Spawn workers
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for idx := range jobs {
                result, err := processItem(data[idx])
                if err != nil {
                    errs <- err
                    return // or continue for non-fatal errors
                }
                results[idx] = result
            }
        }()
    }

    wg.Wait()
    close(errs)

    // Error handling
    var multiErr []error
    for err := range errs {
        multiErr = append(multiErr, err)
    }
    if len(multiErr) > 0 {
        return nil, fmt.Errorf("multiple errors: %v", multiErr)
    }

    return results, nil
}
```

#### Context Support

For long-running operations, support context cancellation:

```go
func ProcessConcurrent(ctx context.Context, data []Item) ([]Result, error) {
    // ... setup ...

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case idx, ok := <-jobs:
                    if !ok {
                        return
                    }
                    // Process item...
                case <-ctx.Done():
                    errs <- ctx.Err()
                    return
                }
            }
        }()
    }

    // ... error handling ...

    // Check for context cancellation
    if errors.Is(multiErr[0], context.Canceled) {
        return nil, context.Canceled
    }
}
```

### Memory Management

1. **Pre-allocate**: Size slices when length is known
   ```go
   results := make([]Result, len(data))  // Good
   var results []Result                  // Bad for known sizes
   ```

2. **Reuse buffers**: For hot paths, consider `sync.Pool`
   ```go
   var bufferPool = sync.Pool{
       New: func() interface{} {
           return make([]byte, 4096)
       },
   }
   ```

3. **Limit growth**: For unbounded inputs, enforce limits
   ```go
   const MaxTokens = 1_000_000
   if p.toknext >= MaxTokens {
       return fmt.Errorf("token limit exceeded")
   }
   ```

### Chunking Strategies

When parallelizing data processing:

1. **Semantic Boundaries** (preferred): Split at logical boundaries
   - JSON: Top-level commas, closing braces
   - XML: Element boundaries
   - Audio: Frame/sample boundaries

2. **Fixed-Size Chunks** (fallback): Equal byte divisions
   - Use for binary formats without clear structure
   - Document limitations

3. **Fallback to Serial**: For small inputs or when splits aren't beneficial
   ```go
   if len(data) < MinParallelSize || numSplits < numWorkers {
       return parseSingleThreaded(data)
   }
   ```

---

## Error Handling Standards

### Error Creation

```go
// Simple errors
return errors.New("unclosed string")

// Formatted errors
return fmt.Errorf("invalid character '%c' at position %d", c, pos)

// Wrapped errors (preferred for context)
if err != nil {
    return fmt.Errorf("failed to decode image: %w", err)
}
```

### Error Checking

```go
// Standard check
if err != nil {
    return nil, err
}

// With context
if err != nil {
    return nil, fmt.Errorf("parsing failed: %w", err)
}

// Specific error types
if errors.Is(err, context.Canceled) {
    return nil, context.Canceled
}
```

### Error Aggregation

For parallel operations, collect all errors:

```go
var multiErr []error
for err := range errs {
    multiErr = append(multiErr, err)
}

if len(multiErr) > 0 {
    // Check for special cases first
    if errors.Is(multiErr[0], context.Canceled) {
        return nil, context.Canceled
    }
    // Return aggregated error
    return nil, fmt.Errorf("multiple errors occurred: %v", multiErr)
}
```

---

## Documentation Requirements

### Package-Level Documentation

```go
// Package jsmngo provides a fast JSON tokenizer with parallel processing.
//
// This package is a pure Go rewrite of the jsmn C library, with added
// concurrency support for processing large JSON files efficiently.
//
// Basic usage:
//
//	p := jsmngo.NewParser(100)
//	_, err := p.Parse(jsonData)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	tokens := p.Tokens()
//
// For large JSON files, use parallel parsing:
//
//	tokens, err := jsmngo.ParseParallel(largeJSON)
//
package jsmngo
```

### Function Documentation

```go
// Parse tokenizes the JSON input and returns the number of tokens parsed.
// It returns an error if the JSON is malformed or incomplete.
//
// The parser maintains internal state and can be reused, but is not safe
// for concurrent use. Use NewParser to create a new instance for each
// goroutine, or use ParseParallel for automatic parallelization.
//
// Example:
//
//	json := []byte(`{"key": "value", "arr": [1, 2, 3]}`)
//	p := NewParser(10)
//	n, err := p.Parse(json)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Parsed %d tokens\n", n)
func Parse(json []byte) (int, error)
```

### Module README

Each production module should have a README.md with:

1. **Overview**: What the module does
2. **Installation**: `go get` command
3. **Quick Start**: Basic usage example
4. **API Documentation**: Key functions and types
5. **Performance**: Benchmark results vs original C library
6. **Limitations**: Known issues and edge cases
7. **Contributing**: Link to CONTRIBUTING.md

---

## Module Porting Guidelines

### When Porting a New C Library

1. **Verify License**: Must be MIT, BSD, Public Domain, or compatible
2. **Start Small**: Port core functionality first, add concurrency later
3. **Safety First**: Use Go's bounds checking, avoid `unsafe` package
4. **Add Tests**: Compare output with original C library
5. **Benchmark**: Compare performance with C version
6. **Document Differences**: Note any API or behavior changes

### Required Files for New Module

```
new-module-go/
├── go.mod              # Module definition
├── new_module.go       # Main implementation
├── new_module_test.go  # Tests
├── new_module_bench.go # Benchmarks
└── README.md           # Module documentation
```

### Initial Implementation Checklist

- [ ] Core functionality working (matches C behavior)
- [ ] Error handling for invalid inputs
- [ ] Tests covering happy path and errors
- [ ] Benchmarks vs C library
- [ ] Thread-safety documented
- [ ] Godoc comments on all exports
- [ ] README with examples
- [ ] Add to main README.md with 🔴 Alpha status

---

## Performance Guidelines

### Allocation Reduction

1. **Avoid allocations in hot paths**:
   ```go
   // Bad
   func process(data []byte) {
       for i := range data {
           temp := make([]byte, 100) // Allocates every iteration
       }
   }

   // Good
   func process(data []byte) {
       temp := make([]byte, 100) // Allocate once
       for i := range data {
           // Reuse temp
       }
   }
   ```

2. **Pre-size slices when possible**:
   ```go
   tokens := make([]Token, 0, estimatedSize) // Pre-allocate capacity
   ```

3. **Use benchmarks to measure**:
   ```go
   func BenchmarkParse(b *testing.B) {
       b.ReportAllocs() // Shows allocations per op
       for i := 0; i < b.N; i++ {
           Parse(data)
       }
   }
   ```

### Concurrency Tuning

1. **Cost/Benefit Analysis**: Parallelism has overhead
   - Only parallelize for large inputs (>4KB typically)
   - Measure speedup with benchmarks at 1,2,4,8 CPUs

2. **Worker Count**: Use `runtime.NumCPU()` but respect job count
   ```go
   numWorkers := runtime.NumCPU()
   if numJobs < numWorkers {
       numWorkers = numJobs
   }
   ```

3. **Minimize Synchronization**: Lock-free when possible
   - Use channels for distribution
   - Pre-allocate result slices to avoid locks

---

## Questions?

- **General Questions**: Open a GitHub Discussion
- **Bug Reports**: Open a GitHub Issue
- **Security Issues**: See SECURITY.md (TODO: create this file)
- **Feature Requests**: Open a GitHub Issue with [Feature Request] tag

---

## Thank You!

Your contributions make SafeHeaders-Go better for everyone. Whether it's fixing a typo, adding tests, or porting a new library, every contribution is valued!

**Maintainer**: @alikatgh
**Last Updated**: 2025-10-31
