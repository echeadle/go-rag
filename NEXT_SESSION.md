# Handoff Prompt — go-rag

Paste the block below at the start of a new session to resume exactly where we left off.

---

We are productionizing the go-rag project using PRD.md as the guide.

**Completed (on main, pushed):**
- PRD item 1: 73 tests across ingest, rag, and web packages (commit 63b4936)
- PRD item 2: structured logging with log/slog — TextHandler to stderr, threaded explicitly, component attributes, request-logger middleware (commit cc7ad63)
- PRD item 3: config validation — `config.Validate()` catches bad EMBEDDING_DIM; DATABASE_URL set + unreachable is now fatal (commit 16f17e6)
- PRD items 11–14 added to PRD.md: PDF ingestion, semantic chunking, hybrid search, reranking (commit 37c7bce)

**Next: PRD item 4 — Dockerfile + docker-compose**

From PRD.md, what's needed:
- Multi-stage `Dockerfile`: `golang:1.26-alpine` to build, `alpine` to run
- `.dockerignore` to exclude `zipfiles/`, `.env`, `documents/`
- `docker-compose.yml` that spins up the app + PostgreSQL + pgvector together
- Health check endpoint (`GET /healthz`) that returns 200 if the DB is reachable

**Working conventions:**
- Always create a feature branch before starting: `git checkout -b feature/dockerfile`
- Merge to main only after running the app and confirming it works
- Read PRD.md at session start to confirm position before writing any code
- Use the advisor before committing to an approach on multi-file changes

Start by reading PRD.md to confirm position, then create the feature branch and plan the implementation before touching any files.
