// Package llm wraps the subset of the OpenAI-compatible chat-completions
// and embeddings APIs that this project needs, built on top of
// github.com/openai/openai-go/v3 — the official, generated SDK.
//
// Why one package, two APIs? Because almost every modern LLM provider
// (OpenAI, Ollama, LM Studio, Groq, Together, Anyscale, vLLM, ...)
// implements the same two endpoints:
//
//	chat completions   text in, text out (the model's reply)
//	embeddings         text in, dense vector out (numeric coords)
//
// Swap models or providers by changing OPENAI_BASE_URL and the model
// identifiers; no code changes required. The SDK honors any base URL
// that speaks the OpenAI wire format.
//
// Important conceptual point: the chat-completions API is STATELESS.
// The server forgets you between requests. The "conversation" only
// exists because the caller replays the full message history on every
// turn. Look at chat.RunREPL to see how that history is maintained.
package llm

import (
	"context"
	"go-rag/config"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Message is one entry in a chat conversation. Role is typically
// "system", "user", or "assistant"; Content is the text payload. We
// keep our own Message type (rather than re-exporting the SDK's
// ChatCompletionMessageParamUnion) so the rest of the codebase —
// internal/chat, internal/web, internal/rag — stays decoupled from any
// one SDK. Translation to the SDK's union type happens inside this
// package, in toSDKMessages.
//
// The three roles, in plain English:
//
//	system     hidden instruction prepended once at the start of a
//	           conversation — defines persona, scope, formatting
//	           rules. Lives in prompts/system.md in this project.
//	user       the human-typed message you want answered.
//	assistant  the model's reply. We append every reply back into
//	           history so the next turn has context to refer to.
//
// (Some APIs add a "tool" role for function-calling responses; we
// don't use it here.)
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client is a thin wrapper over the openai-go SDK client targeting one
// OpenAI-compatible endpoint. Construct with New; reuse across calls so
// the underlying HTTP client can pool connections.
type Client struct {
	cfg config.Config
	sdk openai.Client
}

func New(cfg config.Config) *Client {
	return newClient(cfg, cfg.BaseURL, cfg.APIKey)
}

func NewEmbedder(cfg config.Config) *Client {
	return newClient(cfg, cfg.EmbeddingBaseURL, cfg.EmbeddingAPIKey)
}

// New returns a Client bound to the supplied configuration. Non-empty
// BaseURL and APIKey are forwarded to the SDK as explicit options;
// empty values are skipped so the SDK falls back to its own env-var
// defaults. APIKey in particular is skipped when empty because local
// providers (Ollama, LM Studio) don't require auth and a "Bearer "
// header with no token trips some servers.
func newClient(cfg config.Config, baseURL, apiKey string) *Client {
	opts := []option.RequestOption{}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	return &Client{cfg: cfg, sdk: openai.NewClient(opts...)}
}

func (c *Client) ChatStream(ctx context.Context, messages []Message, onDelta func(string)) (Message, error) {
	stream := c.sdk.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    c.cfg.Model,
		Messages: toSDKMessages(messages),
	})
	defer stream.Close()

	var content strings.Builder
	role := "assistant" // role is always assistant for streamed replies

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue // skip empty chunks
		}

		delta := chunk.Choices[0].Delta
		if delta.Role != "" {
			role = delta.Role // update role if provided (usually only in first chunk)
		}
		if delta.Content != "" {
			content.WriteString(delta.Content) // append new content
			if onDelta != nil {
				onDelta(delta.Content) // callback with the new delta
			}
		}
	}

	if err := stream.Err(); err != nil {
		return Message{}, err
	}

	return Message{Role: role, Content: content.String()}, nil
}

func toSDKMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, m := range messages {
		switch m.Role {
		case "system":
			out = append(out, openai.SystemMessage(m.Content))
		case "assistant":
			out = append(out, openai.AssistantMessage(m.Content))
		default:
			out = append(out, openai.UserMessage(m.Content))
		}
	}
	return out
}



