package main

import (
	"context"
	"fmt"
	"go-rag/app"
	"go-rag/config"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// We need to:
	// - set up app
	// - set up config
	// - set up the LLM client
	// - set up the Read-Eval Loop (REPL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, config.Load()); err != nil {
		fmt.Fprintln(os.Stderr,err)
		os.Exit(1)
	}
}