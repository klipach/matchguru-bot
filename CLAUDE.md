# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests with coverage
go test -v -coverprofile=coverage.out ./...

# Run a single test
go test -v ./auth/ -run TestParseBearerToken

# Lint
go vet ./...

# View coverage
go tool cover -func=coverage.out

# Deploy to GCP
make deploy

# Generate a Firebase JWT token for testing
# Requires a Firebase service account key at ./service_account_key.json
go run ./cmd/gentoken/main.go -uid TEST_USER_ID -apikey YOUR_FIREBASE_API_KEY
```

## Architecture

MatchGuru Bot is a **Google Cloud Functions Gen2** HTTP handler that provides AI-powered soccer/football analysis via streaming chat. It is deployed as a single serverless function (`Bot`) and fronted by Firebase Hosting.

### Request Flow

```
HTTP POST → Firebase JWT auth → parse BotRequest
  → load chat history (Firestore)
  → fetch fixture data (SportMonks API, if gameID provided)
  → render system prompt template (prompts/main.tmpl)
  → call OpenAI gpt-4o-mini-search-preview with streaming
  → pipe chunks through filters (external links, internal links)
  → stream response via SSE
```

### Package Responsibilities

- **`bot.go`** — Entry point. Registers `Bot` HTTP handler. Contains custom `http.RoundTripper` implementations: `modifyingRoundTripper` (strips `temperature`, injects `web_search_options` for OpenAI web search — required by `gpt-4o-mini-search-preview`) and `loggingRoundTripper`.
- **`auth/`** — Firebase ID token validation and bearer token extraction from `Authorization` header.
- **`chat/`** — Loads chat history from Firestore by userID + chatID; converts to `langchaingo` message format.
- **`contract/`** — `BotRequest` / `BotResponse` HTTP DTOs.
- **`fixture/`** — Fetches match fixture details from the SportMonks API.
- **`filter/`** — Stateful streaming filters: `ExternalLinkFilter` removes markdown `[text](url)` links; `InternalLinkFilter` converts `{Team Name|team name}` tokens into markdown links using static maps in `team.go` / `league.go`. Handles partial markdown across SSE chunk boundaries.
- **`log/`** — Custom `slog.Handler` writing JSON to stdout with Cloud Trace correlation.
- **`prompts/`** — Go text template (`main.tmpl`) for the LLM system prompt; rendered with user timezone and fixture context.
- **`cmd/gentoken/`** and **`cmd/team/`** — Dev utilities (JWT generator, SportMonks team fetcher).

### Key External Dependencies

- `github.com/tmc/langchaingo` — LLM framework used to call OpenAI `gpt-4o-mini-search-preview`
- `firebase.google.com/go/v4` + `cloud.google.com/go/firestore` — Auth and chat persistence
- `github.com/GoogleCloudPlatform/functions-framework-go` — GCP Functions entry point
- SportMonks REST API — Fixture/team/league data

### Environment Variables

- `OPENAI_API_KEY`
- `SPORTMONKS_API_KEY`
- Firebase credentials:
  - In GCP: provided via Application Default Credentials (default service account) — no `service_account_key.json` needed
  - Locally/dev tools (e.g. `cmd/gentoken`): via `service_account_key.json` or `FIREBASE_AUTH_EMULATOR_HOST`
