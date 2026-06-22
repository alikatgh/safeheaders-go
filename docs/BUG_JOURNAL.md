# Bug Journal

## Patterns to scan for FIRST

- **Multi-error context flattening**: When collecting errors from goroutines into a slice and formatting as a string, context sentinel errors (context.Canceled, context.DeadlineExceeded) lose their identity. Always check errors.Is() before formatting. Applies to any function that fans out work into goroutines and collects errors.
- **Fuzz test token bounds check on error**: When a parser fails partway through, partial tokens with End=-1 remain accessible via Tokens(). Fuzz tests that validate token bounds must skip the check when Parse() returns an error, or the test will report false positives.
- **Size-gated parallel code path in tests**: If parallel processing only activates above a byte threshold (e.g. >4096 bytes), test data must exceed that threshold or the parallel path is never exercised and coverage is misleadingly low.
- **OO API fuzz test referencing wrong API**: Auto-generated fuzz tests can reference an OO-style API (NewDocument/doc.Parse) when the actual package exposes a functional API (Parse(data) (*Doc, error)). Always compile-check fuzz tests before merging.
- **Unbounded recursive parser = stack-overflow DoS**: Recursive descent parsers (XML elements, JSON values) that recurse once per nesting level will exhaust the stack on adversarial deeply-nested input. Guard with a MaxNestingDepth limit checked at the top of the recursive function, plus MaxInputSize and a shared MaxNodeCount counter. Follow the jsmn-go Config pattern (DefaultConfig/StrictConfig/UnlimitedConfig + sentinel errors via errors.Is); add limits as a NEW ParseWithConfig entrypoint so the existing Parse signature stays back-compatible.

## Chronological Log

### 2026-06-22 — tinyxml2-go: fuzz test references non-existent OO API

- **File**: `tinyxml2-go/tinyxml2_fuzz_test.go:36`
- **Symptom**: `go vet` fails with `undefined: NewDocument`; fuzz tests are completely broken
- **Cause**: Fuzz test was generated against an OO-style API (`NewDocument()`, `doc.Parse()`, etc.) but the actual package uses a functional API (`Parse(data []byte) (*XMLDocument, error)`)
- **Fix**: Rewrote fuzz test to use the actual `Parse()` function and `*Node` struct fields
- **Lesson**: Always run `go vet` on fuzz tests before merging; OO vs functional API mismatches are common in generated test code

### 2026-06-22 — jsmn-go: fuzz seed test reports false positives on parse errors

- **File**: `jsmn-go/jsmn_fuzz_test.go:52`
- **Symptom**: `FuzzParse/seed#15` and `seed#16` fail: "Token 0 has invalid bounds: Start=0 End=-1"
- **Cause**: Fuzz test validates token bounds unconditionally; on failed parses, partial tokens with `End=-1` remain in `p.Tokens()`, causing the bounds check to trigger falsely
- **Fix**: Skip the bounds check when `Parse()` returns a non-nil error; partial tokens are expected on failure
- **Lesson**: Fuzz tests checking output invariants must guard on successful parse only; error paths leave parser state partially populated

### 2026-06-22 — stb-image-go: LoadBatchConcurrent returns string instead of context.Canceled

- **File**: `stb-image-go/stb_image.go:78`
- **Symptom**: `TestLoadBatchConcurrent_Cancellation` failed: expected `context.Canceled`, got `[context canceled context canceled ...]`
- **Cause**: Multiple workers all sent `ctx.Err()` to the errors channel; the collector formatted the entire slice as a string, losing the sentinel error identity
- **Fix**: Before formatting a multi-error, check each error with `errors.Is(e, context.Canceled)` and return it directly if matched
- **Lesson**: When aggregating goroutine errors, always preserve context sentinel errors; formatting loses `errors.Is` identity
