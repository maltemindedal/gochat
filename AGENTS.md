# GoChat Agent Notes

## Toolchain
- Go version is `1.25.3` in `go.mod`; CI runs on `1.25.3` plus a `1.24.x` / `1.25.x` build matrix.
- The only direct third-party runtime dependency is `github.com/gorilla/websocket`.

## Entry Points
- Main binary entrypoint is `cmd/server/main.go`.
- Startup order in `main()`: `server.NewConfigFromEnv()` -> `server.SetConfig()` -> `server.StartHub()` -> `server.SetupRoutes()` -> `server.CreateServer()` -> `server.StartServer()`.
- Routes are defined in `internal/server/routes.go`: `/` health check, `/ws` WebSocket endpoint, `/test` built-in HTML test page.

## Commands
- `make build` runs `make fmt` and `make vet` first, then builds `./cmd/server` into `./bin/`.
- `make build-raw` skips fmt/vet; use it when you only need a fast compile check.
- `make test` runs `go test -v -race ./...`.
- Focused suites: `make test-unit`, `make test-integration`.
- Coverage targets are split: `make test-coverage`, `make test-coverage-unit`, `make test-coverage-integration`.
- `make ci-local` is the heaviest local verification: `clean fmt vet lint test-coverage security-scan deps-check build`.
- If you change dependencies, run `go mod tidy`; CI fails if `go.mod` / `go.sum` become dirty after tidy.

## Dev Server Gotchas
- `make dev` uses Air with `.air.toml`.
- Air excludes `test/` and `bin/`, so changing tests will not restart the dev server.

## Runtime Config
- Runtime config is read directly from process environment in `internal/server/config.go`; the app does not load `.env` files itself.
- Relevant env vars: `SERVER_PORT`, `ALLOWED_ORIGINS`, `MAX_MESSAGE_SIZE`, `RATE_LIMIT_BURST`, `RATE_LIMIT_REFILL_INTERVAL`.
- Default config is strict enough to affect tests: only `http://localhost:8080` is allowed by default unless tests call `server.SetConfig()`.

## WebSocket Behavior
- Message format is JSON `{"content":"..."}` (`internal/server/types.go`).
- The hub broadcasts to all connected clients except the sender.
- Invalid JSON is ignored, not echoed back.
- Oversized messages trip the websocket read limit and close the sender connection.
- Rate-limited messages are dropped and the connection stays open.
- `Client.writePump()` drains queued messages into the same WebSocket frame separated by newlines; tests that read messages must handle batched payloads.

## Test Patterns
- Shared test helpers live in `test/testhelpers`.
- Integration tests that depend on custom origins, size limits, or rate limits update global config with `server.SetConfig(cfg)` and restore defaults with `t.Cleanup(func() { server.SetConfig(nil) })`.
- For focused runs use standard `go test` flags, e.g. `go test -v -race ./test/integration -run TestWebSocketRateLimiting`.

## CI Reality
- GitHub Actions in `.github/workflows/ci.yml` runs: `go mod verify`, `go mod download`, `go build -v ./...`, `go test -v -race -coverprofile=coverage.out ./...`, `golangci-lint`, `govulncheck`, and a `go mod tidy` cleanliness check.
- CI does not currently run `gosec`; `make security-scan` is broader than hosted CI.
