package ingest

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"go-rag/llm"
)

var sentenceTermRe = regexp.MustCompile(`[.!?]+`)

// splitSentences splits text into individual sentences.
// Paragraph breaks (\n\n) are handled first; within each paragraph
// sentence terminators ([.!?]+) mark boundaries. A paragraph with no
// terminators is returned as a single unit.
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var sentences []string
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		locs := sentenceTermRe.FindAllStringIndex(para, -1)
		if len(locs) == 0 {
			sentences = append(sentences, para)
			continue
		}
		prev := 0
		for _, loc := range locs {
			if s := strings.TrimSpace(para[prev:loc[1]]); s != "" {
				sentences = append(sentences, s)
			}
			prev = loc[1]
			for prev < len(para) && para[prev] == ' ' {
				prev++
			}
		}
		if prev < len(para) {
			if s := strings.TrimSpace(para[prev:]); s != "" {
				sentences = append(sentences, s)
			}
		}
	}
	return sentences
}

func cosineSim(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(na) * math.Sqrt(nb)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// chunkSemantic groups sentences into chunks by detecting similarity valleys.
// Sentences whose adjacent cosine similarity falls below threshold are split
// into separate chunks. Returns the full text as a single chunk when fewer
// than two sentences are found (skips the embed call entirely).
func chunkSemantic(ctx context.Context, text string, threshold float64, embedder llm.Embedder) ([]string, error) {
	sentences := splitSentences(text)
	if len(sentences) < 2 {
		if s := strings.TrimSpace(text); s != "" {
			return []string{s}, nil
		}
		return nil, nil
	}

	vecs, err := embedder.Embed(ctx, sentences)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(sentences) {
		return nil, fmt.Errorf("semantic chunk: got %d vectors for %d sentences", len(vecs), len(sentences))
	}

	var chunks []string
	start := 0
	for i := 0; i < len(sentences)-1; i++ {
		if cosineSim(vecs[i], vecs[i+1]) < threshold {
			if c := strings.TrimSpace(strings.Join(sentences[start:i+1], " ")); c != "" {
				chunks = append(chunks, c)
			}
			start = i + 1
		}
	}
	if last := strings.TrimSpace(strings.Join(sentences[start:], " ")); last != "" {
		chunks = append(chunks, last)
	}
	return chunks, nil
}
