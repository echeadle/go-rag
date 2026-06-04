# go-rag

Udemy Course setting up rag using go and postgres

1. Setting up the repo

---

## Roadmap: Basic RAG → Production RAG

Phases listed in priority order. Complete each phase and verify it works before moving to the next.

---

### Phase 1 — Wire Retrieval into the Chat Loop _(most critical)_

- Fix `llm/embed.go` (stub has syntax errors — finish the `Embed()` implementation)
- In `chat/repl.go`, before each LLM call: embed the user query, call `store.Query()`, inject the top-K results as context
- Add a RAG prompt template: *"Answer only from the context below. If the context doesn't contain the answer, say 'I don't know.'"*
- Pass source metadata through so answers can cite their origin

### Phase 2 — Document Ingestion Pipeline

- Build an `ingest` package (or CLI sub-command) that reads files from disk, chunks them, embeds each chunk via `llm.Embed()`, and calls `store.Upsert()`
- Start with plain text; add PDF support later (e.g., `pdfcpu`)
- Example: `go run ./cmd/rag ingest --file ./docs/myfile.txt`

### Phase 3 — Chunking Strategy

- Default to **recursive chunking**: split on paragraph → newline → sentence → word boundaries in that order
- Target chunk size: 500–800 tokens with ~100-token overlap between adjacent chunks
- Store `source`, `chunk_index`, and `char_offset` in each chunk's metadata
- Avoid fixed character splitting — it breaks sentences and degrades retrieval quality

### Phase 4 — Source Attribution

- Format each retrieved chunk to include its source in the prompt (e.g., `[Source: resume.txt, chunk 3]`)
- Surface citations in the final answer so the user knows where information came from

### Phase 5 — Retrieval Quality Filtering

- Add a minimum similarity score threshold (e.g., 0.45 cosine similarity) — discard chunks below it
- Log each retrieved chunk's score and source to stderr for debugging
- If no chunks meet the threshold, have the LLM say "I don't know" rather than hallucinate

### Phase 6 — Observability

- Add structured logging using Go's stdlib `slog`: for every query, log the query text, number of chunks retrieved, top chunk score, and LLM latency
- Track and log token usage per request (prompt + completion tokens)

### Phase 7 — Background File Watcher _(optional)_

- Add a filesystem watcher (`fsnotify`) in `app.Run` to auto-ingest documents when files change
- `app.Run` already has a comment marking this as the intended growth point for a background watcher

### Phase 8 — Web API _(optional)_

- Add an HTTP server alongside the REPL using Go's stdlib `net/http`
- Two endpoints: `POST /ingest` and `POST /query`
- Bind to `127.0.0.1` by default; require an explicit flag to expose externally

### Phase 9 — Production Hardening _(optional)_

- Validate and sanitize all inputs at system boundaries (CLI args, HTTP request bodies)
- Add request timeouts and context cancellation throughout ingest and query paths
- Rate-limit the HTTP API if exposed externally
- Graceful degradation: if the vector store is unavailable, fall back to plain LLM chat with a warning

### Phase 10 — Advanced Features _(later)_

- **Reranking** — after vector retrieval, apply a cross-encoder to reorder chunks by relevance
- **Hybrid search** — combine cosine similarity with keyword search using PostgreSQL `tsvector`
- **Semantic chunking** — use embedding similarity between adjacent sentences to find topic-shift boundaries instead of structural markers
- **Agentic RAG** — let the LLM decide when to retrieve, what to search for, and how many retrieval rounds are needed

---

## Logging

The application uses Go's built-in `log/slog` (structured logging, available since Go 1.21).
All log output goes to **stderr** — it appears in the terminal but is not written to disk by default.

### What the output looks like

Each log line is a timestamp followed by `key=value` pairs:

```
time=2024-01-15T10:23:45.123Z level=INFO  msg="vector store ready"
time=2024-01-15T10:23:45.124Z level=INFO  msg="watching dir for documents" dir=./documents
time=2024-01-15T10:23:45.125Z level=INFO  msg="web chat available" addr=:8080
time=2024-01-15T10:23:46.001Z level=INFO  msg="request" component=web method=GET path=/chat status=200 duration=1.2ms
time=2024-01-15T10:23:50.772Z level=WARN  msg="injection blocked" component=web pattern="(?i)\\bignore..." route=/api/v1/chat/stream
time=2024-01-15T10:24:01.003Z level=ERROR msg="upload ingest failed" component=web file=notes.txt error="unsupported format"
```

The `component` field tells you which part of the app produced the line (`web` or `ingest`).

### Log levels

| Level | Used for |
|---|---|
| `INFO` | Normal events: startup status, HTTP requests |
| `WARN` | Blocked prompt injection attempts — security-relevant, always worth reviewing |
| `ERROR` | Failures: ingest errors, caption errors, template errors |

### Why stderr and not stdout?

The chat REPL and the web server's streaming responses use stdout. Writing logs to stderr keeps
them on a separate stream so log lines never interrupt or corrupt the chat output. On your
terminal the two streams appear mixed together, but they stay independent — a script or tool
that reads stdout won't accidentally receive log lines.

### Saving logs to disk

Standard shell redirection works since logs are just stderr:

```bash
# Overwrite log file each run
go run ./cmd/rag/ 2>app.log

# Append across runs (keeps history)
go run ./cmd/rag/ 2>>app.log

# See logs in the terminal AND save to a file at the same time
go run ./cmd/rag/ 2>&1 | tee app.log
```

### Logs in production

When running as a background service or inside a container, redirect stderr to your log
collector. The structured `key=value` format is directly compatible with Grafana Loki,
Datadog, and CloudWatch — each field is individually searchable and filterable.

```bash
# Docker example: merge stderr into stdout for the container log driver to capture
docker run ... 2>&1
```

---

## Authentication

All `/api/v1/*` routes (chat, upload, image upload, caption) are protected by a static
Bearer token. The `/chat` page, `/healthz`, and `/images/*` are intentionally open —
image tags in the browser cannot send Authorization headers.

### How it works

Every API request from the browser includes an `Authorization: Bearer <key>` header.
The first time you open the chat page, a dialog prompts for the key. The key is stored
in `localStorage` so you only enter it once per browser. A lock icon button (🔒) in the
chat toolbar lets you update it at any time.

If the key is wrong or missing the server returns `401 Unauthorized` and the dialog
re-appears.

### Configuration

Set `API_KEY` in your `.env` file:

```
API_KEY=your-secret-key-here
```

When `API_KEY` is not set the server starts in **open mode** — all API routes are
accessible without a key. This is intentional for local development. A warning is
logged at startup:

```
level=WARN msg="API_KEY not set — authentication disabled; all /api/v1/* routes are open"
```

### Generating a good key

An API key is a long random secret — the security comes entirely from it being
unguessable. Use a cryptographically secure random number generator, not a password
or a word.

**Recommended — 32 bytes of randomness, base64-encoded (same strength as Claude API keys):**

```bash
openssl rand -base64 32
```

Example output: `K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=`

**Alternative — hex encoding (no special characters):**

```bash
openssl rand -hex 32
```

Example output: `a3f2c1d4e5b6a7f8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2`

Both produce 256 bits of entropy — more than enough. Copy the output directly into
your `.env`. Do not reuse passwords, phrases, or anything shorter than 32 characters.

### Why this is different from an SSH key

An **SSH key** is an asymmetric key *pair* — a private key you keep and a public key
you give to the server (e.g. GitHub). You never send the private key anywhere; you use
it to sign a challenge and the server verifies the signature. This works when you
cannot safely share a secret with the server ahead of time.

An **API key** is a symmetric secret — both you and the server know the same value.
You include it in every request; the server compares it to what it stored. Simpler,
and the right choice for a single-user personal tool where you control both sides.

---

## API Reference

All API endpoints are versioned under `/api/v1/`. Requests to protected endpoints must
include an `Authorization: Bearer <key>` header.

### Open endpoints (no auth required)

| Method | Path | Description |
|---|---|---|
| `GET` | `/chat` | Serves the chat UI (HTML page) |
| `GET` | `/healthz` | Health check — pings the DB; returns `200 ok` or `503 db unavailable` |
| `GET` | `/metrics` | Prometheus metrics endpoint |
| `GET` | `/images/*` | Serves uploaded images by filename |

### Protected endpoints (`Authorization: Bearer <key>` required)

#### `POST /api/v1/chat/stream`

Stream a chat completion over Server-Sent Events (SSE).

**Request body** (`application/json`):
```json
{
  "messages": [
    { "role": "user", "content": "What does the document say about X?" }
  ]
}
```
Full conversation history is sent on each call — the server is stateless.

**Response** (`text/event-stream`):
```
event: delta
data: "Hello"

event: delta
data: " world"

event: done
data: ""
```
On error: `event: error` with the error message as a quoted JSON string.

---

#### `POST /api/v1/upload`

Ingest a text document (`.txt`, `.md`, `.markdown`, `.pdf`) into the vector store.

**Request body** (`multipart/form-data`):
- `file` — the document file (max 10 MB)

**Response** (`application/json`):
```json
{ "source": "notes.txt", "bytes": 4096, "chunks": 12 }
```

---

#### `POST /api/v1/upload/image`

Upload an image and store it with a text description for retrieval.

**Request body** (`multipart/form-data`):
- `image` — image file (`.png`, `.jpg`, `.jpeg`, `.webp`, `.gif`; max 10 MB)
- `description` — text description to embed and index (required)

**Response** (`application/json`):
```json
{
  "source": "1718123456789-photo.jpg",
  "image_path": "/images/1718123456789-photo.jpg",
  "description": "A diagram showing the architecture",
  "bytes": 204800,
  "chunks": 1
}
```

---

#### `POST /api/v1/caption`

Generate a text description for an image using the vision model.
Only available when the configured model supports vision.

**Request body** (`multipart/form-data`):
- `image` — image file (`.png`, `.jpg`, `.jpeg`, `.webp`, `.gif`; max 10 MB)

**Response** (`application/json`):
```json
{ "description": "A screenshot of a terminal showing Go test output." }
```

---

### Error responses

All endpoints return standard HTTP status codes:

| Code | Meaning |
|---|---|
| `400` | Bad request — malformed body, missing required field, or empty file |
| `401` | Unauthorized — missing or invalid Bearer token |
| `415` | Unsupported Media Type — file format not accepted |
| `429` | Too Many Requests — per-IP rate limit exceeded (default: 100 req/min) |
| `502` | Bad Gateway — LLM or caption service error |
| `503` | Service Unavailable — vector store or DB not reachable |
