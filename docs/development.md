# Development Guide

## Build

```bash
# Using Makefile (recommended)
make build                    # Build binary
make run                      # Run application
make test                     # Run all tests
make test-coverage           # Run tests and generate coverage report
make test-coverage-html      # Generate HTML coverage report
make fmt                     # Format code
make lint                    # Code check
make check                   # Run all checks (format, check, test)
make clean                   # Clean build files
make all                     # Complete build process

# Manual build
go build -o swe-agent cmd/main.go

# Run
./swe-agent
```

## Code Formatting

```bash
# Using Makefile (recommended)
make fmt                      # Format code
make vet                      # Code check
make lint                     # Full check (includes format check)
make tidy                     # Tidy dependencies

# Manual operations
go fmt ./...                  # Format code
go vet ./...                  # Code check
go mod tidy                   # Tidy dependencies
```

## Testing

### Test Coverage

Overall: **84.7%** coverage across all modules

| Module            | Coverage |
|-------------------|----------|
| toolconfig        | 98.0%    |
| web               | 95.2%    |
| github/data       | **93.4%** ← **Updated with pagination tests** |
| prompt            | 92.3%    |
| dispatcher        | 91.6%    |
| webhook           | 89.6%    |
| executor          | 85.5%    |
| github            | 85.4%    |
| codex provider    | 85.3%    |
| claude provider   | 83.2%    |

### Run Tests

```bash
# Run all tests with coverage
go test ./... -cover

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Adding a New AI Provider

1. Create directory in `internal/provider/<name>/`
2. Implement `Provider` interface:
   ```go
   type Provider interface {
       GenerateCode(ctx, req) (*CodeResponse, error)
       Name() string
   }
   ```
3. Provider can choose:
   - Return `Files` list (Executor will apply these files)
   - Directly modify files in `req.RepoPath` (Executor will auto-detect)
4. Add case in `factory.go`
5. Add test file
6. Update documentation

## Docker Development

```bash
# Using Makefile (recommended)
make docker-build           # Build Docker image
make docker-run             # Run Docker container (requires .env file)
make docker-stop            # Stop and remove container
make docker-logs            # View container logs

# Manual Docker commands
docker build -t swe-agent .

# Run container
docker run -d \
  -p 8000:8000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e PROVIDER=codex \
  -e CODEX_MODEL=gpt-5-codex \
  --name swe-agent \
  swe-agent
```

### Docker Compose

```yaml
version: "3.8"

services:
  swe-agent:
    build: .
    ports:
      - "8000:8000"
    environment:
      - GITHUB_APP_ID=${GITHUB_APP_ID}
      - GITHUB_PRIVATE_KEY=${GITHUB_PRIVATE_KEY}
      - GITHUB_WEBHOOK_SECRET=${GITHUB_WEBHOOK_SECRET}
      - PROVIDER=codex
      - CODEX_MODEL=gpt-5-codex
      - TRIGGER_KEYWORD=/code
    restart: unless-stopped
```

## Dependencies

- **Go 1.25+** - Build and runtime environment
- **Codex CLI** / **Claude Code CLI** - AI code generation
- **GitHub CLI (`gh`)** - Git operations
- **Gorilla Mux** - HTTP routing

### AI Provider Support

Currently supported AI providers:

- **Codex** (Recommended) - Requires Codex CLI, optional `OPENAI_API_KEY`
- **Claude** (Anthropic) - Requires `ANTHROPIC_API_KEY`

Switch via environment variable `PROVIDER=codex` or `PROVIDER=claude`.

## Code Style

- Run `go fmt`
- Follow Linus's "good taste" principles
- Keep functions under 50 lines
- Avoid deep nesting
- Add unit tests (target coverage >75%)
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages

## Troubleshooting

### 1. Webhook not firing

Check:
- Is the GitHub App installed correctly
- Is the webhook URL reachable
- Does the webhook secret match
- Review the GitHub App's Recent Deliveries
- If the response code is 503, the job queue is full; retry later or increase `DISPATCHER_QUEUE_SIZE`

### 2. Codex/Claude API errors

Check:
- Is the API key correct
- Is the CLI installed properly (`codex --version` or `claude --version`)
- Has the API quota been exhausted
- Is the network connection stable

### 3. Git operations failing

Check:
- Is the `gh` CLI installed and authenticated (`gh auth status`)
- Does the GitHub App have Contents write permission
- Is there a branch name conflict
- Is the network connection stable

### 4. PR not created

Possible causes:
- The AI did not modify any files (analysis-only result)
- Git detected no changes
- Push failed (permission issue)

Check the logs:
```
[Codex] Command completed in 2.5s
No file changes detected in working directory (analysis/answer only)
```

### 5. Task stuck

- Check whether the 10-minute timeout triggered
- Compare the timestamps between `[Codex] Executing` and `Command completed` in the logs
- Manually test whether the codex command works