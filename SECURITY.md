# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

**Note**: Versions below 1.0 are considered pre-release and may not receive security patches. Please upgrade to stable versions.

## Reporting a Vulnerability

We take the security of SafeHeaders-Go seriously. If you discover a security vulnerability, please follow these steps:

### 1. **DO NOT** Open a Public Issue

Security vulnerabilities should not be disclosed publicly until a fix is available. Opening a public issue could put users at risk.

### 2. Report Privately

Please report security vulnerabilities by emailing: **safeheaders@aulenor.com** (or create a GitHub Security Advisory)

Alternatively, you can use GitHub's private vulnerability reporting feature:
1. Go to the repository's "Security" tab
2. Click "Report a vulnerability"
3. Fill in the details

### 3. Include Details

Please include the following information in your report:

- **Type of vulnerability** (e.g., DoS, buffer overflow, memory exhaustion)
- **Affected module(s)** (e.g., jsmn-go, stb-image-go)
- **Attack scenario** (how can this be exploited?)
- **Proof of concept** (code sample or steps to reproduce)
- **Suggested fix** (if you have one)
- **Impact assessment** (how severe is this?)

### Example Report Template

```
**Module**: jsmn-go
**Vulnerability Type**: Denial of Service (DoS)
**Severity**: High

**Description**:
The ParseParallel function does not limit the maximum number of tokens,
allowing an attacker to send malicious JSON that causes memory exhaustion.

**Proof of Concept**:
```go
// Send 1GB of nested arrays: [[[[...]]]]
maliciousJSON := bytes.Repeat([]byte("["), 1000000)
ParseParallel(maliciousJSON) // Causes OOM
```

**Impact**:
An attacker can crash the application by sending specially crafted JSON.

**Suggested Fix**:
Add a MaxTokens limit (default: 1,000,000) to ParserConfig.
```

### 4. Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 1 week
- **Fix Development**: 1-4 weeks (depending on severity)
- **Public Disclosure**: After fix is released and users have time to upgrade

### 5. Disclosure Policy

We follow responsible disclosure principles:

1. **Private Disclosure**: Report sent privately to maintainers
2. **Fix Development**: We work on a patch in a private branch
3. **Security Advisory**: We prepare a GitHub Security Advisory
4. **Release**: We release a patched version
5. **Public Disclosure**: We publish the advisory 2 weeks after release

### 6. Credit

We will credit you in:
- The security advisory
- The CHANGELOG.md file
- The GitHub release notes

If you prefer to remain anonymous, please let us know.

## Security Best Practices for Users

### Input Validation

Always validate and limit input sizes:

```go
const MaxJSONSize = 10 * 1024 * 1024 // 10MB limit

if len(jsonData) > MaxJSONSize {
    return errors.New("input too large")
}
```

### Use Context for Timeouts

Prevent long-running operations from blocking:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

images, err := stbimagego.LoadBatchConcurrent(ctx, imageData)
```

### Limit Concurrency

Control resource usage in production:

```go
runtime.GOMAXPROCS(4) // Limit to 4 CPUs
```

### Monitor Memory Usage

For large-scale processing, monitor memory:

```go
var m runtime.MemStats
runtime.ReadMemStats(&m)
if m.Alloc > 1*1024*1024*1024 { // 1GB
    log.Warn("High memory usage detected")
}
```

## Built-in DoS Protections

These guards ship enabled by default. Each is configurable so callers can tune
or disable it; all are conservative enough not to affect legitimate inputs.

| Module | Protection | Knob (default) |
|--------|------------|----------------|
| jsmn-go | Max input size + max token count | `ParseWithConfig` / `Config.MaxInputSize` (100 MB), `MaxTokens` (1,000,000); `StrictConfig` is tighter |
| tinyxml2-go | Max input size, node count, nesting depth | `ParseWithConfig` / `Config` |
| dr-wav-go | Data/`fmt` chunk allocation capped to bytes actually present (a size header can't force an OOM) | always on |
| stb-image-go | Decode-bomb guard — rejects images over a pixel cap before decoding | `MaxImagePixels` (64 MP; 0 disables) |
| miniz-go | Decompression-bomb guard — caps DEFLATE/ZIP output | `MaxDecompressedSize` (256 MiB; 0 disables) |

Defense in depth on top of these: set application-level size limits, use context
timeouts, and limit `GOMAXPROCS` for concurrent batch APIs.

## Known Security Considerations

### DoS via Large Inputs

**Affected Modules**: All parsing modules (jsmn-go, tinyxml2-go, etc.)

**Risk**: Unbounded input can cause memory exhaustion.

**Mitigation**: built-in limits are enabled by default (see *Built-in DoS
Protections* above). Additionally set application-level input-size limits, use
context timeouts, and monitor memory usage for defense in depth.

### Parallel Processing Amplification

**Affected Modules**: All modules with concurrent processing

**Risk**: Parallel processing can amplify memory usage (e.g., 4x for 4 CPU cores)

**Mitigation**:
- Limit GOMAXPROCS in production
- Process data in batches
- Use streaming APIs when available

### Malicious File Parsing

**Affected Modules**: stb-image-go, stb-truetype-go, dr-wav-go

**Risk**: Malformed image/font/audio files can cause excessive memory allocation.

**Mitigation**: built-in guards reject the common attacks before allocating —
stb-image rejects decode bombs (`MaxImagePixels`), dr-wav caps chunk allocations
to the bytes present, and miniz caps decompression output (`MaxDecompressedSize`).
For additional safety, sandbox untrusted file processing and set
application-level size limits.

## Security Audit Status

**Last Audit**: Not yet conducted
**Next Audit**: Planned for v1.0 release

We welcome security researchers to audit our code. If you're interested in performing a security audit, please contact us.

## Dependencies

SafeHeaders-Go has **zero external dependencies** (pure stdlib). This minimizes supply chain attack surface.

## Vulnerability Disclosure History

No vulnerabilities have been publicly disclosed yet.

---

**Maintainer**: @alikatgh
**Last Updated**: 2026-06-23
