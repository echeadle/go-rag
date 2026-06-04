# Handoff Prompt — go-rag

Paste the block below at the start of a new session to resume exactly where we left off.

---

We are productionizing the go-rag project using PRD.md as the guide.

**Completed (on main, pushed):**
- PRD item 1: 73 tests across ingest, rag, and web packages (63b4936)
- PRD item 2: structured logging with log/slog — TextHandler to stderr, component attributes, request-logger middleware (cc7ad63)
- PRD item 3: config validation — `config.Validate()` catches bad EMBEDDING_DIM; fatal on bad DATABASE_URL (16f17e6)
- PRD item 4: production-grade Dockerfile — multi-stage build, distroless runtime, non-root, read_only + cap_drop ALL, named bridge network (da10e9c)
- PRD item 5: Bearer token auth on all /api/v1/* routes; requireAuth middleware; key dialog in browser (7d4979c)
- PRD item 6: per-IP rate limiting on /api/v1/* via go-chi/httprate; RATE_LIMIT_REQUESTS env var (6d8260c)
- PRD item 7: goose DB migrations; migration 001 creates embeddings table with dynamic EMBEDDING_DIM (e5cdbd8)
- PRD item 8: Prometheus metrics at GET /metrics; 4 metrics across web/ingest/rag/llm; dev docker-compose (c66f199)
- PRD item 9: API versioning — all routes now /api/v1/*; full API Reference in README (02c8eba)
- PRD item 10: frontend hardening — CSP nonce, client-side file validation, alert() → renderNotice(); CSRF skip documented
- PRD item 11: PDF ingestion via pdfcpu (7f9693a)
- PRD item 12: semantic chunking — splitSentences + cosineSim + chunkSemantic; CHUNK_SEMANTIC_THRESHOLD env var (default 0.75); wired through config → watcher + web upload (1783d03)

**Next: PRD item 13 — Hybrid search**

Combine cosine-similarity vector retrieval with PostgreSQL `tsvector` keyword search (full-text search). The goal is to return results even when the query embedding doesn't land near the right chunk. Steps are defined in PRD.md.

**Working conventions:**
- Create a feature branch first: `git checkout -b feature/hybrid-search`
- Read PRD.md at session start to confirm the exact steps
- Use the advisor before committing to approach on multi-file changes
- Merge to main only after running the app and confirming it works
