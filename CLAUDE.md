# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`ping-itor` — module path `github.com/maxlesscode/ping-itor`, Go 1.26+.

This is a greenfield project. Update this file as architecture takes shape.

## Commands

```bash
# Build
go build ./...

# Test (race detector required)
go test -race ./...

# Single test
go test -race -run TestFoo ./path/to/package

# Lint
golangci-lint run

# Vet
go vet ./...

# Format
gofmt -w .
```

## Architecture

No source code yet — architecture notes will be added here once the package structure is established. Key decisions to document: entry points (`cmd/`), core domain packages, external integrations.
