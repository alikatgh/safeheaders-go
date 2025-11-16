# Pull Request

## Summary

<!-- Brief description of what this PR does (1-2 sentences) -->

## Type of Change

<!-- Check all that apply -->

- [ ] 🐛 Bug fix (non-breaking change that fixes an issue)
- [ ] ✨ New feature (non-breaking change that adds functionality)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] 📝 Documentation update
- [ ] 🎨 Code refactoring (no functional changes)
- [ ] ⚡ Performance improvement
- [ ] ✅ Test improvement
- [ ] 🔧 CI/CD or tooling change

## Affected Module(s)

<!-- Check all that apply -->

- [ ] jsmn-go
- [ ] stb-image-go
- [ ] stb-truetype-go
- [ ] tinyxml2-go
- [ ] cjson-go
- [ ] miniz-go
- [ ] cgltf-go
- [ ] dr-wav-go
- [ ] Infrastructure / Project-wide

## Changes

<!-- Detailed list of changes made in this PR -->

-
-
-

## Related Issues

<!-- Link to related issues -->

Fixes #
Closes #
Related to #

## Testing

### Tests Added/Modified

<!-- Describe the tests you added or modified -->

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Benchmarks added/updated
- [ ] Fuzz tests added/updated
- [ ] Tests pass locally (`go test ./...`)
- [ ] Tests pass with race detector (`go test -race ./...`)

### Manual Testing Performed

<!-- Describe any manual testing you did -->

```go
// Example code showing the feature works
```

### Test Coverage

<!-- If applicable, include coverage change -->

- Previous coverage: __%
- New coverage: __%
- Change: +/- __%

## Performance Impact

<!-- If this PR affects performance, include benchmarks -->

**Before**:
```
BenchmarkParse-8    1000000    1500 ns/op    500 B/op    10 allocs/op
```

**After**:
```
BenchmarkParse-8    1500000    1000 ns/op    300 B/op     5 allocs/op
```

**Analysis**: 33% faster, 40% less memory, 50% fewer allocations

## Breaking Changes

<!-- If this is a breaking change, describe the impact and migration path -->

### API Changes

**Before**:
```go
func OldAPI(param string) error
```

**After**:
```go
func NewAPI(ctx context.Context, param string) error
```

### Migration Guide

```go
// Old code:
err := OldAPI("value")

// New code:
err := NewAPI(context.Background(), "value")
```

## Documentation

<!-- Check all that apply -->

- [ ] Updated module README.md
- [ ] Updated CHANGELOG.md
- [ ] Updated ISSUES.md (if fixing a tracked issue)
- [ ] Added/updated godoc comments
- [ ] Added/updated code examples
- [ ] No documentation needed

## Pre-submission Checklist

<!-- Check all that apply -->

### Code Quality

- [ ] Code follows project style guidelines (see CONTRIBUTING.md)
- [ ] Self-review of code performed
- [ ] Code passes `gofmt` formatting
- [ ] Code passes `golangci-lint` with no new warnings
- [ ] No commented-out code or debug statements
- [ ] Error handling is consistent with project standards

### Testing & Safety

- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test -race ./...`)
- [ ] No new compiler warnings
- [ ] Memory safety verified (no unsafe pointer arithmetic)
- [ ] Concurrency safety verified (goroutine-safe or documented otherwise)

### Documentation

- [ ] Public functions have godoc comments
- [ ] Complex logic has inline comments
- [ ] CHANGELOG.md updated (if user-facing change)
- [ ] README.md updated (if needed)

### Dependencies

- [ ] No new external dependencies added (or justified in PR description)
- [ ] go.mod and go.sum updated (if dependencies changed)

## Security Considerations

<!-- Answer these questions if applicable -->

- **Does this PR handle user input?** Yes / No
  - If yes, is input validated? ___
- **Does this PR affect memory allocation?** Yes / No
  - If yes, are there limits to prevent DoS? ___
- **Does this PR involve concurrency?** Yes / No
  - If yes, is it race-free? ___
- **Does this PR expose new public APIs?** Yes / No
  - If yes, are they safe for concurrent use? ___

## Additional Context

<!-- Any other information reviewers should know -->

## Screenshots (if applicable)

<!-- For visual changes, include before/after screenshots -->

---

## For Maintainers

<!-- Maintainers: fill this out before merging -->

### Review Checklist

- [ ] Code review completed
- [ ] Tests reviewed and sufficient
- [ ] Documentation reviewed
- [ ] Breaking changes (if any) are acceptable
- [ ] Security implications reviewed
- [ ] Performance implications reviewed
- [ ] Ready to merge

### Post-Merge Actions

- [ ] Update project board
- [ ] Create GitHub release (if version bump)
- [ ] Announce in discussions (if significant change)
- [ ] Update roadmap (if applicable)

---

**Thank you for contributing to SafeHeaders-Go!** 🎉
