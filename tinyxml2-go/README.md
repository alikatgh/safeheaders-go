# tinyxml2-go

[![Go Report Card](https://goreportcard.com/badge/github.com/alikatgh/safeheaders-go/tinyxml2-go)](https://goreportcard.com/report/github.com/alikatgh/safeheaders-go/tinyxml2-go)
[![Go CI](https://github.com/alikatgh/safeheaders-go/actions/workflows/go.yml/badge.svg)](https://github.com/alikatgh/safeheaders-go/actions/workflows/go.yml)

An idiomatic, zero-CGO Go port of the `tinyxml2.h` C library for XML parsing.

## Current Status: v0.1.0

This version provides a minimal, pure-Go parser that correctly processes the XML declaration and identifies the root element of a document.

**Features:**
- ✅ Parses XML declaration (`<?xml ... ?>`)
- ✅ Identifies the root element tag

**Work in Progress:**
- 🔜 **v0.2.0**: Full DOM tree parsing (children, attributes, text).
- 🔜 **v0.3.0**: Concurrent tree traversal helpers.

The full port is tracked in our main project issues.