# 02 · Modules, packages and go.work

> **Objectives:** Understand the difference between a Go package and a Go module,
> learn how `go.mod` pins a module's identity and dependencies, and see how
> `go.work` stitches nine independent modules into a single development workspace.
> Estimated time: 20 minutes.

---

## What this actually means (plain English)

No jargon — here's what the ideas in this lesson *actually* mean, and why they matter.

- A **package** is a folder of `.go` files that share a `package` declaration.
  Think of it as one chapter in a book — self-contained but part of a larger work.
- A **module** is a directory tree with a `go.mod` at its root. It is the unit
  you version, publish, and depend on. One `go.mod` = one module, which may
  contain many packages.
- A **workspace** (`go.work`) is a local override that says "when building any of
  these modules, prefer the local source tree instead of downloading from the
  internet." Nothing gets published — it's purely a developer convenience.
- **Replace directives** serve a similar role inside a single `go.mod`: they
  redirect an import path to a local directory. Examples use them so they compile
  without a workspace.
- **`GOWORK=off`** disables the workspace for a single command, making Go behave
  exactly as it would for a downstream user who has only that one module checked
  out.

**Why it matters:** SafeHeaders-Go is nine separate modules that must be
developed together. Without a workspace, every cross-module change would require
publishing a new version tag before the next module could depend on it — a
workflow that collapses under rapid iteration.

---

## One module up close — `jsmn-go/go.mod`

Every Go module starts with a two-line minimum:

```go
// jsmn-go/go.mod
module github.com/alikatgh/safeheaders-go/jsmn-go

go 1.23
```

- `module` declares the canonical import path. Any file anywhere on earth can
  write `import "github.com/alikatgh/safeheaders-go/jsmn-go"` and Go's toolchain
  knows exactly where to fetch it from.
- `go 1.23` is the minimum language version this module requires. You can write
  `go 1.23` features freely; the toolchain enforces the floor.
- There is no `require` block because `jsmn-go` has no third-party dependencies —
  a deliberate design choice for a security-hardening library.

All nine modules in this repo follow the same pattern:
`github.com/alikatgh/safeheaders-go/<name>`, replacing `<name>` with the
subdirectory (e.g. `dr-wav-go`, `miniz-go`).

---

## The workspace — `go.work`

When you want to work on several modules simultaneously without publishing
intermediate tags, Go 1.18 introduced workspace mode:

```go
// go.work  (repo root)
go 1.24.0

toolchain go1.24.7

use (
    ./cgltf-go
    ./cjson-go
    ./dr-wav-go
    ./jsmn-go
    ./linenoise-go
    ./miniz-go
    ./stb-image-go
    ./stb-truetype-go
    ./tinyxml2-go
)
```

Each `use` line tells the toolchain: "this directory contains a real `go.mod`;
treat it as the authoritative source for that module path."

The effect is invisible but powerful: if `cjson-go` ever imported `jsmn-go`,
the workspace would resolve that import straight to `./jsmn-go/` on disk
instead of downloading a published version. Right now none of the nine modules
depend on each other (each is a standalone port), but the workspace still
matters for cross-cutting tooling — `go work sync`, `govulncheck`, and IDE
navigation all understand it.

!!! note "The toolchain line"
    `toolchain go1.24.7` pins the exact Go toolchain binary used to build the
    workspace. This is distinct from the minimum language version (`go 1.24.0`).
    Think of it as a `.nvmrc` for Go — it ensures everyone on the team compiles
    with the same binary.

---

## Why examples use `GOWORK=off` and replace directives

The `examples/` subdirectory contains standalone demonstration programs. Each
one is its **own module** with its own `go.mod`:

```go
// examples/json-parser/go.mod
module github.com/alikatgh/safeheaders-go/examples/json-parser

go 1.23

require github.com/alikatgh/safeheaders-go/jsmn-go v0.5.0

replace github.com/alikatgh/safeheaders-go/jsmn-go => ../../jsmn-go
```

Two things are happening here:

1. **`require ... v0.5.0`** — the example declares an explicit published version.
   This is how a real downstream user would depend on the library.
2. **`replace ... => ../../jsmn-go`** — during local development, Go is told to
   use the local directory instead of fetching `v0.5.0` from the internet.

Notice that `examples/` is **not** listed in `go.work`. That's intentional.
Example modules are treated as if they are external consumers. Keeping them out
of the workspace means the workspace resolver doesn't override their `replace`
directives in unexpected ways.

When the Makefile builds examples it also sets `GOWORK=off`:

```makefile
# Makefile (examples target, abridged)
examples:
    @for ex in examples/json-parser examples/jsmn-demo \
                examples/production-usage examples/linenoise-repl; do \
        (cd $$ex && GOWORK=off go build ./...) || exit 1; \
        (cd $$ex && GOWORK=off go vet ./...)  || exit 1; \
    done
    @(cd examples/json-parser && GOWORK=off go run .) || exit 1
    @(cd examples/jsmn-demo   && GOWORK=off go run .) || exit 1
```

`GOWORK=off` prevents the parent workspace from leaking into the example's
build. The `replace` directive in the example's own `go.mod` still applies, so
the local source is used — but now exclusively through the `replace` mechanism,
exactly as an end-user would experience it after cloning only that subdirectory.

!!! warning "Forgetting GOWORK=off on examples"
    If you run `go build ./...` from the repo root **without** `GOWORK=off`, the
    workspace `use` blocks take effect and the `replace` directives inside
    example `go.mod` files may be silently ignored or shadowed. Always use
    `GOWORK=off` (or `cd` into the example directory and build from there) when
    testing examples as a standalone consumer.

---

## The MODULES variable — single source of truth in the Makefile

Managing nine modules means every `make` target — tests, linting, fuzzing,
dependency updates — must iterate over all of them. Rather than hardcoding the
list nine times, the Makefile declares it once:

```makefile
# Makefile
# Single source of truth for the list of Go modules. Keep alphabetized.
MODULES := cgltf-go cjson-go dr-wav-go jsmn-go linenoise-go miniz-go \
           stb-image-go stb-truetype-go tinyxml2-go
```

Every looping target then expands that variable:

```makefile
test:
    @for dir in $(MODULES); do \
        echo "Testing $$dir..."; \
        (cd $$dir && go test -v ./...) || exit 1; \
    done
```

The comment "Keep alphabetized" is load-bearing documentation: it signals that
this list is the canonical registry — if you add a new module, you update
`MODULES`, add a `use` line to `go.work`, and everything else (tests, lint,
security scans, fuzz) picks it up automatically.

!!! tip "Adding a tenth module"
    The checklist for adding a new module to this repo is exactly three steps:
    
    1. Create the directory with its own `go.mod` (correct module path, minimum Go version).
    2. Add a `use ./new-module` line to `go.work`.
    3. Add `new-module` (alphabetically) to `MODULES` in the Makefile.
    
    No other file needs changing for the module to participate in `make test`,
    `make lint`, `make fuzz`, and all other CI targets.

---

## Workspace vs. replace directives — when to use each

| Scenario | Tool |
|---|---|
| Multiple modules in one repo, developed together | `go.work` |
| One module that needs a local fork of a dependency | `replace` in `go.mod` |
| Simulating a downstream consumer in CI | `GOWORK=off` + `replace` |
| Pinning a specific Go toolchain for the whole repo | `toolchain` line in `go.work` |

---

!!! note "Try it"
    From the repo root, verify the workspace is valid and list all module paths:
    
    ```bash
    go work sync
    go list -m all
    ```
    
    Expected outcome: `go work sync` exits silently (no errors).
    `go list -m all` prints nine lines, one per module, e.g.:
    ```
    github.com/alikatgh/safeheaders-go/cgltf-go
    github.com/alikatgh/safeheaders-go/cjson-go
    ...
    github.com/alikatgh/safeheaders-go/tinyxml2-go
    ```
    
    Then confirm `GOWORK=off` isolates an example:
    
    ```bash
    cd examples/json-parser
    GOWORK=off go build ./...
    ```
    
    This should compile cleanly using the `replace` directive inside
    `examples/json-parser/go.mod` — no workspace involvement.

---

## Key takeaways

- A **package** is a folder; a **module** is a versioned unit with a `go.mod`
  that declares its import path and minimum Go version.
- `go.work` wires multiple local modules together for development without
  requiring published version tags — add a `use` line per module.
- Example programs are intentionally **outside** the workspace so they behave
  like real downstream consumers; `GOWORK=off` enforces that isolation in CI.
- The `MODULES` variable in the Makefile is the single authoritative list — one
  edit there propagates to tests, linting, fuzzing, and security scans.
- When you add a new module, the three-step checklist is: `go.mod`, `go.work`
  `use` line, `MODULES` entry.
