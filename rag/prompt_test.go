package rag

import (
	"go-rag/vector"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatContext_Empty(t *testing.T) {
	result := formatContext(nil)
	assert.Empty(t, result)

	result = formatContext([]vector.Result{})
	assert.Empty(t, result)
}

func TestFormatContext_SingleHit(t *testing.T) {
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content:  "Go is a statically typed language.",
				Metadata: map[string]string{"source": "go-intro.txt"},
			},
			Score: 0.95,
		},
	}

	result := formatContext(hits)
	assert.Contains(t, result, "[1]")
	assert.Contains(t, result, "go-intro.txt")
	assert.Contains(t, result, "Go is a statically typed language.")
	assert.Contains(t, result, "0.95")
}

func TestFormatContext_MultipleHits(t *testing.T) {
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content:  "First chunk.",
				Metadata: map[string]string{"source": "a.txt"},
			},
			Score: 0.90,
		},
		{
			Document: vector.Document{
				Content:  "Second chunk.",
				Metadata: map[string]string{"source": "b.txt"},
			},
			Score: 0.80,
		},
	}

	result := formatContext(hits)
	assert.Contains(t, result, "[1]")
	assert.Contains(t, result, "[2]")
	assert.Contains(t, result, "a.txt")
	assert.Contains(t, result, "b.txt")

	// [1] should appear before [2]
	assert.Less(t, strings.Index(result, "[1]"), strings.Index(result, "[2]"))
}

func TestFormatContext_MissingSource(t *testing.T) {
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content:  "Some content.",
				Metadata: map[string]string{},
			},
			Score: 0.70,
		},
	}

	result := formatContext(hits)
	assert.Contains(t, result, unknownSource)
}

func TestFormatContext_ImageHit(t *testing.T) {
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content: "A cat sitting on a mat.",
				Metadata: map[string]string{
					"source":     "cat.png",
					"type":       "image",
					"image_path": "/images/cat.png",
				},
			},
			Score: 0.88,
		},
	}

	result := formatContext(hits)
	assert.Contains(t, result, "cat.png")
	assert.Contains(t, result, "[image: /images/cat.png]")
	assert.Contains(t, result, "A cat sitting on a mat.")
}

func TestFormatContext_ImageHitMissingPath(t *testing.T) {
	// type=image but no image_path → falls through to normal format
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content: "Some image description.",
				Metadata: map[string]string{
					"source": "pic.png",
					"type":   "image",
				},
			},
			Score: 0.75,
		},
	}

	result := formatContext(hits)
	assert.Contains(t, result, "pic.png")
	assert.NotContains(t, result, "[image:")
}

func TestFormatContext_ContainsPreamble(t *testing.T) {
	hits := []vector.Result{
		{
			Document: vector.Document{
				Content:  "Any content.",
				Metadata: map[string]string{"source": "x.txt"},
			},
			Score: 0.50,
		},
	}

	result := formatContext(hits)
	// The preamble instructs citing sources; verify it's present
	assert.Contains(t, result, "Cite sources by filename")
}
