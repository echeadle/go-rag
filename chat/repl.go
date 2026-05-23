package chat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go-rag/llm"
	"os"
	"strings"
	"sync"
	"time"
)

type Options struct {
	SystemPromptFile string
}

func RunREPL(ctx context.Context, client *llm.Client, opts Options) error {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 10 MiB max token limit should be enough for anyone
	history, err := seedHistory(opts.SystemPromptFile)
	if err != nil {
		return fmt.Errorf("loading system prompt: %w", err)
	}

	fmt.Println("Chat session started. Type Q to quit.")

	for {
		fmt.Print("\n> ")
		if !in.Scan() {
			if err := in.Err(); err != nil {
				return err
			}
			return nil // EOF is a clean exit
		}

		input := strings.TrimSpace(in.Text())
		if input == "" {
			continue // ignore empty input
		}

		if strings.EqualFold(input, "q") || strings.EqualFold(input, "/exit") || strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
			fmt.Println("Goodbye.")
			return nil
		}

		history = append(history, llm.Message{Role: "user", Content: input})

		spin := startSpinner("Thinking...")
		var stopOnce sync.Once
		reply, err := client.ChatStream(ctx, history, func(s string) {
			stopOnce.Do(spin.Stop)
			fmt.Print(s)
		})

		stopOnce.Do(spin.Stop)
		fmt.Println()

		if err != nil {
			fmt.Fprintln(os.Stderr, "error: ", err)
			history = history[:len(history)-1]
			continue
		}

		history = append(history, reply)
	}
}

type spinner struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

func startSpinner(label string) *spinner {
	s := &spinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go func() {
		defer close(s.done)
		frames := []string{"|", "/", "-", "\\"}
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				return
			case <-t.C:
				fmt.Printf("\r%s %s", frames[i%len(frames)], label)
				i++
			}
		}
	}()
	return s
}

func (s *spinner) Stop() {
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

func seedHistory(systemPromptFile string) ([]llm.Message, error) {
	if systemPromptFile == "" {
		return nil, nil // no system prompt is fine
	}

	data, err := os.ReadFile(systemPromptFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // missing file is treated as "no system prompt"
	}
	if err != nil {
		return nil, fmt.Errorf("read system prompt: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil // empty file is treated as "no system prompt"
	}

	return []llm.Message{{Role: "system", Content: content}}, nil
}
