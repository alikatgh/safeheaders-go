# Architecture Documentation

This document describes the architecture, design patterns, and technical decisions behind SafeHeaders-Go.

## Table of Contents

- [Project Structure](#project-structure)
- [Design Principles](#design-principles)
- [Module Architecture](#module-architecture)
- [Concurrency Patterns](#concurrency-patterns)
- [Error Handling](#error-handling)
- [Performance Considerations](#performance-considerations)
- [Security Architecture](#security-architecture)
- [Testing Strategy](#testing-strategy)

---

## Project Structure

### Directory Layout

```
safeheaders-go/
├── .github/                    # GitHub configuration
│   ├── workflows/              # CI/CD workflows
│   ├── ISSUE_TEMPLATE/         # Issue templates
│   ├── PULL_REQUEST_TEMPLATE.md
│   ├── CODEOWNERS
│   └── dependabot.yml
├── .vscode/                    # VS Code configuration
│   ├── settings.json
│   ├── launch.json
│   ├── tasks.json
│   └── extensions.json
├── scripts/                    # Automation scripts
│   ├── benchmark.sh
│   ├── generate-testdata.sh
│   ├── integration-test.sh
│   └── release.sh
├── testdata/                   # Test data files
│   ├── small.json
│   ├── medium.json
│   ├── large.json
│   └── ...
├── examples/                   # Usage examples
│   ├── json-parser/
│   └── production-usage/
├── jsmn-go/                    # JSON parser module
│   ├── jsmn.go                 # Core implementation
│   ├── config.go               # Configuration & validation
│   ├── parallel.go             # Parallel parsing
│   ├── jsmn_test.go            # Tests
│   ├── jsmn_fuzz_test.go       # Fuzz tests
│   ├── jsmn_bench.go           # Benchmarks
│   ├── go.mod
│   └── README.md
├── [other modules]/            # Similar structure
├── go.work                     # Go workspace
├── go.mod                      # Root module
├── Makefile                    # Development tasks
├── Dockerfile                  # Production container
├── docker-compose.yml          # Container orchestration
└── [documentation files]
```

### Module Independence

Each module is **independent**:
- Has its own `go.mod` file
- Can be imported separately
- Has its own tests and benchmarks
- Can be versioned independently

**Workspace benefits:**
- Shared tooling across modules
- Easier cross-module development
- Consistent versions during development

---

## Design Principles

### 1. Memory Safety First

**Principle:** No unsafe operations, no buffer overflows.

**Implementation:**
- Use Go's built-in bounds checking
- Avoid `unsafe` package entirely
- Pre-allocate slices with known capacities
- Validate all inputs

**Example:**
```go
// Bad: Could panic
data[i]

// Good: Checked access
if i < len(data) {
    return data[i]
}
```

### 2. Zero Dependencies

**Principle:** Pure stdlib only.

**Benefits:**
- No supply chain attacks
- Easier auditing
- Smaller binaries
- Simpler deployment

**Exception:** Development tools (linters, etc.) are allowed.

### 3. Concurrency-First Design

**Principle:** Parallelism is a first-class feature, not an afterthought.

**Implementation:**
- All parsers support parallel mode
- Context support for cancellation
- Worker pool patterns
- Automatic strategy selection

### 4. Production-Ready Defaults

**Principle:** Defaults should be safe for production.

**Implementation:**
- Sensible input limits (100MB default)
- Token limits to prevent DoS
- Timeout protection via context
- Graceful error handling

---

## Module Architecture

### Layered Design

Each module follows a layered architecture:

```
┌─────────────────────────────────────┐
│     Public API Layer                │  ← Simple, ergonomic API
├─────────────────────────────────────┤
│     Configuration Layer             │  ← Validation & limits
├─────────────────────────────────────┤
│     Strategy Layer                  │  ← Serial vs. Parallel
├─────────────────────────────────────┤
│     Core Implementation             │  ← Parsing logic
├─────────────────────────────────────┤
│     Utilities                       │  ← Helper functions
└─────────────────────────────────────┘
```

### Example: jsmn-go

**Public API** (`jsmn.go`):
- `NewParser(capacity)` - Simple parser creation
- `Parse(data)` - Serial parsing
- `ParseParallel(data)` - Parallel parsing (legacy)

**Configuration** (`config.go`):
- `Config` struct with limits
- `DefaultConfig()`, `StrictConfig()`, `UnlimitedConfig()`
- Input validation

**Strategy** (`parallel.go`):
- `ParseWithConfig(ctx, data, config)` - New unified API
- `ParseParallelWithContext(ctx, data)` - Context-aware parallel
- Automatic serial vs. parallel selection

**Core** (`jsmn.go`):
- `Parser` struct
- Token types
- Parsing logic

---

## Concurrency Patterns

### Worker Pool Pattern

All parallel implementations use the worker pool pattern:

```go
// 1. Create job channel
jobs := make(chan Job, numJobs)

// 2. Create result channel
results := make(chan Result, numJobs)

// 3. Spawn workers
for i := 0; i < numWorkers; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for job := range jobs {
            result := process(job)
            results <- result
        }
    }()
}

// 4. Wait and collect
wg.Wait()
close(results)
```

### Context Cancellation

All long-running operations support context:

```go
func ProcessWithContext(ctx context.Context, data []byte) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Do work
        }
    }
}
```

### Chunking Strategy

**Semantic boundaries** (preferred):
```go
// Find safe split points (e.g., top-level commas in JSON)
splits := findSplitPoints(data)
```

**Fixed-size chunks** (fallback):
```go
// Equal byte divisions
chunkSize := len(data) / numWorkers
```

### Automatic Strategy Selection

```go
func Parse(data []byte) {
    if len(data) < threshold {
        return parseSerial(data)  // Small input
    }
    return parseParallel(data)  // Large input
}
```

---

## Error Handling

### Error Types

**Sentinel errors:**
```go
var (
    ErrInputTooLarge = errors.New("input too large")
    ErrTooManyTokens = errors.New("too many tokens")
    ErrEmptyInput    = errors.New("empty input")
)
```

**Error wrapping:**
```go
if err != nil {
    return fmt.Errorf("parse failed: %w", err)
}
```

**Error checking:**
```go
if errors.Is(err, ErrInputTooLarge) {
    // Handle specific error
}
```

### Error Aggregation

For parallel operations:

```go
var multiErr []error
for err := range errs {
    multiErr = append(multiErr, err)
}

if len(multiErr) > 0 {
    // Check for special cases (context cancellation)
    if errors.Is(multiErr[0], context.Canceled) {
        return context.Canceled
    }
    return fmt.Errorf("multiple errors: %v", multiErr)
}
```

---

## Performance Considerations

### Memory Allocation

**Pre-allocation:**
```go
// Good: Pre-allocate if size is known
tokens := make([]Token, 0, estimatedSize)

// Bad: Grows dynamically
var tokens []Token
```

**Pool reuse:**
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 4096)
    },
}
```

### Benchmarking

**Always benchmark:**
```bash
make bench
```

**Use benchstat for comparison:**
```bash
benchstat old.txt new.txt
```

**Report allocations:**
```go
func BenchmarkParse(b *testing.B) {
    b.ReportAllocs()
    // ...
}
```

### Parallelism Overhead

**Threshold-based selection:**
- Small inputs (< 4KB): Serial
- Large inputs (≥ 4KB): Parallel

**Cost-benefit analysis:**
- Parallel overhead: ~50-100μs
- Break-even point: ~4KB input

---

## Security Architecture

### Input Validation

**Three-tier validation:**

1. **Size validation:**
   ```go
   if len(data) > config.MaxInputSize {
       return ErrInputTooLarge
   }
   ```

2. **Token count validation:**
   ```go
   if tokenCount > config.MaxTokens {
       return ErrTooManyTokens
   }
   ```

3. **Timeout protection:**
   ```go
   ctx, cancel := context.WithTimeout(ctx, timeout)
   defer cancel()
   ```

### DoS Prevention

**Resource limits:**
- Max input size (default: 100MB)
- Max tokens (default: 1M)
- Timeout (user-configured)

**Configurable presets:**
- `DefaultConfig()` - 100MB, 1M tokens
- `StrictConfig()` - 10MB, 100K tokens
- `UnlimitedConfig()` - No limits (unsafe)

### Security Scanning

**Automated scanning:**
- `gosec` - Static analysis
- `govulncheck` - Vulnerability database
- Weekly scheduled scans

---

## Testing Strategy

### Test Pyramid

```
         /\
        /  \   E2E Tests (Integration)
       /────\
      /      \  Integration Tests
     /────────\
    /          \ Unit Tests
   /────────────\
  /   Fuzz Tests  \ (Continuous)
 /──────────────────\
```

### Test Types

**Unit Tests:**
- Happy path
- Error paths
- Edge cases
- Boundary conditions

**Fuzz Tests:**
- Random input generation
- Crash detection
- Boundary violation detection

**Integration Tests:**
- Cross-module compatibility
- Example execution
- Build verification

**Benchmarks:**
- Performance regression detection
- Parallel vs. serial comparison
- Memory profiling

### Coverage Requirements

- **Minimum:** 70%
- **Target:** 80%+
- **Critical paths:** 100%

---

## CI/CD Pipeline

### Workflow

```
┌─────────────┐
│   Push/PR   │
└──────┬──────┘
       │
       v
┌─────────────┐
│   Lint      │  ← golangci-lint
└──────┬──────┘
       │
       v
┌─────────────┐
│   Test      │  ← go test -race
└──────┬──────┘
       │
       v
┌─────────────┐
│  Coverage   │  ← 70% threshold
└──────┬──────┘
       │
       v
┌─────────────┐
│  Security   │  ← gosec + govulncheck
└──────┬──────┘
       │
       v
┌─────────────┐
│   Build     │  ← Multi-OS
└──────┬──────┘
       │
       v
┌─────────────┐
│   Deploy    │  ← On release
└─────────────┘
```

---

## Future Architecture Plans

### v1.0 Roadmap

- [ ] Streaming APIs for large files
- [ ] WebAssembly support
- [ ] Smart chunking for better parallelism
- [ ] Internal worker pool library

### v2.0 Ideas

- [ ] Plugin system for custom parsers
- [ ] gRPC service wrappers
- [ ] Kubernetes operators

---

**Last Updated**: 2025-11-16
**Maintainer**: @alikatgh
