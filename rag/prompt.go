package rag

import (
	"fmt"
	"go-rag/vector"
	"strings"
)

const contextPreamble = ` Use the following excerpts from the document collection to answer the question.
Cite sources by filename when you draw from them. If the excerpts do not contain enough information
to answer the question, say so clearly — do not answer from general knowledge.`

const unknownSource = "(unknown source)"

func formatContext(hits []vector.Result) string {
	if len(hits) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(contextPreamble)
	sb.WriteString("\n\n---Excerpts---\n\n")

	for i, h := range hits {
		source := h.Metadata["source"]
		if source == "" {
			source = unknownSource
		}
		if h.Metadata["type"] == "image" && h.Metadata["image_path"] != "" {
			fmt.Fprintf(&sb, "[%d] Source: %s [image: %s] (similarity %.2f)\n%s\n\n",
				i+1, source, h.Metadata["image_path"], h.Score, h.Content)
			continue
		}
		fmt.Fprintf(&sb, "[%d] Source: %s (similarity %.2f)\n%s\n\n", i+1, source, h.Score, h.Content)
	}

	return strings.TrimSpace(sb.String())
}
