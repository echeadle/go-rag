package pgvector

import (
	"testing"

	"go-rag/vector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doc(id string) vector.Result {
	return vector.Result{Document: vector.Document{ID: id, Content: id}}
}

func TestRRF_BothEmpty(t *testing.T) {
	out := reciprocalRankFusion(nil, nil, 5)
	assert.Empty(t, out)
}

func TestRRF_OneListOnly(t *testing.T) {
	a := []vector.Result{doc("a"), doc("b"), doc("c")}
	out := reciprocalRankFusion(a, nil, 5)
	require.Len(t, out, 3)
	// First result has highest RRF score (rank 1 in list a, absent from b)
	assert.Equal(t, "a", out[0].ID)
	assert.Equal(t, "b", out[1].ID)
	assert.Equal(t, "c", out[2].ID)
}

func TestRRF_NoOverlap(t *testing.T) {
	a := []vector.Result{doc("a1"), doc("a2")}
	b := []vector.Result{doc("b1"), doc("b2")}
	out := reciprocalRankFusion(a, b, 10)
	require.Len(t, out, 4)
	ids := make([]string, len(out))
	for i, r := range out {
		ids[i] = r.ID
	}
	assert.Contains(t, ids, "a1")
	assert.Contains(t, ids, "b1")
}

func TestRRF_OverlapBoostsScore(t *testing.T) {
	// "shared" appears rank-1 in both lists — should score highest.
	// "vector-only" appears rank-1 in list a only.
	a := []vector.Result{doc("shared"), doc("vector-only")}
	b := []vector.Result{doc("shared"), doc("keyword-only")}
	out := reciprocalRankFusion(a, b, 5)
	require.NotEmpty(t, out)
	assert.Equal(t, "shared", out[0].ID, "doc in both lists should rank first")
}

func TestRRF_TopKCap(t *testing.T) {
	a := []vector.Result{doc("a"), doc("b"), doc("c")}
	b := []vector.Result{doc("d"), doc("e"), doc("f")}
	out := reciprocalRankFusion(a, b, 2)
	assert.Len(t, out, 2)
}

func TestRRF_ScoreSetOnOutput(t *testing.T) {
	a := []vector.Result{doc("x")}
	out := reciprocalRankFusion(a, nil, 5)
	require.Len(t, out, 1)
	assert.Greater(t, out[0].Score, float32(0), "RRF score should be positive")
}

func TestRRF_Deduplication(t *testing.T) {
	// Same ID in both lists should appear exactly once in output.
	shared := doc("dup")
	out := reciprocalRankFusion([]vector.Result{shared}, []vector.Result{shared}, 10)
	count := 0
	for _, r := range out {
		if r.ID == "dup" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate ID should appear once")
}
