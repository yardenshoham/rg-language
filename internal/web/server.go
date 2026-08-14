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
	"net/url"
	"strings"
	"time"

	g "maragu.dev/gomponents"

	"github.com/yardenshoham/rg-language/internal/web/pages"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

//go:embed static
var staticFiles embed.FS

// maxInputRunes bounds what one request may ask the models to do.
const maxInputRunes = 500

// Server is the HTTP server, middleware and all.
type Server struct {
	logger   *slog.Logger
	pipeline *pipeline.Pipeline
	handler  http.Handler
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// NewServer registers all routes. The pipeline is loaded before the port opens, so
// both models are resident by the time anything can reach these handlers — which
// is also what makes Railway's health check honest.
func NewServer(logger *slog.Logger, p *pipeline.Pipeline) *Server {
	s := &Server{logger: logger, pipeline: p}
	mux := http.NewServeMux()

	mux.Handle("GET /static/", cacheAssets(http.FileServerFS(staticFiles)))
	mux.HandleFunc("GET /{$}", s.transformed("home page", pages.Home))
	mux.HandleFunc("GET /transform", s.transformed("result fragment",
		func(_ string, result pipeline.Result) g.Node { return pages.Result(result) }))
	// A ServeMux wildcard spans a whole segment, so .wav comes off in the handler.
	mux.HandleFunc("GET /audio/{name}", s.handleAudio)
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, _ *http.Request) {
		s.render(w, "about page", pages.About())
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	s.handler = s.instrument(mux)
	return s
}

// ListenAndServe blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 10 * time.Second}

	s.logger.Info("server starting", "addr", addr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

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

// render writes n as HTML. The charset is not optional: without it browsers guess
// and Hebrew comes out as mojibake.
func (s *Server) render(w http.ResponseWriter, what string, n g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := n.Render(w); err != nil {
		s.logger.Error("rendering "+what, "error", err)
	}
}

// transformed builds the handler for a page made of transformed input. An empty
// box short-circuits to the zero Result, which renders as nothing.
func (s *Server) transformed(what string, page func(string, pipeline.Result) g.Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		text := strings.TrimSpace(r.FormValue("text"))
		if runes := []rune(text); len(runes) > maxInputRunes {
			text = string(runes[:maxInputRunes])
		}
		// Keep the address bar in step with the box, so the URL is always
		// shareable. htmx ignores this on a plain navigation, where it is already
		// the URL. The bounded text is what goes in, so a shared link cannot ask
		// for more than the box allowed.
		if r.Header.Get("HX-Request") != "" {
			shareable := "/"
			if text != "" {
				shareable += "?text=" + url.QueryEscape(text)
			}
			w.Header().Set("HX-Replace-Url", shareable)
		}

		var result pipeline.Result
		if text != "" {
			var err error
			if result, err = s.pipeline.Transform(r.Context(), text); err != nil {
				s.logger.Error("transforming", "error", err)
				http.Error(w, "לא הצלחנו לתרגם את זה", http.StatusInternalServerError)
				return
			}
		}
		s.render(w, what, page(text, result))
	}
}

// handleAudio serves synthesized speech. The hash covers the phonemes and the
// synthesis settings, so a URL's content can never change — hence immutable.
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
	// ServeContent, not Write, for the range requests: Safari probes with Range
	// before playing anything, and seeking needs them everywhere. WAV barely
	// compresses, so nothing gzips it.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(wav))
}

// cacheAssets caches the embedded assets for an hour — not longer, because their
// URLs are not content-addressed and an embedded file has no modification time to
// revalidate against, so a redeploy's new CSS would take days to arrive. They total
// about 60 KB, so re-fetching is cheap.
func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}

// instrument logs every request and turns a panic into a 500. Recovery runs before
// the log line so the log reports the 500.
func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("panic recovered", "error", err, "path", r.URL.Path)
				http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
			}
			s.logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration", time.Since(start).Round(time.Microsecond),
			)
		}()
		next.ServeHTTP(rw, r)
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
