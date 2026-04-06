# dakera-go

Go SDK for the Dakera AI agent memory platform — idiomatic Go client with full async support.

## Key Commands
```bash
go build ./...          # Build
go test ./...           # Run tests (set DAKERA_API_URL + DAKERA_API_KEY for integration tests)
go vet ./...            # Vet
gofmt -w .              # Format
go mod tidy             # Sync dependencies
```

## Architecture
- `client.go` — DakeraClient struct; HTTP transport, auth header injection, retries
- `types.go` — Memory, Session, Agent, KPI, Namespace request/response structs
- `events.go` — Server-sent events (SSE) streaming support
- `errors.go` — DakeraError type; HTTP status → typed error mapping
- `client_test.go` — Integration tests

## Conventions
- Go 1.21+; standard library HTTP client preferred over third-party for core transport
- Exported symbols have godoc comments
- Version matches server SDK version (e.g., v0.9.13) — tagged as `v0.9.x`
- SDK batch: all 4 SDKs (py, js, rs, go) sync together after a server API change
