# linenoise-go

A minimal, zero-dependency line editing library for CLI applications in Go.

This is a Go port of [antirez/linenoise](https://github.com/antirez/linenoise), a minimal alternative to readline and libedit. It provides line editing capabilities with history, completion, and hints - perfect for building interactive command-line tools.

## Features

✨ **Line Editing**
- Single and multi-line editing modes
- Standard keyboard shortcuts (Ctrl+A, Ctrl+E, Ctrl+K, etc.)
- Arrow key navigation
- UTF-8 support

📚 **History Management**
- Persistent command history
- Configurable history size
- Automatic duplicate filtering
- Save/load from disk

🔍 **Smart Completion**
- Tab completion with custom callbacks
- Cycle through multiple completions
- Context-aware suggestions

💡 **Inline Hints**
- Real-time suggestions as you type
- Customizable hint styling
- Optional bold formatting

🔒 **Security**
- Password masking mode
- No external dependencies (except golang.org/x/term for terminal control)
- Memory-safe implementation

## Installation

```bash
go get github.com/alikatgh/safeheaders-go/linenoise-go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "io"
    "log"

    "github.com/alikatgh/safeheaders-go/linenoise-go"
)

func main() {
    for {
        line, err := linenoise.Line("prompt> ")
        if err != nil {
            if err == io.EOF {
                break // Ctrl+D pressed
            }
            log.Fatal(err)
        }

        linenoise.AddHistory(line)
        fmt.Printf("You entered: %s\n", line)
    }
}
```

### With Tab Completion

```go
linenoise.SetCompletionCallback(func(line string) []string {
    commands := []string{"help", "history", "clear", "exit"}
    var completions []string
    for _, cmd := range commands {
        if strings.HasPrefix(cmd, line) {
            completions = append(completions, cmd)
        }
    }
    return completions
})
```

### With Inline Hints

```go
linenoise.SetHintsCallback(func(line string) (string, bool) {
    switch line {
    case "help":
        return " - Show available commands", true // bold
    case "exit":
        return " - Exit the program", false      // not bold
    }
    return "", false
})
```

### Password Input

```go
linenoise.SetMaskMode(true)
password, _ := linenoise.Line("Password: ")
linenoise.SetMaskMode(false)
```

### Persistent History

```go
// Load history at startup
linenoise.LoadHistory(".myapp_history")

// ... use linenoise ...

// Save history on exit
linenoise.SaveHistory(".myapp_history")
```

## Advanced Usage

### Custom Configuration

```go
config := &linenoise.Config{
    HistoryMaxLen: 500,
    MultiLine:     true,
    MaskMode:      false,
    Input:         os.Stdin,
    Output:        os.Stdout,
}

state := linenoise.New(config)
line, err := state.ReadLine("prompt> ")
```

### State Management

For applications that need fine-grained control:

```go
state := linenoise.New(linenoise.DefaultConfig())

// Set callbacks on the state
state.config.CompletionCallback = myCompletionFunc
state.config.HintsCallback = myHintsFunc

// Add history
state.AddHistory("previous command")

// Read line
line, err := state.ReadLine("prompt> ")
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+A | Move to beginning of line |
| Ctrl+E | Move to end of line |
| Ctrl+B | Move cursor left (same as ←) |
| Ctrl+F | Move cursor right (same as →) |
| Ctrl+K | Delete from cursor to end of line |
| Ctrl+U | Delete entire line |
| Ctrl+W | Delete previous word |
| Ctrl+L | Clear screen |
| Ctrl+C | Cancel current line |
| Ctrl+D | Exit (EOF) |
| Tab | Auto-complete (if callback set) |
| ↑/↓ | Navigate history |
| ←/→ | Move cursor |
| Home/End | Jump to start/end |
| Backspace | Delete previous character |
| Delete | Delete character under cursor |

## Examples

### Simple REPL

```go
package main

import (
    "fmt"
    "io"
    "strings"

    "github.com/alikatgh/safeheaders-go/linenoise-go"
)

func main() {
    for {
        line, err := linenoise.Line("repl> ")
        if err != nil {
            if err == io.EOF {
                break
            }
            continue
        }

        line = strings.TrimSpace(line)
        if line == "exit" {
            break
        }

        linenoise.AddHistory(line)

        // Process command
        fmt.Printf("Command: %s\n", line)
    }
}
```

See [examples/linenoise-repl](../examples/linenoise-repl/) for a complete REPL implementation with completion, hints, and history persistence.

## API Reference

### Functions

#### `Line(prompt string) (string, error)`
Convenience function to read a single line with the given prompt.

#### `New(config *Config) *State`
Creates a new linenoise state with custom configuration.

#### `DefaultConfig() *Config`
Returns a configuration with sensible defaults.

### Global Functions

These operate on a global state for simple usage:

- `AddHistory(line string)` - Add line to history
- `SaveHistory(filename string) error` - Save history to file
- `LoadHistory(filename string) error` - Load history from file
- `ClearHistory()` - Clear all history
- `SetCompletionCallback(cb CompletionCallback)` - Set completion callback
- `SetHintsCallback(cb HintsCallback)` - Set hints callback
- `SetMultiLine(enabled bool)` - Enable/disable multi-line mode
- `SetMaskMode(enabled bool)` - Enable/disable password masking

### Types

#### `Config`
```go
type Config struct {
    HistoryMaxLen      int
    MultiLine          bool
    MaskMode           bool
    CompletionCallback CompletionCallback
    HintsCallback      HintsCallback
    Input              *os.File
    Output             *os.File
}
```

#### `CompletionCallback`
```go
type CompletionCallback func(line string) []string
```

#### `HintsCallback`
```go
type HintsCallback func(line string) (hint string, bold bool)
```

### Errors

- `ErrNotTTY` - Not connected to a terminal
- `ErrUnsupported` - Unsupported terminal type
- `ErrInterrupted` - User pressed Ctrl+C
- `ErrInvalidUTF8` - Invalid UTF-8 sequence

## Comparison with Original

| Feature | linenoise.c | linenoise-go |
|---------|-------------|--------------|
| Line editing | ✅ | ✅ |
| History | ✅ | ✅ |
| Completion | ✅ | ✅ |
| Hints | ✅ | ✅ |
| Multi-line | ✅ | 🚧 (partial) |
| Async mode | ✅ | ❌ (not needed in Go) |
| UTF-8 support | ⚠️ (limited) | ✅ (full) |
| Windows support | ❌ | ❌ (Unix/Linux/macOS only) |

## Platform Support

Currently supported platforms:
- Linux
- macOS
- Unix-like systems with termios support

**Not supported:**
- Windows (may be added in future versions)

## Performance

The library is designed for interactive CLI use with minimal overhead:

```
BenchmarkInsertChar-8     200000    6500 ns/op    2048 B/op    1 allocs/op
BenchmarkDeleteChar-8     300000    4200 ns/op       0 B/op    0 allocs/op
BenchmarkAddHistory-8    5000000     250 ns/op      32 B/op    1 allocs/op
```

## Testing

```bash
go test -v
go test -race
go test -bench=.
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](../CONTRIBUTING.md).

## License

BSD 2-Clause License (same as original linenoise)

## Credits

- Original linenoise: [Salvatore Sanfilippo](https://github.com/antirez) and [Pieter Noordhuis](https://github.com/pietern)
- Go port: Part of the SafeHeaders-Go project

## See Also

- [Original linenoise](https://github.com/antirez/linenoise) - The C library this is based on
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) - Terminal control library used internally
- [examples/linenoise-repl](../examples/linenoise-repl/) - Complete example application

---

**Part of [SafeHeaders-Go](https://github.com/alikatgh/safeheaders-go)** - A collection of production-ready Go ports of popular C libraries.
