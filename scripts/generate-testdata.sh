#!/bin/bash
# Generate test data files for SafeHeaders-Go
# This script is a wrapper around the Go-based test data generator

set -e

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    exit 1
fi

# Run the Go-based generator (much faster than bash loops)
go run "$(dirname "$0")/generate-testdata.go"
