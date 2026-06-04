package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go-rag/ingest"
	"go-rag/llm"
	"go-rag/rag"
	"go-rag/vector"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed templates/*.gohtml
var templatesFS embed.FS

const maxUploadBytes = 10 << 20

type uploadResponse struct {
	Source string `json:"source"`
	Bytes  int    `json:"bytes"`
	Chunks int    `json:"chunks"`
}

type Options struct {
	Addr             string
	SystemPromptFile string
	Title            string
	Store            vector.Store
	ProcessedDir     string
	ImagesDir        string
	Logger           *slog.Logger
	ServerAPIKey     string
	// RateLimitRequests is the max requests per IP per minute on /api/v1/* routes.
	// 0 disables rate limiting.
	RateLimitRequests int
}

type Server struct {
	client       *llm.Client
	embedder     *llm.Client
	retriever    *rag.Retriever
	store        vector.Store
	processedDir string
	imagesDir    string
	tpl          *template.Template
	system       string
	title        string
	logger            *slog.Logger
	serverAPIKey      string
	rateLimitRequests int
}

func New(client, embedder *llm.Client, retriever *rag.Retriever, opts Options) (*Server, error) {
	tpl, err := template.ParseFS(templatesFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	title := opts.Title
	if title == "" {
		title = "RAG Chat"
	}

	lg := opts.Logger
	if lg == nil {
		lg = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.ServerAPIKey == "" {
		lg.Warn("API_KEY not set — authentication disabled; all /api/v1/* routes are open")
	}
	if opts.RateLimitRequests > 0 {
		lg.Info("rate limiting enabled", slog.Int("requests_per_minute", opts.RateLimitRequests))
	} else {
		lg.Warn("rate limiting disabled")
	}
	return &Server{
		client:       client,
		embedder:     embedder,
		retriever:    retriever,
		store:        opts.Store,
		processedDir: opts.ProcessedDir,
		imagesDir:    opts.ImagesDir,
		tpl:          tpl,
		system:       readSystemPrompt(opts.SystemPromptFile),
		title:        title,
		logger:            lg,
		serverAPIKey:      opts.ServerAPIKey,
		rateLimitRequests: opts.RateLimitRequests,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(s.logger))

	// Open routes — no auth required.
	r.Get("/healthz", s.handleHealthz)
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/chat", s.handleChatPage)

	// /images/* must stay open: <img src> tags cannot send Authorization headers.
	fs := http.FileServer(http.Dir(s.imagesDir))
	r.Handle("/images/*", http.StripPrefix("/images", fs))

	// All /api/v1/* routes require a valid Bearer token and are rate-limited per IP.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(requireAuth(s.serverAPIKey))
		if s.rateLimitRequests > 0 {
			r.Use(httprate.LimitByIP(s.rateLimitRequests, time.Minute))
		}

		// Injection defense on text-input routes (chat stream and document upload).
		r.Group(func(r chi.Router) {
			r.Use(InjectionDefense(s.logger))
			r.Post("/chat/stream", s.handleChatStream)
			r.Post("/upload", s.handleUpload)
			if s.imagesDir != "" {
				r.Post("/upload/image", s.handleUploadImage)
			}
		})

		// Caption: authed but no injection defense (binary file upload, not text).
		if s.client != nil && s.client.HasVision() {
			r.Post("/caption", s.handleCaption)
		}
	})

	return r
}

type captionResponse struct {
	Description string `json:"description"`
}

func (s *Server) handleCaption(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "upload too large or malformed: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !ingest.IsImage(filepath.Base(header.Filename)) {
		http.Error(w, "unsupported image format", http.StatusUnsupportedMediaType)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	mime := header.Header.Get("Content-Type")
	desc, err := s.client.DescribeImage(r.Context(), mime, content)
	if err != nil {
		s.logger.Error("caption failed", slog.String("file", header.Filename), slog.Any("error", err))
		http.Error(w, "caption failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(captionResponse{Description: strings.TrimSpace(desc)})
}

type uploadImageResponse struct {
	Source      string `json:"source"`
	ImagePath   string `json:"image_path"`
	Description string `json:"description"`
	Bytes       int    `json:"bytes"`
	Chunks      int    `json:"chunks"`
}

func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		http.Error(w, "ingest is not configured (no vector store)", http.StatusServiceUnavailable)
		return
	}

	if s.imagesDir == "" {
		http.Error(w, "image upload not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "upload too large or malformed: "+err.Error(), http.StatusBadRequest)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing 'image' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	original := filepath.Base(header.Filename)
	if !ingest.IsImage(original) {
		http.Error(w, "unsupported image format (allowed: .png, .jpg, .jpeg, .webp, .gif)", http.StatusUnsupportedMediaType)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	saved := fmt.Sprintf("%d-%s", time.Now().UnixNano(), safeFileName(original))

	if err := os.MkdirAll(s.imagesDir, 0o755); err != nil {
		http.Error(w, "mkdir images dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dest := filepath.Join(s.imagesDir, saved)
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		http.Error(w, "write image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	chunks, err := ingest.ProcessImage(r.Context(), saved, description, ingest.Options{}, s.embedder, s.store)
	if err != nil {
		_ = os.Remove(dest)
		s.logger.Error("image ingest failed", slog.String("file", saved), slog.Any("error", err))
		http.Error(w, "ingest failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(uploadImageResponse{
		Source:      saved,
		ImagePath:   ingest.ImagePathPrefix + saved,
		Description: description,
		Bytes:       len(content),
		Chunks:      chunks,
	})

}

func safeFileName(name string) string {
	var sb strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	out := sb.String()
	if out == "" || out == "." || out == ".." {
		return "image"
	}
	return out
}

type chatRequest struct {
	Messages []llm.Message `json:"messages"`
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		http.Error(w, "ingest is not configured (no vector store)", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "upload too large or malformed: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file' field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	if !ingest.IsSupported(name) {
		http.Error(w, "unsupported format", http.StatusUnsupportedMediaType)
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	chunks, err := ingest.ProcessContent(r.Context(), name, content, ingest.Options{}, s.embedder, s.store)
	if err != nil {
		s.logger.Error("upload ingest failed", slog.String("file", name), slog.Any("error", err))
		http.Error(w, "ingest failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if s.processedDir != "" {
		dest := filepath.Join(s.processedDir, name)
		if err := os.MkdirAll(s.processedDir, 0o755); err != nil {
			s.logger.Error("mkdir processed dir", slog.String("dir", s.processedDir), slog.Any("error", err))
		} else if err := os.WriteFile(dest, content, 0o644); err != nil {
			s.logger.Error("archive file", slog.String("dest", dest), slog.Any("error", err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(uploadResponse{
		Source: name,
		Bytes:  len(content),
		Chunks: chunks,
	})

}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json:"+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages must not be empty", http.StatusBadRequest)
		return
	}

	if last := req.Messages[len(req.Messages)-1]; last.Role != "user" {
		http.Error(w, "last message must be from user", http.StatusBadRequest)
		return
	}

	history := req.Messages
	if s.system != "" {
		history = append([]llm.Message{{Role: "system", Content: s.system}}, history...)
	}

	turn := history
	if s.retriever != nil {
		ctxText, err := s.retriever.Retrieve(r.Context(), history)
		if err != nil {
			s.logger.Error("retrieval error", slog.Any("error", err))
		} else {
			turn = withInlineContext(history, ctxText)
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	send := func(event, data string) {
		if event != "" {
			fmt.Fprintf(w, "event: %s\n", event)
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	_, err := s.client.ChatStream(r.Context(), turn, func(delta string) {
		enc, _ := json.Marshal(delta)
		send("delta", string(enc))
	})
	if err != nil {
		enc, _ := json.Marshal(err.Error())
		send("error", string(enc))
		return
	}
	send("done", `""`)
}

func withInlineContext(history []llm.Message, contextText string) []llm.Message {
	if len(history) == 0 || contextText == "" {
		return history
	}
	last := history[len(history)-1]
	if last.Role != "user" {
		return history
	}
	out := make([]llm.Message, len(history))
	copy(out, history)
	out[len(out)-1] = llm.Message{
		Role:    "user",
		Content: contextText + "\n\n--- Question ---\n\n" + last.Content,
	}
	return out
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.store.Ping(ctx); err != nil {
			s.logger.Error("healthz db ping failed", slog.Any("error", err))
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func (s *Server) handleChatPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.ExecuteTemplate(w, "chat.gohtml", map[string]any{
		"Title":          s.title,
		"CaptionEnabled": s.client.HasVision(),
	}); err != nil {
		s.logger.Error("template error", slog.Any("error", err))
	}
}

// requireAuth returns middleware that checks for a valid Bearer token.
// When key is empty (API_KEY unset) every request is allowed through — this
// preserves local dev behaviour without requiring a key to be configured.
func requireAuth(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			got := strings.TrimPrefix(header, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			// Skip high-frequency scrape/health routes to avoid log spam.
			if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
				return
			}

			// Use chi's resolved route pattern to avoid unbounded label cardinality
			// (e.g. /images/* must not become one series per filename).
			pattern := chi.RouteContext(r.Context()).RoutePattern()
			if pattern == "" {
				pattern = r.URL.Path
			}

			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)),
			)
			requestsTotal.WithLabelValues(pattern, fmt.Sprintf("%d", ww.Status())).Inc()
		})
	}
}

func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutDownCtrx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutDownCtrx)
		return nil
	case err := <-errCh:
		return err
	}
}

func readSystemPrompt(path string) string {
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}

	return strings.TrimSpace(string(data))
}
