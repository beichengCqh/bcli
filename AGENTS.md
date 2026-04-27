# Repository Guidelines

## Project Structure & Module Organization

This is a Go CLI project for `bcli`, a personal command center with CLI, TUI, MCP, profile, credential, and utility features.

- `main.go` and `cmd/bcli/`: process entry points.
- `internal/cli/`: command parsing, help text, exit codes, and user-facing CLI behavior.
- `internal/core/`: reusable business logic shared by CLI, TUI, and MCP.
- `internal/storage/`: config file and credential-store adapters.
- `internal/tui/`: Bubble Tea terminal UI.
- `internal/mcp/`: MCP server adapter.
- `md/`: private design notes and planning documents.

Keep new business rules in `internal/core` when they must be reused across entry points.

## Build, Test, and Development Commands

- `go build -o bcli .`: build the root binary.
- `go build -o bcli ./cmd/bcli`: build the `cmd/bcli` entry point.
- `go test ./...`: run all tests.
- `go test ./internal/tui`: run only TUI tests while iterating.
- `./bcli tui`: launch the terminal UI after building.
- `./bcli mcp serve`: run the MCP server over stdio.

Run `gofmt` on touched Go files before finishing changes.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs for indentation, `gofmt` for layout, and concise package-level APIs. Prefer small, explicit functions over broad abstractions. Keep CLI/TUI/MCP adapters thin; place shared behavior in `internal/core`.

Name tests as `Test<Behavior>` and keep fake stores local to the relevant test package unless reused widely.

## Testing Guidelines

Tests use Go’s standard `testing` package. Add focused unit tests for new behavior, especially command parsing, profile/auth handling, and TUI state transitions. For credential-related changes, assert that secrets are stored through `auth.Service` and never leaked into output or error messages.

Always run `go test ./...` before handing off changes.

## Commit & Pull Request Guidelines

Recent commits use Conventional Commit style, for example `feat: add profile management tui`. Prefer short imperative titles such as `fix: avoid leaking credential errors` or `feat: add tools page to tui`.

Pull requests should include a brief summary, test results, and any user-facing command or TUI changes. Link related issues when available. Include screenshots or terminal recordings for substantial TUI changes.

## Security & Configuration Tips

Do not commit secrets, tokens, `.env` files, or local credential data. Runtime profile config defaults to `~/.bcli/configs/`; passwords belong in the system credential store, not JSON config or logs.
