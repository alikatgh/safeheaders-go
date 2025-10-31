# Examples

This directory contains working examples for SafeHeaders-Go modules.

## Running Examples

```bash
# JSON tokenizer demo
cd jsmn-demo
go run main.go

# Font cache demo (if available)
cd truetype-demo
go run main.go
```

## Available Examples

- **jsmn-demo** - JSON tokenization with parallel parsing
- More examples coming soon!

## Creating Your Own

Each example is a standalone Go program. To create your own:

1. Create a new directory in `examples/`
2. Initialize with `go mod init example-name`
3. Add your code in `main.go`
4. Run with `go run main.go`
