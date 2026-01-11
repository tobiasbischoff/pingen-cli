# Repository Guidelines

## Project Structure & Module Organization
- `cmd/pingen-cli/`: Go entrypoint for the CLI.
- `internal/pingen/`: Core logic (API client, config handling).
- `docs/`: Reference materials (`swagger-docs.json` for API, `cli-guidelines.md` for UX).
- `bin/`: Local build output (ignored by git).

## Build, Test, and Development Commands
- `go build -o ./bin/pingen-cli ./cmd/pingen-cli`: Build a local binary.
- `./bin/pingen-cli --help`: Run the built CLI.
- `go run ./cmd/pingen-cli --help`: Run without building a binary.
- `go test ./...`: Run tests (none currently, but keep this wired in).

## Coding Style & Naming Conventions
- Use standard Go formatting: run `gofmt -w` on modified `.go` files.
- Package layout follows Go convention: `cmd/` for entrypoints, `internal/` for non‑exported packages.
- Public identifiers should be exported only when needed; keep helpers unexported.

## Testing Guidelines
- No test framework is set up yet. When adding tests, place them alongside code as `*_test.go`.
- Prefer table‑driven tests for API/config behaviors.
- Keep tests deterministic; avoid real network calls (mock HTTP instead).

## Commit & Pull Request Guidelines
- Git history is not available in this repository, so no established commit message convention is known.
- If you add commits later, use clear, imperative messages (e.g., “Add letter send command”).
- PRs should include a brief summary, how to test, and any relevant API or CLI behavior changes.

## Security & Configuration Tips
- Config is stored at `$XDG_CONFIG_HOME/pingen/config.json` or `~/.config/pingen/config.json`.
- Prefer secrets via env or files over flags; avoid logging tokens.
- Use staging by default; pass `--env production` only for real sends.
