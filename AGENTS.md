# Repository Guidelines

## Project Structure & Module Organization

Komari is a Go server application. `main.go` delegates CLI setup to `cmd/`; `internal/` contains private lifecycle, configuration, scheduler, plugin, and metric-store code; and `pkg/` provides reusable metric, RPC, JavaScript-runtime, and time packages. Database models and persistence logic live in `database/`, wire protocols in `protocol/`, shared helpers in `utils/`, and HTTP/WebSocket handlers in `web/`. Tests sit beside their implementation as `*_test.go`. The default UI is built in the separate `komari-monitor/komari-web` repository and packed into `web/public/defaultTheme/`; see `web/public/readme.md`.

## Build, Test, and Development Commands

- `go mod download` installs module dependencies declared in `go.mod`.
- `go build -o komari .` builds the server after the frontend embed archive has been prepared.
- `go run . server --listen 127.0.0.1:25774 --database ./data/komari.db` starts a local instance with an isolated SQLite database.
- `go test ./...` runs the complete test suite; use `go test ./internal/metricstore -run TestName` for focused iteration.
- `go vet ./...` performs standard static checks.
- `gofmt -w path/to/file.go` formats changed Go files before review.

The CI frontend action clones `komari-web`, runs `npm install` and `npm run build`, then creates `web/public/defaultTheme/dist.tar.zst`. Reproduce those steps when a clean checkout lacks embedded assets.

## Coding Style & Naming Conventions

Follow idiomatic Go and let `gofmt` define tabs, spacing, and import grouping. Use short, lowercase package names; exported identifiers in `PascalCase`; local variables in `camelCase`; and descriptive filenames consistent with nearby code. Keep HTTP handlers thin and place reusable domain logic in `internal/` or `pkg/`. Wrap errors with context using `%w`, and do not log secrets, tokens, DSNs, or passwords.

## Testing Guidelines

Use Go's `testing` package; existing tests also use `testify`. Name tests `TestBehavior` and favor table-driven cases for validation, protocol, and persistence boundaries. Add regression coverage beside every bug fix. Tests must be deterministic, use temporary directories or isolated databases, and avoid live network dependencies.

## Commit & Pull Request Guidelines

History favors concise Conventional Commit subjects such as `fix(plugin): detect protocol upgrades` and `feat(terminal): support session reattachment`; use `feat`, `fix`, `test`, `docs`, `refactor`, or `chore` with an optional scope. Keep each commit focused. Pull requests should explain the problem and solution, link relevant issues, list verification commands, and include screenshots for UI-visible changes. Call out migrations, configuration changes, and compatibility risks explicitly.
