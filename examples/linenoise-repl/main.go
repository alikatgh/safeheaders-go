// Package main provides a demonstration of the linenoise-go library.
//
// This is a simple REPL (Read-Eval-Print Loop) that showcases:
// - Line editing with history
// - Tab completion
// - Inline hints
// - Command execution
//
// Usage:
//
//	go run main.go
//
// Available commands:
//   - help: Show available commands
//   - history: Show command history
//   - clear: Clear screen
//   - save <file>: Save history to file
//   - load <file>: Load history from file
//   - exit/quit: Exit the REPL
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alikatgh/safeheaders-go/linenoise-go"
)

var commands = []string{
	"help",
	"history",
	"clear",
	"save",
	"load",
	"exit",
	"quit",
	"echo",
	"reverse",
}

func main() {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  Linenoise-Go Demo REPL")
	fmt.Println("  Type 'help' for available commands")
	fmt.Println("  Press Tab for completion, Ctrl+D or 'exit' to quit")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Set up completion callback
	linenoise.SetCompletionCallback(func(line string) []string {
		var completions []string
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, line) {
				completions = append(completions, cmd)
			}
		}
		return completions
	})

	// Set up hints callback
	linenoise.SetHintsCallback(func(line string) (string, bool) {
		switch line {
		case "help":
			return " - Show available commands", false
		case "history":
			return " - Show command history", false
		case "clear":
			return " - Clear screen", false
		case "save":
			return " <file> - Save history to file", false
		case "load":
			return " <file> - Load history from file", false
		case "exit", "quit":
			return " - Exit the REPL", false
		case "echo":
			return " <text> - Echo text back", false
		case "reverse":
			return " <text> - Reverse text", false
		}
		return "", false
	})

	// Load history if exists
	linenoise.LoadHistory(".linenoise_history")

	// Main REPL loop
	for {
		line, err := linenoise.Line("repl> ")
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				break
			}
			if err == linenoise.ErrInterrupted {
				fmt.Println("\n(Ctrl+C pressed, use 'exit' or Ctrl+D to quit)")
				continue
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Add to history
		linenoise.AddHistory(line)

		// Process command
		if !processCommand(line) {
			break
		}
	}

	// Save history on exit
	if err := linenoise.SaveHistory(".linenoise_history"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save history: %v\n", err)
	}
}

func processCommand(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return true
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "help":
		showHelp()

	case "history":
		showHistory()

	case "clear":
		fmt.Print("\x1b[H\x1b[2J")

	case "save":
		if len(args) == 0 {
			fmt.Println("Usage: save <filename>")
		} else {
			if err := linenoise.SaveHistory(args[0]); err != nil {
				fmt.Printf("Error saving history: %v\n", err)
			} else {
				fmt.Printf("History saved to %s\n", args[0])
			}
		}

	case "load":
		if len(args) == 0 {
			fmt.Println("Usage: load <filename>")
		} else {
			if err := linenoise.LoadHistory(args[0]); err != nil {
				fmt.Printf("Error loading history: %v\n", err)
			} else {
				fmt.Printf("History loaded from %s\n", args[0])
			}
		}

	case "exit", "quit":
		fmt.Println("Goodbye!")
		return false

	case "echo":
		fmt.Println(strings.Join(args, " "))

	case "reverse":
		text := strings.Join(args, " ")
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		fmt.Println(string(runes))

	default:
		fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", cmd)
	}

	return true
}

func showHelp() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("══════════════════")
	fmt.Println("  help            - Show this help message")
	fmt.Println("  history         - Show command history")
	fmt.Println("  clear           - Clear the screen")
	fmt.Println("  save <file>     - Save command history to file")
	fmt.Println("  load <file>     - Load command history from file")
	fmt.Println("  echo <text>     - Echo text back")
	fmt.Println("  reverse <text>  - Reverse the text")
	fmt.Println("  exit, quit      - Exit the REPL")
	fmt.Println()
	fmt.Println("Keyboard Shortcuts:")
	fmt.Println("═══════════════════")
	fmt.Println("  Tab             - Auto-complete commands")
	fmt.Println("  Up/Down         - Navigate history")
	fmt.Println("  Ctrl+A          - Move to beginning of line")
	fmt.Println("  Ctrl+E          - Move to end of line")
	fmt.Println("  Ctrl+K          - Delete to end of line")
	fmt.Println("  Ctrl+U          - Delete entire line")
	fmt.Println("  Ctrl+W          - Delete previous word")
	fmt.Println("  Ctrl+L          - Clear screen")
	fmt.Println("  Ctrl+C          - Cancel current line")
	fmt.Println("  Ctrl+D          - Exit (EOF)")
	fmt.Println()
}

func showHistory() {
	// Note: We can't access history directly with the current API
	// This is a simplified version
	fmt.Println("\nCommand history is maintained across sessions.")
	fmt.Println("Use Up/Down arrows to navigate through history.")
	fmt.Println("History is saved to .linenoise_history on exit.")
	fmt.Println()
}
