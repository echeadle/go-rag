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
	"go-rag/chat"
	"go-rag/config"
	"go-rag/llm"
	"go-rag/vector"
	"go-rag/vector/pgvector"
	"log"
	"os"
)

// Run is the program's main loop. In lesson 1 there is only the
// foreground REPL, so Run constructs the LLM client and hands it
// straight to chat.RunREPL.
func Run(ctx context.Context, cfg config.Config) error {
	logger := log.New(os.Stderr, "[rag] ", log.LstdFlags)

	client := llm.New(cfg)

	store, err := openStore(ctx, cfg)
	if err != nil {
		logger.Printf("vector store disabled: %v", err)
	}

	if store != nil {
		defer store.Close()
		logger.PrintF("vector store ready")
	}

	return chat.RunREPL(ctx, client, chat.Options{
		SystemPromptFile: cfg.SystemPromptFile,
	})
}

func openStore(ctx context.Context,cfg config.Config) (vector.Store, error) {
	if cfg.DatabaseURL == "" {
		return nil, nil
	}
	s, err := pgvector.New(ctx, pgvector.Options {
		DSN: cfg.DatabaseURL,
		EmbeddingDim: cfg.EmbeddingDim,
	})
	if err != nil {
		return nil, err
	}
}