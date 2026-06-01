package ingest

import (
	"context"
	"errors"
	"fmt"
	"go-rag/llm"
	"go-rag/vector"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 500 * time.Millisecond

func Watch(ctx context.Context, opts Options, embedder llm.Embedder, store vector.Store, logger *slog.Logger) error {
	if filepath.Clean(opts.SourceDir) == filepath.Clean(opts.ProcessedDir) {
		return errors.New("source and processed directories must be different")
	}
	if err := os.MkdirAll(opts.SourceDir, 0o755); err != nil {
		return fmt.Errorf("create source dir: %w", err)
	}
	if err := os.MkdirAll(opts.ProcessedDir , 0o755); err != nil {
		return fmt.Errorf("create processed dir: %w", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	defer w.Close()

	if err := w.Add(opts.SourceDir); err != nil {
		return fmt.Errorf("watch source dir: %w", err)
	}

	processedAbs, err := filepath.Abs((opts.ProcessedDir))
	if err != nil {
		return fmt.Errorf("resolve processed dir: %w", err)
	}

	handle := func(path string) {
		base := filepath.Base(path)
		if err := processOne(ctx, path, opts, embedder, store); err != nil {
			logger.Error("process file", slog.String("file", base), slog.Any("error", err))
			return
		}
		dst := filepath.Join(opts.ProcessedDir, base)
		if err := os.Rename(path, dst); err != nil {
			logger.Error("move to processed", slog.String("file", base), slog.Any("error", err))
			return
		}
		logger.Info("ingested", slog.String("file", base))
	}

	entries, err := os.ReadDir(opts.SourceDir)
	if err != nil {
		return fmt.Errorf("read source dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		go handle(filepath.Join(opts.SourceDir, e.Name()))
	}

	var (
		timersMu sync.Mutex
		timers = make(map[string]*time.Timer)
	)

	schedule := func(path string) {
		timersMu.Lock()
		defer timersMu.Unlock()
		if t, ok := timers[path]; ok {
			t.Reset(debounceDelay)
			return
		}

		timers[path] = time.AfterFunc(debounceDelay, func() {
			timersMu.Lock()
			delete(timers, path)
			timersMu.Unlock()
			handle(path)
		})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if !shouldProcess(ev.Name, processedAbs) {
				continue
			}
			schedule(ev.Name)
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			logger.Error("watcher error", slog.Any("error", err))
		}
	}
}

func shouldProcess(path, processedAbs string) bool {
	if strings.HasPrefix(filepath.Base(path), ".") {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if processedAbs != "" && strings.HasPrefix(abs, processedAbs+string(filepath.Separator)) {
		return false
	}
	return true
}
