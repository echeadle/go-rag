# go-rag: Path to Production

## Context

This project was built lesson-by-lesson as a Udemy course on RAG with Go and PostgreSQL.
The course is now complete. The code is clean, compiles, and demonstrates all the core RAG
concepts: document ingestion, vector embeddings, similarity retrieval, context injection,
streaming chat, vision/image support, and prompt injection defense.

The goal of this document is to identify what would need to change — and in what order —
to turn this learning project into something you'd be comfortable deploying and leaving
running in the real world.

---

## What's Already Production-Ready

Worth noting before the gap list — several things were done right from the start:

- **Provider-agnostic LLM client** — swap models by changing env vars, no code changes
- **Graceful shutdown** — signal context, `wg.Wait()`, `srv.Shutdown()`
- **Prompt injection defense** — `InjectionDefense` middleware with regex pattern scanning
- **Config via env vars** — no hardcoded secrets, `.env` for local dev only
- **Streaming SSE** — proper `text/event-stream` with flush on every delta
- **Image safety** — `safeFileName`, `MaxBytesReader`, MIME validation
- **HTTP response header timeout** — bumped to 30 min for local Ollama models

---

## Gap Analysis: What Production Needs

### 1. Tests — Highest Priority

**Current state:** Zero tests.

**What's needed:**
- Unit tests for `rag/prompt.go` (formatContext, withInlineContext)
- Unit tests for `web/injection.go` (scanForInjection with known-bad and known-good inputs)
- Unit tests for `ingest/` (chunking, IsSupported, IsImage, ProcessContent)
- Integration test for the full RAG pipeline (ingest → embed → retrieve → format)
- HTTP handler tests using `net/http/httptest` for the web routes

**Why it matters:** The injection defense in particular needs regression tests — those
regex patterns are subtle and easy to break when editing.

**Go testing tools to use:** standard `testing` package, `net/http/httptest`, and
`github.com/stretchr/testify` for assertions.

---

### 2. Structured Logging

**Current state:** Plain `log.Printf` calls scattered across packages.

**What's needed:** Replace with Go's built-in `log/slog` (available since Go 1.21).
Structured logs (key=value pairs) make it possible to search and filter in tools like
Grafana Loki, Datadog, or CloudWatch.

**What changes:**
- Pass a `*slog.Logger` through `app.Run` instead of constructing a bare `log.New`
- Replace `log.Printf("[web] ...")` calls with `logger.Error(...)` / `logger.Info(...)`
  with structured attributes: `slog.String("file", name)`, `slog.Int("chunks", n)`, etc.
- Add request logging middleware (chi's `middleware.Logger` or a custom slog handler)

---

### 3. Authentication & Authorization

**Current state:** All API endpoints are open — anyone who can reach the server can
ingest documents, upload images, and query the vector store.

**What's needed (minimum viable):**
- A static API key in the `Authorization: Bearer <token>` header, checked by middleware
- The key loaded from env (`API_KEY`), not hardcoded
- Applied to all `/api/*` routes

**What's needed (full multi-user):**
- User accounts with JWT tokens
- Per-user vector store namespacing (a `user_id` column on the embeddings table)
- Sign-up / sign-in endpoints

For a personal tool, the static API key approach is usually enough.

---

### 4. Rate Limiting

**Current state:** No rate limiting. A bad actor (or a runaway script) could hammer
the `/api/caption` endpoint and rack up Ollama GPU time or OpenAI API costs.

**What's needed:**
- Per-IP or per-token rate limiting on API endpoints
- `golang.org/x/time/rate` (standard library extension) for a simple token bucket
- Or `github.com/go-chi/httprate` which plugs straight into chi

---

### 5. Configuration Validation

**Current state:** `config.Load()` returns whatever is in the env — including empty
strings for required fields like `DATABASE_URL` — and errors only appear at runtime
when something actually tries to use the missing value.

**What's needed:**
- Validate required fields at startup, before any goroutines launch
- Print a clear error and exit if `DATABASE_URL` is set but unreachable
- Validate `EMBEDDING_DIM` is a sensible value (> 0, matches what the model actually produces)

---

### 6. Dockerfile & Deployment

**Current state:** No container, no deployment config.

**What's needed (minimum):**
- Multi-stage `Dockerfile`: `golang:1.26-alpine` to build, `alpine` to run
- `.dockerignore` to exclude `zipfiles/`, `.env`, `documents/`
- `docker-compose.yml` that spins up the app + PostgreSQL + pgvector together
- Health check endpoint (`GET /healthz`) that returns 200 if the DB is reachable

**Nice to have:**
- GitHub Actions CI: `go build`, `go vet`, `go test ./...` on every push
- Automatic image build and push to a registry on merge to main

---

### 7. Database Migrations

**Current state:** `pgvector.New()` runs a `CREATE TABLE IF NOT EXISTS` on startup —
fine for learning, brittle for production because you can't evolve the schema safely.

**What's needed:**
- A migration tool: `golang-migrate/migrate` or `pressly/goose` are both Go-native
- Migrations live in `db/migrations/` as numbered SQL files
- The app applies pending migrations at startup (or a separate `migrate up` command)

---

### 8. Observability / Metrics

**Current state:** No metrics, no tracing.

**What's needed:**
- A `/metrics` endpoint exposing Prometheus counters:
  - `rag_requests_total` (by route, status)
  - `rag_ingest_chunks_total`
  - `rag_retrieval_latency_seconds`
  - `rag_llm_latency_seconds` (by model)
- `github.com/prometheus/client_golang` is the standard Go client

**Nice to have:** OpenTelemetry traces through the ingest → embed → retrieve → generate
pipeline, so you can see exactly where latency is coming from.

---

### 9. API Versioning & Documentation

**Current state:** Routes are `/api/chat/stream`, `/api/upload`, etc. — no version prefix.

**What's needed:**
- Version prefix: `/api/v1/chat/stream` — makes it safe to change the contract later
- OpenAPI spec (can be hand-written or generated with `swaggo/swag`)
- At minimum, a `README.md` that documents every endpoint, its request shape, and
  its response shape

---

### 10. Frontend Hardening

**Current state:** The `chat.gohtml` template is functional but minimal.

**What's needed:**
- CSRF protection (`github.com/justinas/nosurf` plugs into chi)
- Content Security Policy header
- File type validation on the client side before upload (reduces bad requests)
- Error display in the UI when the server returns 4xx/5xx

---

## Recommended Order of Work

| Priority | Item | Why first |
|---|---|---|
| 1 | Tests | Everything else is safer to change once tests exist |
| 2 | Structured logging | Quick win; makes debugging everything else easier |
| 3 | Config validation | Catches setup errors before confusing runtime failures |
| 4 | Dockerfile + docker-compose | Repeatable full-stack environment |
| 5 | Authentication | Before exposing to any network beyond localhost |
| 6 | Rate limiting | Before any real traffic |
| 7 | Database migrations | Before schema needs to change |
| 8 | Metrics | Once running, know what it's doing |
| 9 | API versioning + docs | Before anyone else integrates with it |
| 10 | Frontend hardening | Backend must be solid first |

---

## What This Is Already Good Enough For

Even without the above, the current code is solid for:
- A personal knowledge-base chatbot running on your own machine
- Continued learning and experimentation
- A portfolio piece that demonstrates real RAG architecture in Go

The production gap is mostly about **operability** (logs, metrics, deploy) and
**safety** (auth, rate limits, tests) — not about the core RAG logic, which is
well-structured and clean.
