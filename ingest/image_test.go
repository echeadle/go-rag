package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsImage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"png lowercase", "photo.png", true},
		{"jpg lowercase", "photo.jpg", true},
		{"jpeg lowercase", "photo.jpeg", true},
		{"webp lowercase", "photo.webp", true},
		{"gif lowercase", "anim.gif", true},
		{"png uppercase", "PHOTO.PNG", true},
		{"jpg uppercase", "PHOTO.JPG", true},
		{"mixed case", "Photo.Jpeg", true},
		{"path with dir", "/images/photo.png", true},
		{"txt unsupported", "doc.txt", false},
		{"pdf unsupported", "doc.pdf", false},
		{"mp4 unsupported", "video.mp4", false},
		{"no extension", "noextension", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsImage(tt.input))
		})
	}
}
