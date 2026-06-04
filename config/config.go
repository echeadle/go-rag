package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BaseURL          string
	APIKey           string
	Model            string
	SystemPromptFile string
	DatabaseURL      string
	EmbeddingDim     int
	EmbeddingBaseURL string
	EmbeddingAPIKey  string
	EmbeddingModel   string
	IngestDir        string
	ProcessedDir     string

	HTTPAddr    string
	ImageDir    string
	VisionModel string
	// ServerAPIKey guards all /api/v1/* routes. Loaded from API_KEY env var.
	// Distinct from APIKey (OPENAI_API_KEY) — different secret, different purpose.
	ServerAPIKey string

	// RateLimitRequests is the max requests per IP per minute on /api/v1/* routes.
	// 0 disables rate limiting.
	RateLimitRequests int

	// ChunkSemanticThreshold enables semantic chunking when > 0.
	// Sentences with adjacent cosine similarity below this value become chunk
	// boundaries. 0 disables semantic chunking (uses fixed-size chunker).
	ChunkSemanticThreshold float64
}

func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		BaseURL:          os.Getenv("OPENAI_BASE_URL"),
		APIKey:           os.Getenv("OPENAI_API_KEY"),
		Model:            os.Getenv("OPENAI_MODEL"),
		SystemPromptFile: os.Getenv("SYSTEM_PROMPT_FILE"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		EmbeddingDim:     atoiOr(os.Getenv("EMBEDDING_DIM"), 0),
		EmbeddingBaseURL: os.Getenv("EMBEDDING_BASE_URL"),
		EmbeddingAPIKey:  os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingModel:   os.Getenv("EMBEDDING_MODEL"),
		IngestDir:        os.Getenv("INGEST_DIR"),
		ProcessedDir:     os.Getenv("PROCESSED_DIR"),
		HTTPAddr:         os.Getenv("HTTP_ADDR"),
		ImageDir:         os.Getenv("IMAGES_DIR"),
		VisionModel:      os.Getenv("VISION_MODEL"),
		ServerAPIKey:           os.Getenv("API_KEY"),
		RateLimitRequests:      atoiOr(os.Getenv("RATE_LIMIT_REQUESTS"), 100),
		ChunkSemanticThreshold: parseFloatOr(os.Getenv("CHUNK_SEMANTIC_THRESHOLD"), 0.75),
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}

	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 768
	}

	if cfg.EmbeddingBaseURL == "" {
		cfg.EmbeddingBaseURL = cfg.BaseURL
		if cfg.EmbeddingAPIKey == "" {
			cfg.EmbeddingAPIKey = cfg.APIKey
		}
	}

	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "nomic-embed-text"
	}

	if cfg.IngestDir == "" {
		cfg.IngestDir = "./documents"
	}

	if cfg.ProcessedDir == "" {
		cfg.ProcessedDir = "./documents/processed"
	}

	if cfg.ImageDir == "" {
		cfg.ImageDir = "./documents/images"
	}

	return cfg
}

// Validate checks that cfg contains sensible values. Call it after Load()
// and before starting any goroutines so bad config exits early with a clear message.
func Validate(cfg Config) error {
	if cfg.EmbeddingDim <= 0 {
		return fmt.Errorf("EMBEDDING_DIM must be greater than zero, got %d", cfg.EmbeddingDim)
	}
	return nil
}

func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func parseFloatOr(s string, fallback float64) float64 {
	if s == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return f
}
