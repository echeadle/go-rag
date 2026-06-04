package pgvector

import (
	"sort"

	"go-rag/vector"
)

// rrfK is the rank constant used in Reciprocal Rank Fusion.
// 60 is the standard value from the original RRF paper (Cormack et al. 2009).
const rrfK = 60

// reciprocalRankFusion merges two ranked result lists using RRF.
// A document's score is the sum of 1/(rrfK+rank) across every list it
// appears in, so a document ranked first in both lists scores highest.
// Results are sorted by RRF score descending and capped at topK.
func reciprocalRankFusion(a, b []vector.Result, topK int) []vector.Result {
	type entry struct {
		doc   vector.Result
		score float64
	}

	seen := make(map[string]*entry, len(a)+len(b))

	add := func(list []vector.Result) {
		for i, r := range list {
			s := 1.0 / float64(rrfK+i+1)
			if e, ok := seen[r.ID]; ok {
				e.score += s
			} else {
				cp := r
				seen[r.ID] = &entry{doc: cp, score: s}
			}
		}
	}
	add(a)
	add(b)

	merged := make([]entry, 0, len(seen))
	for _, e := range seen {
		merged = append(merged, *e)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].score > merged[j].score
	})

	if topK > 0 && len(merged) > topK {
		merged = merged[:topK]
	}

	out := make([]vector.Result, len(merged))
	for i, e := range merged {
		out[i] = e.doc
		out[i].Score = float32(e.score)
	}
	return out
}
