// Package web serves the site.
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	g "maragu.dev/gomponents"

	"github.com/yardenshoham/rg-language/internal/web/pages"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

//go:embed static
var staticFiles embed.FS

// maxInputRunes bounds what one request may ask the models to chew on.
const maxInputRunes = 500

// Server is the HTTP server. It is an http.Handler, middleware and all.
type Server struct {
	logger   *slog.Logger
	pipeline *pipeline.Pipeline
	mux      *http.ServeMux
	handler  http.Handler
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// NewServer registers all routes. The pipeline is loaded before the server
// starts listening, so by the time anything can reach these handlers both models
// are resident — which is also why Railway's health check does the right thing:
// the port simply does not answer until the models are up.
func NewServer(logger *slog.Logger, p *pipeline.Pipeline) *Server {
	s := &Server{logger: logger, pipeline: p, mux: http.NewServeMux()}

	s.mux.Handle("GET /static/", cacheAssets(http.FileServerFS(staticFiles)))
	s.mux.HandleFunc("GET /{$}", s.handleHome)
	s.mux.HandleFunc("GET /transform", s.handleTransform)
	// A ServeMux wildcard has to span a whole path segment, so the .wav suffix
	// comes off in the handler.
	s.mux.HandleFunc("GET /audio/{name}", s.handleAudio)
	s.mux.HandleFunc("GET /about", s.handleAbout)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	s.handler = s.loggingMiddleware(s.recoveryMiddleware(s.mux))
	return s
}

// ListenAndServe blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server starting", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// render writes n as an HTML response. The charset is not optional: without it
// browsers guess, and Hebrew comes out as mojibake.
func (s *Server) render(w http.ResponseWriter, what string, n g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if n == nil {
		// An empty fragment, which is how htmx clears the result when the box is
		// emptied. Rendering a nil node would panic.
		return
	}
	if err := n.Render(w); err != nil {
		s.logger.Error("rendering "+what, "error", err)
	}
}

// input reads and bounds the text field.
func input(r *http.Request) string {
	text := strings.TrimSpace(r.FormValue("text"))
	if runes := []rune(text); len(runes) > maxInputRunes {
		text = string(runes[:maxInputRunes])
	}
	return text
}

func (s *Server) transform(w http.ResponseWriter, r *http.Request) (string, pipeline.Result, bool) {
	text := input(r)
	if text == "" {
		return "", pipeline.Result{}, true
	}
	result, err := s.pipeline.Transform(r.Context(), text)
	if err != nil {
		s.logger.Error("transforming", "error", err)
		http.Error(w, "לא הצלחנו לתרגם את זה", http.StatusInternalServerError)
		return "", pipeline.Result{}, false
	}
	return text, result, true
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	text, result, ok := s.transform(w, r)
	if !ok {
		return
	}
	s.render(w, "home page", pages.Home(text, result))
}

func (s *Server) handleTransform(w http.ResponseWriter, r *http.Request) {
	_, result, ok := s.transform(w, r)
	if !ok {
		return
	}
	s.render(w, "result fragment", pages.Result(result))
}

func (s *Server) handleAbout(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "about page", pages.About())
}

// handleAudio serves synthesized speech. The hash covers the phonemes and the
// synthesis settings, so a URL's content can never change — which is what makes
// the immutable caching honest.
func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	hash, ok := strings.CutSuffix(r.PathValue("name"), ".wav")
	if !ok {
		http.NotFound(w, r)
		return
	}
	wav, err := s.pipeline.Audio(r.Context(), hash)
	switch {
	case errors.Is(err, pipeline.ErrUnknownAudio):
		s.logger.Info("audio not found", "hash", hash)
		http.Error(w, "אין הקלטה כזאת", http.StatusNotFound)
		return
	case err != nil:
		// Synthesis failed, which is ours to answer for, not the caller's.
		s.logger.Error("synthesizing audio", "hash", hash, "error", err)
		http.Error(w, "לא הצלחנו להקליט את זה", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	// ServeContent rather than a plain Write, for the range requests: Safari
	// probes a media URL with Range before it will play anything, and seeking
	// needs them everywhere. WAV barely compresses, so nothing gzips it.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(wav))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, "ok")
}

// cacheAssets caches the embedded assets for an hour. Their URLs are not
// content-addressed the way the audio's are, and an embedded file carries no
// modification time for the browser to revalidate against, so a long cache would
// mean a redeploy's new CSS not arriving for days. They total about 60 KB.
func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", time.Since(start).Round(time.Microsecond),
		)
	})
}

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("panic recovered", "error", err, "path", r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
