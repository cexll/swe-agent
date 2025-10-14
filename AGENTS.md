# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` holds the HTTP server entry point; compiled binary name is `swe-agent`.
- `internal/` contains production code grouped by responsibility (e.g., `webhook/`, `executor/`, `github/`, `provider/`).
- `internal/provider/claude` and `internal/provider/codex` host AI integrations; keep shared logic in `internal/provider`.
- Tests live beside source files; golden data resides under `internal/github` fixtures.

## Build, Test, and Development Commands
- `make build` – compiles the Go service into `./swe-agent`.
- `make run` – runs the server against local env vars (defaults to port 8000).
- `make test` / `go test ./...` – runs the full unit test suite.
- `make docker-build` – builds the runtime image with Claude Code & Codex CLIs baked in.
- `make docker-run` – launches the container with `.env` configuration mapped in.

## Coding Style & Naming Conventions
- Follow standard Go formatting via `go fmt`; lint with `go vet` (both wired into the `make lint` target).
- Keep functions shallow; prefer small, composable helpers inside `internal/` packages.
- Use snake_case for environment variables (`GITHUB_WEBHOOK_SECRET`), and lower-kebab for Docker images/branches (`swe-agent/<issue>-<timestamp>`).
- Log messages should start with component tags (e.g., `[Claude]`, `[Codex]`) to match existing output.

## Testing Guidelines
- Tests use Go’s built-in `testing` package; mirror production package names (e.g., `internal/executor/task_test.go`).
- Aim to preserve ≥70% coverage; new features should include focused regression tests.
- Name tests with behavior intent (`TestGenerateCode_Integration`) and group scenarios with table-driven patterns where possible.
- Run `go test ./... -cover` before opening a pull request; stash artifacts like `coverage.out` from version control unless explicitly needed.

## Commit & Pull Request Guidelines
- Follow the conventional, lowercase prefixes observed in history (`feat:`, `chore:`, `build:`); keep subject lines under 72 characters.
- Reference GitHub issues in the body when applicable, and describe behavior changes plus validation steps.
- Pull requests should: summarize the change, note provider/runtime impacts (CLI versions, ports), attach screenshots for UI-affecting updates, and list manual verification commands.

## Security & Configuration Tips
- Store secrets in `.env`; never commit private keys. Required fields include `GITHUB_APP_ID`, `GITHUB_PRIVATE_KEY`, `GITHUB_WEBHOOK_SECRET`, and `ANTHROPIC_API_KEY` or OpenAI credentials.
- Default port is 8000; update docs/config if you change it.
- When testing Claude or Codex flows, ensure the CLIs are available locally or build via `make docker-build` to reproduce container behavior.
