package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunk_Empty(t *testing.T) {
	assert.Nil(t, chunk("", 100, 10))
	assert.Nil(t, chunk("   ", 100, 10))
	assert.Nil(t, chunk("\n\n", 100, 10))
}

func TestChunk_ShortText(t *testing.T) {
	// Text shorter than size → single chunk, no split
	result := chunk("hello world", 100, 10)
	require.Len(t, result, 1)
	assert.Equal(t, "hello world", result[0])
}

func TestChunk_ExactSize(t *testing.T) {
	text := strings.Repeat("a", 100)
	result := chunk(text, 100, 0)
	require.Len(t, result, 1)
	assert.Equal(t, text, result[0])
}

func TestChunk_SplitsOnParagraph(t *testing.T) {
	// "\n\n" at position >= 70% of size triggers paragraph split
	// size=20, threshold=14; put \n\n at position 15
	text := "123456789012345\n\nABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := chunk(text, 20, 0)
	require.Greater(t, len(result), 1)
	// First chunk must end at or near the paragraph break
	assert.Contains(t, result[0], "123456789012345")
	assert.Contains(t, result[1], "ABCDEFGHIJ")
}

func TestChunk_SplitsOnSentence(t *testing.T) {
	// ". " at >= 70% of size; no paragraph break available
	// size=20, threshold=14; put ". " at position 15
	text := "First sentence is. Second part of this text goes on."
	result := chunk(text, 20, 0)
	require.Greater(t, len(result), 1)
	assert.True(t, strings.HasPrefix(result[0], "First sentence is"))
}

func TestChunk_SplitsOnWord(t *testing.T) {
	// No paragraph or sentence break; splits on last space >= 70%
	// size=10, threshold=7; "hello wor" has space at index 5 (< 7)
	// use a longer prefix so space falls at >= 7
	text := "helloXXX world and more text follows here"
	result := chunk(text, 10, 0)
	require.Greater(t, len(result), 1)
	// Each chunk should be trimmed (no leading/trailing whitespace)
	for _, c := range result {
		assert.Equal(t, strings.TrimSpace(c), c)
	}
}

func TestChunk_HardCut(t *testing.T) {
	// No whitespace at all → forced hard cut at size boundary
	text := strings.Repeat("x", 25)
	result := chunk(text, 10, 0)
	// 25 chars, size=10, overlap=0: expect at least 2 chunks
	require.GreaterOrEqual(t, len(result), 2)
	for _, c := range result {
		assert.LessOrEqual(t, len(c), 10)
	}
}

func TestChunk_OverlapPresent(t *testing.T) {
	// With overlap, the end of chunk N should appear at the start of chunk N+1
	text := "The quick brown fox jumps over the lazy dog and keeps running far away into the distance."
	result := chunk(text, 30, 10)
	require.Greater(t, len(result), 1)
	for i := 1; i < len(result); i++ {
		// Last 10 chars of previous chunk should appear somewhere in next chunk
		prev := result[i-1]
		tail := prev[max(0, len(prev)-10):]
		assert.Contains(t, result[i], strings.TrimSpace(tail),
			"chunk %d should overlap with chunk %d", i, i-1)
	}
}

func TestChunk_OverlapClampedWhenTooLarge(t *testing.T) {
	// overlap >= size → clamped to size/2; should not infinite-loop
	text := "word1 word2 word3 word4 word5 word6 word7 word8"
	result := chunk(text, 10, 15) // overlap > size
	require.NotNil(t, result)
	assert.Greater(t, len(result), 0)
}

func TestChunk_NegativeOverlap(t *testing.T) {
	// Negative overlap treated as 0
	text := "some text that is long enough to be split into multiple chunks here"
	result := chunk(text, 20, -5)
	require.NotNil(t, result)
	assert.Greater(t, len(result), 0)
}

func TestChunk_AllChunksNonEmpty(t *testing.T) {
	text := "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software."
	result := chunk(text, 30, 5)
	for i, c := range result {
		assert.NotEmpty(t, c, "chunk %d is empty", i)
		assert.Equal(t, strings.TrimSpace(c), c, "chunk %d has leading/trailing whitespace", i)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
