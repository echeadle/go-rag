package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSupported(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"txt lowercase", "doc.txt", true},
		{"md lowercase", "notes.md", true},
		{"markdown lowercase", "readme.markdown", true},
		{"txt uppercase", "DOC.TXT", true},
		{"md uppercase", "NOTES.MD", true},
		{"path with dir", "/some/path/file.txt", true},
		{"pdf unsupported", "file.pdf", false},
		{"png unsupported", "image.png", false},
		{"docx unsupported", "report.docx", false},
		{"no extension", "noextension", false},
		{"empty string", "", false},
		{"just a dot", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSupported(tt.input))
		})
	}
}
