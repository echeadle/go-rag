package ingest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDF extracts plain text from a PDF byte slice.
// Returns an error for corrupted PDFs or PDFs with no extractable text
// (e.g. image-only / scanned documents).
func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	tr, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}

	raw, err := io.ReadAll(tr)
	if err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}

	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", errors.New("pdf contains no extractable text (image-only or empty document)")
	}

	return text, nil
}
