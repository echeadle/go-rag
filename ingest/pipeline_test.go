package ingest

import (
	"context"
	"errors"
	"go-rag/vector"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEmbedder returns zero-valued vectors of the given dimension.
type fakeEmbedder struct{ dim int }

func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, f.dim)
	}
	return out, nil
}

// errEmbedder always returns an error.
type errEmbedder struct{}

func (errEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errors.New("embed failed")
}

// mismatchEmbedder returns fewer vectors than requested.
type mismatchEmbedder struct{}

func (mismatchEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	return [][]float32{}, nil // always empty regardless of input length
}

// fakeStore records upserted documents and is a no-op for everything else.
type fakeStore struct {
	upserted []vector.Document
	deleted  []string
}

func (s *fakeStore) Upsert(_ context.Context, docs []vector.Document) error {
	s.upserted = append(s.upserted, docs...)
	return nil
}

func (s *fakeStore) DeleteBySource(_ context.Context, source string) error {
	s.deleted = append(s.deleted, source)
	return nil
}

func (s *fakeStore) Query(_ context.Context, _ []float32, _ int) ([]vector.Result, error) {
	return nil, nil
}

func (s *fakeStore) Delete(_ context.Context, _ []string) error { return nil }
func (s *fakeStore) Close() error                               { return nil }
func (s *fakeStore) Ping(_ context.Context) error               { return nil }

// errStore always fails on Upsert.
type errStore struct{ fakeStore }

func (errStore) Upsert(_ context.Context, _ []vector.Document) error {
	return errors.New("upsert failed")
}

// --- ProcessContent tests ---

func TestProcessContent_NilEmbedder(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("hello"), Options{}, nil, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedder")
}

func TestProcessContent_NilStore(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("hello"), Options{}, fakeEmbedder{dim: 4}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store")
}

func TestProcessContent_UnsupportedFormat(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.pdf", []byte("hello"), Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestProcessContent_EmptyContent(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte(""), Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestProcessContent_WhitespaceOnlyContent(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("   \n\n  "), Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestProcessContent_EmbedError(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("some content"), Options{}, errEmbedder{}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embed")
}

func TestProcessContent_EmbedMismatch(t *testing.T) {
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("some content"), Options{}, mismatchEmbedder{}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vectors")
}

func TestProcessContent_UpsertError(t *testing.T) {
	store := &errStore{}
	_, err := ProcessContent(context.Background(), "doc.txt", []byte("some content"), Options{}, fakeEmbedder{dim: 4}, store)
	require.Error(t, err)
}

func TestProcessContent_HappyPath(t *testing.T) {
	store := &fakeStore{}
	n, err := ProcessContent(context.Background(), "doc.txt", []byte("Go is a great language for building reliable software."), Options{}, fakeEmbedder{dim: 8}, store)
	require.NoError(t, err)
	assert.Greater(t, n, 0, "expected at least one chunk")
	assert.Len(t, store.upserted, n)
}

func TestProcessContent_DocumentMetadata(t *testing.T) {
	store := &fakeStore{}
	content := []byte("Metadata test content for the ingest pipeline.")
	_, err := ProcessContent(context.Background(), "/some/path/notes.md", content, Options{}, fakeEmbedder{dim: 4}, store)
	require.NoError(t, err)
	require.NotEmpty(t, store.upserted)

	doc := store.upserted[0]
	assert.Equal(t, "notes.md", doc.Metadata["source"], "source should be basename only")
	assert.NotEmpty(t, doc.Metadata["ingested_at"])
	assert.NotEmpty(t, doc.Metadata["chunk_index"])
	assert.NotEmpty(t, doc.Metadata["chunks"])
}

func TestProcessContent_DeleteBySourceCalledBeforeUpsert(t *testing.T) {
	store := &fakeStore{}
	_, err := ProcessContent(context.Background(), "notes.txt", []byte("some content here"), Options{}, fakeEmbedder{dim: 4}, store)
	require.NoError(t, err)
	assert.Contains(t, store.deleted, "notes.txt", "DeleteBySource should be called with the basename")
}

func TestProcessContent_ChunkCountMatchesUpserted(t *testing.T) {
	store := &fakeStore{}
	// Short content → one chunk
	n, err := ProcessContent(context.Background(), "small.txt", []byte("tiny"), Options{ChunkSize: 1000}, fakeEmbedder{dim: 4}, store)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, store.upserted, 1)
}

// --- ProcessImage tests ---

func TestProcessImage_NilEmbedder(t *testing.T) {
	_, err := ProcessImage(context.Background(), "photo.png", "A cat on a mat.", Options{}, nil, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedder")
}

func TestProcessImage_NilStore(t *testing.T) {
	_, err := ProcessImage(context.Background(), "photo.png", "A cat on a mat.", Options{}, fakeEmbedder{dim: 4}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "store")
}

func TestProcessImage_UnsupportedFormat(t *testing.T) {
	_, err := ProcessImage(context.Background(), "doc.txt", "A description.", Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestProcessImage_EmptyDescription(t *testing.T) {
	_, err := ProcessImage(context.Background(), "photo.png", "", Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestProcessImage_WhitespaceDescription(t *testing.T) {
	_, err := ProcessImage(context.Background(), "photo.png", "   ", Options{}, fakeEmbedder{dim: 4}, &fakeStore{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestProcessImage_HappyPath(t *testing.T) {
	store := &fakeStore{}
	n, err := ProcessImage(context.Background(), "cat.jpg", "A fluffy orange cat sitting on a window sill.", Options{}, fakeEmbedder{dim: 8}, store)
	require.NoError(t, err)
	assert.Greater(t, n, 0)
	assert.Len(t, store.upserted, n)
}

func TestProcessImage_DocumentMetadata(t *testing.T) {
	store := &fakeStore{}
	_, err := ProcessImage(context.Background(), "/uploads/cat.png", "A cat.", Options{}, fakeEmbedder{dim: 4}, store)
	require.NoError(t, err)
	require.NotEmpty(t, store.upserted)

	doc := store.upserted[0]
	assert.Equal(t, "cat.png", doc.Metadata["source"])
	assert.Equal(t, "image", doc.Metadata["type"])
	assert.Equal(t, "/images/cat.png", doc.Metadata["image_path"])
	assert.NotEmpty(t, doc.Metadata["ingested_at"])
}

func TestProcessImage_AllSupportedExtensions(t *testing.T) {
	extensions := []string{"photo.png", "photo.jpg", "photo.jpeg", "photo.webp", "anim.gif"}
	for _, name := range extensions {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			_, err := ProcessImage(context.Background(), name, "A description.", Options{}, fakeEmbedder{dim: 4}, store)
			require.NoError(t, err)
		})
	}
}
