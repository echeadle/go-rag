package app

// Package app wires the pieces of lesson 1 together: the LLM client
// and the chat REPL. Keeping orchestration here means cmd/rag/main.go
// stays a thin shim that's easy to read.
//
// The shape of the program in lesson 1, viewed from this file:
//
//	llm.Client      talks HTTP to the chat-completions endpoint
//	chat.RunREPL    foreground: terminal chat loop
//
// Later lessons will grow this file: a vector store will be opened
// here, a background ingest watcher will be spawned, an optional web
// server will start alongside the REPL. The wiring point is always
// the same — Run takes a Config, builds the components, and starts
// the foreground loop.
import (
	"context"
	"fmt"
	"go-rag/chat"
	"go-rag/config"
	"go-rag/ingest"
	"go-rag/llm"
	"go-rag/rag"
	"go-rag/vector"
	"go-rag/vector/pgvector"
	"go-rag/web"
	"log/slog"
	"os"
	"sync"

	"golang.org/x/term"
)

// Run is the program's main loop. In lesson 1 there is only the
// foreground REPL, so Run constructs the LLM client and hands it
// straight to chat.RunREPL.
func Run(parent context.Context, cfg config.Config) error {
	// TextHandler to stderr keeps status lines off the stdout chat stream.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	client := llm.New(cfg)

	embedder := llm.NewEmbedder(cfg)

	// Open the vector store. A nil store with a nil error means
	// "no DATABASE_URL configured" — chat-only mode, intentional.
	// Any real error (bad DSN, server unreachable, migration failure)
	// is fatal: if DATABASE_URL is set, a broken store is not recoverable.
	store, err := openStore(ctx, cfg)
	if err != nil {
		return fmt.Errorf("vector store: %w", err)
	}

	ingestLogger := logger.With(slog.String("component", "ingest"))
	var wg sync.WaitGroup
	if store != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := ingest.Options{
				SourceDir:         cfg.IngestDir,
				ProcessedDir:      cfg.ProcessedDir,
				SemanticThreshold: cfg.ChunkSemanticThreshold,
			}
			if err := ingest.Watch(ctx, opts, embedder, store, ingestLogger); err != nil && ctx.Err() == nil {
				logger.Error("watcher stopped", slog.Any("error", err))
			}
		}()
		logger.Info("watching dir for documents", slog.String("dir", cfg.IngestDir))
	}

	// Defer Close so the connection pool drains
	// cleanly on exit (Ctrl-C, REPL quit, or any error). Guarded by
	// the nil-check because openStore returns a nil interface when
	// DATABASE_URL is unset.
	if store != nil {
		defer store.Close()
		logger.Info("vector store ready")
	}

	//get a retriever, which means we need a rewriter
	var retriever *rag.Retriever
	if store != nil {
		retriever = rag.New(embedder, store, rag.Options{
			TopK:     5,
			Rewriter: rag.NewRewriter(client),
		})
	}

	if cfg.HTTPAddr != "" {
		srv, err := web.New(client, embedder, retriever, web.Options{
			Addr:                   cfg.HTTPAddr,
			SystemPromptFile:       cfg.SystemPromptFile,
			Store:                  store,
			ProcessedDir:           cfg.ProcessedDir,
			ImagesDir:              cfg.ImageDir,
			Logger:                 logger.With(slog.String("component", "web")),
			ServerAPIKey:           cfg.ServerAPIKey,
			RateLimitRequests:      cfg.RateLimitRequests,
			ChunkSemanticThreshold: cfg.ChunkSemanticThreshold,
		})
		if err != nil {
			logger.Error("web server disabled", slog.Any("error", err))
		} else {
			wg.Go(func() {
				if err := srv.Run(ctx, cfg.HTTPAddr); err != nil && ctx.Err() == nil {
					logger.Error("web server stopped", slog.Any("error", err))
				}
			})
			logger.Info("web chat available", slog.String("addr", cfg.HTTPAddr))
		}
	}

	var replErr error
	if isTerminal() {
		replErr = chat.RunREPL(ctx, client, retriever, chat.Options{
			SystemPromptFile: cfg.SystemPromptFile,
		})
	} else {
		// No TTY (container or piped stdin): run as a pure server.
		// Block until the signal context is cancelled (Ctrl-C / SIGTERM).
		logger.Info("no TTY detected — running in server-only mode")
		<-ctx.Done()
	}

	cancel()
	wg.Wait()
	return replErr
}

// isTerminal reports whether os.Stdin is an interactive terminal.
// When it is not (container, piped input, CI), the REPL is skipped
// and the process runs as a pure HTTP server.
// os.Stdin.Stat() is not sufficient — /dev/null is also a character device.
// term.IsTerminal uses the TIOCGETA ioctl to ask the kernel directly.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func openStore(ctx context.Context, cfg config.Config) (vector.Store, error) {
	if cfg.DatabaseURL == "" {
		return nil, nil
	}
	s, err := pgvector.New(ctx, pgvector.Options{
		DSN:          cfg.DatabaseURL,
		EmbeddingDim: cfg.EmbeddingDim,
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}
