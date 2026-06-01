package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a fully-populated Config equivalent to what Load()
// produces when all required env vars are set.
func validConfig() Config {
	return Config{
		BaseURL:        "http://localhost:11434/v1",
		Model:          "llama3",
		EmbeddingDim:   768,
		EmbeddingModel: "nomic-embed-text",
		IngestDir:      "./documents",
		ProcessedDir:   "./documents/processed",
		ImageDir:       "./documents/images",
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	require.NoError(t, Validate(validConfig()))
}

func TestValidate_EmbeddingDimZero(t *testing.T) {
	cfg := validConfig()
	cfg.EmbeddingDim = 0
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMBEDDING_DIM")
}

func TestValidate_EmbeddingDimNegative(t *testing.T) {
	cfg := validConfig()
	cfg.EmbeddingDim = -1
	err := Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMBEDDING_DIM")
}

func TestValidate_EmbeddingDimLargeValid(t *testing.T) {
	cfg := validConfig()
	cfg.EmbeddingDim = 1536
	require.NoError(t, Validate(cfg))
}

func TestValidate_EmbeddingDimOne(t *testing.T) {
	cfg := validConfig()
	cfg.EmbeddingDim = 1
	require.NoError(t, Validate(cfg))
}
