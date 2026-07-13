# Contributing to x-skills

Thank you for your interest in contributing!

## Prerequisites

- [Go](https://go.dev/dl/) 1.26.5 or later
- `make` (optional; all commands can be run with raw `go` tooling)

## Quick Start

```bash
# Clone the repository
git clone https://github.com/InkyQuill/x-skills.git
cd x-skills

# Build
go build ./...

# Run tests
go test -race ./...

# Lint
go vet ./...
```

To build the checkout and overwrite the normal installation on macOS or Linux:

```bash
./scripts/install-dev.sh
```

On Windows PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-dev.ps1
```

The installed `x-skills version` reports `dev`.

## Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes
4. Add tests for new functionality
5. Run `go test -race ./...` and `go vet ./...`
6. Commit with a descriptive message
7. Push and open a Pull Request

## Code Guidelines

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Add doc comments on all exported symbols
- Write table-driven tests
- Run `go vet ./...` before committing

### Architecture Decisions

Significant design decisions are recorded as ADRs in `docs/adr/`. When a choice
involves trade-offs worth weighing, write or update an ADR rather than only
changing code or docs.

Current behavior is maintained in the [CLI guide](./docs/cli.md), [TUI guide](./docs/tui.md), and [remote skills guide](./docs/remote-skills.md). Update the relevant guide and its documentation assertions when behavior changes; plans and specs are historical implementation inputs, not user documentation.

### Terminology

Project-specific terms are defined in [CONTEXT.md](./CONTEXT.md). Use that
vocabulary consistently — it keeps discussions, issues, and code commentary
precise.

## Reporting Issues

Use [GitHub Issues](https://github.com/InkyQuill/x-skills/issues). Include:

- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- If relevant, the output of `x-skills doctor`
