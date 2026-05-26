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
