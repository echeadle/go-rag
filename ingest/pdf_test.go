package ingest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPDF_KnownGood(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.pdf")
	require.NoError(t, err)

	text, err := extractPDF(data)
	require.NoError(t, err)
	assert.Contains(t, text, "This is a heading")
	assert.Contains(t, text, "This is content")
}

func TestExtractPDF_Corrupted(t *testing.T) {
	_, err := extractPDF([]byte("this is not a pdf"))
	assert.Error(t, err)
}

func TestExtractPDF_EmptyBytes(t *testing.T) {
	_, err := extractPDF([]byte{})
	assert.Error(t, err)
}
