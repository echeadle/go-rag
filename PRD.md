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

**Goal:** Build toward a production-grade container step by step — not just a quick
spin-up. Each step is verified before moving to the next. Security is not an afterthought
added at the end; it is layered in progressively so each decision is understood.

**Step 1 — Working multi-stage build:**
- Build stage: `golang:1.26-alpine` — compiles the binary with CGO disabled (`CGO_ENABLED=0`)
  so the output is a fully static binary with no libc dependency
- Runtime stage: `alpine:3.21` — small but has a shell and package manager for debugging
- `.dockerignore` to keep secrets and large dirs out of the build context:
  `zipfiles/`, `.env`, `documents/`, `.git/`
- Verify: `docker build` succeeds and `docker run` starts the app

**Step 2 — Health check endpoint + docker-compose:**
- Add `GET /healthz` route that pings the DB and returns 200/503
- `docker-compose.yml` wiring app + `pgvector/pgvector:pg16` together
- Compose passes env vars via an `.env` file (gitignored); no secrets in the image
- Verify: `docker compose up` starts both services; `/healthz` returns 200

**Step 3 — Non-root user (first security layer):**
- Add a dedicated `appuser` in the Dockerfile (`adduser -D -u 1001 appuser`)
- Switch to that user before `CMD` — process no longer runs as root inside the container
- Verify: `docker exec` confirms `whoami` returns `appuser`, not `root`

**Step 4 — Minimal attack surface:**
- Switch runtime base from `alpine` to `gcr.io/distroless/static-debian12` — no shell,
  no package manager, no utilities an attacker could use
- Only the compiled binary and the embedded templates exist in the image
- Verify: `docker run --entrypoint sh` fails (no shell — that's the point)

**Step 5 — Harden the runtime flags:**
- In docker-compose, add per-service security options:
  - `read_only: true` — filesystem is read-only; app can only write to explicitly mounted volumes
  - `security_opt: [no-new-privileges:true]` — process cannot gain elevated privileges via setuid
  - `cap_drop: [ALL]` — drop all Linux capabilities; add back only what's needed (likely none)
- Verify: app starts and operates normally with all restrictions in place

**Step 6 — Network isolation:**
- Define an explicit bridge network in docker-compose; do not use the default network
- The app container can reach the database; the database is not exposed to the host
- The app's HTTP port is the only thing published to the host
- Verify: `docker network ls` shows a named network; DB port is not reachable from host

**Step 7 — Image vulnerability scan:**
- Run `docker scout cves` (or `trivy image`) against the built image
- Fix any HIGH/CRITICAL CVEs by updating base image versions or dependencies
- Document the scan result so there is a baseline to compare future builds against

**Nice to have (after the above):**
- GitHub Actions CI: `go build`, `go vet`, `go test ./...` + Trivy scan on every push
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

### 11. PDF Ingestion

**Current state:** The ingest pipeline supports `.txt` and `.md`/`.markdown` only. PDF is
the most common format for real-world knowledge bases — manuals, papers, reports, etc.

**What's needed:**
- A pure-Go PDF text extraction library — `github.com/ledongthuc/pdf` requires no external
  tools and handles most text-based PDFs
- Update `IsSupported()` in `ingest/ingest.go` to include `.pdf`
- Add a `extractPDF(data []byte) (string, error)` function and route `.pdf` files through
  it before chunking in `ProcessContent`
- Tests: known-good PDF → extracted text contains expected strings; corrupted/empty PDF → error

**Note:** Image-only (scanned) PDFs produce no extractable text. Those documents belong in
the image pipeline — upload as an image, caption it with the vision model, then ingest the
caption. The extractor should return a clear error (not empty string) for image-only PDFs
so the caller can surface a useful message.

---

### 12. Semantic Chunking

**Current state:** Documents are split at fixed character boundaries (`ChunkSize`, `Overlap`
in `ingest.Options`). This can cut sentences or paragraphs mid-thought, which degrades
retrieval quality when the meaningful unit spans a chunk boundary.

**What's needed:**
- Replace fixed-size splitting with a topic-boundary detector: embed adjacent sentences,
  compute cosine similarity between consecutive pairs — a sharp similarity drop signals a
  topic shift, chunk there
- Fall back to the current fixed chunker when the document is too short, has no sentence
  boundaries, or the embedder is unavailable
- Tests: document with clear topic shifts produces chunks that don't split mid-topic;
  short document → single chunk; embedder error → falls back to fixed chunking

**Key tradeoff:** Semantic chunking makes ingest slower and more expensive — it requires
one embedding call per sentence rather than none. The payoff is chunks that align with
topic boundaries, which significantly improves how well retrieval finds relevant content.
Implement PDF ingestion first; chunking quality matters more once you have richer documents.

---

### 13. Hybrid Search

**Current state:** Retrieval uses pure vector similarity (cosine distance via pgvector).
This works well for semantically-phrased queries but can miss results for exact terms —
names, codes, specific product identifiers, abbreviations.

**What's needed:**
- Add a `tsvector` column to the embeddings table (requires a DB migration — item 7 first)
- At query time, run both a pgvector similarity search and a PostgreSQL `ts_query` keyword
  search, then merge the two result sets using Reciprocal Rank Fusion (RRF) — a simple
  algorithm that combines ranked lists without needing to tune score weights
- `vector/pgvector` needs a `HybridQuery` method alongside the existing `Query`
- Tests: query with a proper name that doesn't embed similarly → found by keyword path;
  semantic query → found by vector path; both paths return the same document → deduplicated

**Key tradeoff:** Adds schema complexity (migration, new column) and query complexity, but
meaningfully improves recall for mixed natural-language + exact-term queries. Do semantic
chunking first — better chunks raise the quality ceiling that hybrid search then exploits.

---

### 14. Reranking

**Current state:** Retrieved chunks are returned in order of vector similarity score (or
RRF score, once hybrid search is in place). That score is a decent but imperfect proxy
for relevance to the user's exact question.

**What's needed:**
- After initial retrieval (`TopK` results), run a cross-encoder reranker model that scores
  each (query, chunk) pair together — far more accurate than embedding similarity alone
- Re-sort chunks by reranker score; pass only the top N (typically 3–5) to the LLM
- Model options: `cross-encoder/ms-marco-MiniLM-L-6-v2` via a local Ollama endpoint, or a
  hosted reranking API (Cohere Rerank, Jina Reranker)
- Tests: reranker re-orders a known mis-ranked retrieval result into the correct order;
  reranker unavailable → falls back to original retrieval order with a logged warning

**Key tradeoff:** Adds latency (one model call per retrieved chunk) and potential cost if
using a hosted API. This is the final layer of retrieval quality polish — do semantic
chunking and hybrid search first. With good chunks and good recall, reranking is the
difference between "usually right" and "consistently right."

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
| 11 | PDF ingestion | Expands what the knowledge base can ingest |
| 12 | Semantic chunking | Biggest single improvement to retrieval quality |
| 13 | Hybrid search | Improves recall for exact-term queries; needs item 7 first |
| 14 | Reranking | Final retrieval polish; most impactful once 12 and 13 are done |

---

## What This Is Already Good Enough For

Even without the above, the current code is solid for:
- A personal knowledge-base chatbot running on your own machine
- Continued learning and experimentation
- A portfolio piece that demonstrates real RAG architecture in Go

The production gap is mostly about **operability** (logs, metrics, deploy) and
**safety** (auth, rate limits, tests) — not about the core RAG logic, which is
well-structured and clean.
