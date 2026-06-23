module github.com/alikatgh/safeheaders-go/examples/linenoise-repl

go 1.24.0

toolchain go1.24.4

require github.com/alikatgh/safeheaders-go/linenoise-go v0.0.0

require (
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
)

replace github.com/alikatgh/safeheaders-go/linenoise-go => ../../linenoise-go
