package ingest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seededEmbedder returns pre-seeded vectors for known strings; unknown texts
// get zero vectors of the same dimension as the first seed.
type seededEmbedder struct {
	seeds map[string][]float32
}

func (e seededEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	dim := 4
	for _, v := range e.seeds {
		dim = len(v)
		break
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := e.seeds[t]; ok {
			cp := make([]float32, len(v))
			copy(cp, v)
			out[i] = cp
		} else {
			out[i] = make([]float32, dim)
		}
	}
	return out, nil
}

// --- splitSentences ---

func TestSplitSentences_Empty(t *testing.T) {
	assert.Nil(t, splitSentences(""))
}

func TestSplitSentences_Whitespace(t *testing.T) {
	assert.Nil(t, splitSentences("   "))
}

func TestSplitSentences_NoTerminator(t *testing.T) {
	result := splitSentences("some content")
	require.Len(t, result, 1)
	assert.Equal(t, "some content", result[0])
}

func TestSplitSentences_SingleSentence(t *testing.T) {
	result := splitSentences("Hello world.")
	require.Len(t, result, 1)
	assert.Equal(t, "Hello world.", result[0])
}

func TestSplitSentences_TwoSentences(t *testing.T) {
	result := splitSentences("First sentence. Second sentence.")
	require.Len(t, result, 2)
	assert.Equal(t, "First sentence.", result[0])
	assert.Equal(t, "Second sentence.", result[1])
}

func TestSplitSentences_ParagraphBreak(t *testing.T) {
	result := splitSentences("Para one.\n\nPara two.")
	require.Len(t, result, 2)
	assert.Equal(t, "Para one.", result[0])
	assert.Equal(t, "Para two.", result[1])
}

func TestSplitSentences_ExclamationQuestion(t *testing.T) {
	result := splitSentences("Really! Are you sure?")
	require.Len(t, result, 2)
	assert.Equal(t, "Really!", result[0])
	assert.Equal(t, "Are you sure?", result[1])
}

// --- cosineSim ---

func TestCosineSim_BothZero(t *testing.T) {
	a := []float32{0, 0, 0}
	assert.Equal(t, 0.0, cosineSim(a, a))
}

func TestCosineSim_OneZero(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}
	assert.Equal(t, 0.0, cosineSim(a, b))
}

func TestCosineSim_Identical(t *testing.T) {
	a := []float32{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSim(a, a), 1e-9)
}

func TestCosineSim_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSim(a, b), 1e-9)
}

func TestCosineSim_AntiParallel(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}
	assert.InDelta(t, -1.0, cosineSim(a, b), 1e-9)
}

// --- chunkSemantic ---

func TestChunkSemantic_EmptyText(t *testing.T) {
	chunks, err := chunkSemantic(context.Background(), "", 0.75, errEmbedder{})
	require.NoError(t, err)
	assert.Nil(t, chunks)
}

func TestChunkSemantic_SingleSentence(t *testing.T) {
	// One sentence → no embed call → whole text returned as-is.
	chunks, err := chunkSemantic(context.Background(), "Hello world.", 0.75, errEmbedder{})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "Hello world.", chunks[0])
}

func TestChunkSemantic_NoTerminator(t *testing.T) {
	// splitSentences returns 1 item → no embed call.
	chunks, err := chunkSemantic(context.Background(), "some content", 0.75, errEmbedder{})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "some content", chunks[0])
}

func TestChunkSemantic_EmbedError(t *testing.T) {
	// Two sentences → tries to embed → error propagates.
	_, err := chunkSemantic(context.Background(), "First sentence. Second sentence.", 0.75, errEmbedder{})
	require.Error(t, err)
}

func TestChunkSemantic_AllSameTopic(t *testing.T) {
	s1 := "The cat sat on the mat."
	s2 := "Dogs are loyal companions."
	e := seededEmbedder{seeds: map[string][]float32{
		s1: {1, 0, 0, 0},
		s2: {1, 0, 0, 0}, // identical → sim=1.0 ≥ 0.75 → no split
	}}
	chunks, err := chunkSemantic(context.Background(), s1+" "+s2, 0.75, e)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Contains(t, chunks[0], "cat")
	assert.Contains(t, chunks[0], "Dogs")
}

func TestChunkSemantic_TopicShift(t *testing.T) {
	a1 := "The cat sat on the mat."
	a2 := "Dogs are loyal companions."
	b1 := "Computers process data using transistors."
	b2 := "Software engineers write code."

	text := a1 + " " + a2 + "\n\n" + b1 + " " + b2

	e := seededEmbedder{seeds: map[string][]float32{
		a1: {1, 0, 0, 0},
		a2: {1, 0, 0, 0},
		b1: {0, 1, 0, 0}, // orthogonal → sim=0 < 0.75 → split between a2 and b1
		b2: {0, 1, 0, 0},
	}}
	chunks, err := chunkSemantic(context.Background(), text, 0.75, e)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	assert.Contains(t, chunks[0], "cat")
	assert.Contains(t, chunks[0], "Dogs")
	assert.Contains(t, chunks[1], "Computers")
	assert.Contains(t, chunks[1], "Software")
}

// --- ProcessContent with semantic enabled ---

func TestProcessContent_SemanticEnabled(t *testing.T) {
	a1 := "The cat sat on the mat."
	a2 := "Dogs are loyal companions."
	b1 := "Computers process data using transistors."
	b2 := "Software engineers write code."
	text := a1 + " " + a2 + "\n\n" + b1 + " " + b2

	e := seededEmbedder{seeds: map[string][]float32{
		a1: {1, 0, 0, 0},
		a2: {1, 0, 0, 0},
		b1: {0, 1, 0, 0},
		b2: {0, 1, 0, 0},
	}}
	store := &fakeStore{}
	n, err := ProcessContent(context.Background(), "doc.txt", []byte(text), Options{SemanticThreshold: 0.75}, e, store)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, store.upserted, 2)
}
