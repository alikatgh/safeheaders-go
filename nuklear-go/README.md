# nuklear-go: Go Port of nuklear.h (Immediate-Mode GUI)

This is an early, stubbed port of the [nuklear.h](https://github.com/Immediate-Mode-UI/Nuklear/blob/master/nuklear.h) single-header C library for immediate-mode GUI. The goal is to provide a safe, zero-CGO Go rewrite with opt-in concurrency (e.g., goroutine-based rendering for multi-panel UIs). However, the current implementation is minimal and non-functional—it's a placeholder with empty stubs for core logic, intended as a starting point for community contributions. It does not yet render any UI elements or handle input/output, and the concurrency is demonstrative but adds no real value.

## Current Status
This port is a proof-of-concept stub and not ready for use. Key features like window management, buttons, input handling, and actual drawing are missing. The concurrent rendering spawns goroutines but calls empty functions, providing no speedup and potential overhead. It's here to spark PRs for a full port—focus on zero-CGO safety and Go patterns.

## Limitations
- 100% stubbed—no actual GUI rendering, input, or output (needs integration like ebiten or SDL).
- Concurrency is fake (spawns goroutines for empty work)—adds complexity without benefit.
- No state management (Context is empty struct).
- Not usable—PRs for minimal working GUI (e.g., button/label) welcome to address C-to-Go translation challenges like immediate-mode state.