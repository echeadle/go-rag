# Handoff Prompt — go-rag

Paste the block below at the start of a new session to resume exactly where we left off.

---

We are productionizing the go-rag project using PRD.md as the guide.

**Completed (on main, pushed):**
- PRD item 1: 73 tests across ingest, rag, and web packages (commit 63b4936)
- PRD item 2: structured logging with log/slog — TextHandler to stderr, threaded explicitly, component attributes, request-logger middleware (commit cc7ad63)
- PRD item 3: config validation — `config.Validate()` catches bad EMBEDDING_DIM; DATABASE_URL set + unreachable is now fatal (commit 16f17e6)
- PRD items 11–14 added to PRD.md: PDF ingestion, semantic chunking, hybrid search, reranking (commit 37c7bce)

**Next: PRD item 4 — Dockerfile + docker-compose (production-grade, step by step)**

The goal is NOT a quick spin-up. We are building toward a fully hardened production
container, one verified step at a time. Ed wants to understand each layer — don't jump
ahead. The steps are defined in PRD.md section 6:

- Step 1: Working multi-stage build (golang:1.26-alpine → alpine runtime), .dockerignore
- Step 2: /healthz endpoint + docker-compose (app + pgvector/pgvector:pg16)
- Step 3: Non-root user inside the container
- Step 4: Distroless runtime image (no shell, minimal attack surface)
- Step 5: Hardened runtime flags (read_only, no-new-privileges, cap_drop ALL)
- Step 6: Network isolation (named bridge network, DB not exposed to host)
- Step 7: Image vulnerability scan (docker scout or trivy)

Complete and verify each step before moving to the next. Explain what each security
measure does and why it matters — this is a learning exercise as much as a build task.

**Working conventions:**
- Create a feature branch first: `git checkout -b feature/dockerfile`
- Merge to main only after running the app and confirming it works
- Read PRD.md section 6 at session start before writing any files
- Use the advisor before committing to an approach on multi-file changes
