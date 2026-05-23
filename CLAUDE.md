# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

This is a Udemy course project building a RAG (Retrieval-Augmented Generation) system in Go with PostgreSQL. Code is added incrementally as lessons progress — the `zipfiles/rag-course/` directory holds the course's reference implementation.

## Commands

```bash
# Run the application
go run ./cmd/rag/

# Build
go build ./cmd/rag/

# Run all tests
go test ./...

# Run tests for a single package
go test ./chat/

# Tidy dependencies
go mod tidy
```

## Configuration

All config is loaded from `.env` (via `godotenv`) with environment variable fallbacks. Required variables:

```
OPENAI_BASE_URL=      # defaults to https://api.openai.com/v1 — override for Ollama, LM Studio, etc.
OPENAI_API_KEY=       # omit for local providers that don't require auth
OPENAI_MODEL=         # defaults to gpt-4o-mini
SYSTEM_PROMPT_FILE=   # optional path to a .md file loaded as the system message
```

The client is provider-agnostic: any OpenAI-compatible endpoint works by changing `OPENAI_BASE_URL`.

## Architecture

```
cmd/rag/main.go   → entry point; sets up signal context, calls app.Run
config/config.go  → loads .env + env vars into a Config struct
app/app.go        → wiring: constructs llm.Client, starts chat.RunREPL
llm/client.go     → HTTP wrapper around the OpenAI-compatible API (chat + embeddings)
chat/repl.go      → terminal read-eval loop; owns conversation history
```

**Key design points:**

- `app.Run` is the intended growth point for future lessons — vector store init, background ingest watcher, and optional web server will all be added here.
- The chat-completions API is stateless; conversation memory is maintained by replaying the full `[]llm.Message` history on every call (see `chat.RunREPL`).
- `llm.Message` is a project-local type (not a re-export of the SDK type) so `chat`, `web`, and future `rag` packages stay decoupled from the SDK.
- `llm.Client.ChatStream` streams token deltas via an `onDelta` callback; the spinner in `chat.RunREPL` stops on the first delta so streamed output appears immediately.
