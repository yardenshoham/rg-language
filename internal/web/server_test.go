package web_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/internal/web"
)

// get serves path under config with no models loaded; headers are name, value pairs. A
// recorder rather than a real listener, so the suite needs no ports and no bodies to close.
func get(t *testing.T, config web.Config, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	web.NewServer(slog.New(slog.DiscardHandler), nil, config).ServeHTTP(rec, req)
	return rec
}

// check is get plus what almost every route asserts: the status, and what the body contains.
func check(t *testing.T, config web.Config, path string, status int, want ...string) *httptest.ResponseRecorder {
	t.Helper()
	rec := get(t, config, path)
	if rec.Code != status {
		t.Errorf("GET %s: status = %d, want %d", path, rec.Code, status)
	}
	for _, w := range want {
		if !strings.Contains(rec.Body.String(), w) {
			t.Errorf("GET %s is missing %q", path, w)
		}
	}
	return rec
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec := check(t, web.Config{}, "/health", http.StatusOK)
	if got := rec.Body.String(); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}
}

// Hebrew is mojibake unless the charset is in both header and document.
func TestHomeIsUTF8Hebrew(t *testing.T) {
	t.Parallel()
	rec := check(t, web.Config{}, "/", http.StatusOK,
		`<meta charset="utf-8">`,
		`lang="he"`,
		`dir="rtl"`,
		"שפת הריש גימל",
		`<textarea`,
	)
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

// Emptying the textarea asks for an empty transform, which must come back as an
// empty fragment, not an error, or the stale result stays on screen.
func TestEmptyTransformClearsTheResult(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/transform?text=", "/transform", "/transform?text=%20%20"} {
		rec := check(t, web.Config{}, path, http.StatusOK)
		if got := rec.Body.String(); got != "" {
			t.Errorf("GET %s: body = %q, want empty", path, got)
		}
	}
}

// htmx swaps the result without navigating, so the URL stays shareable only if the
// response says what it should be. A plain navigation is already at its URL.
func TestHtmxKeepsTheURLShareable(t *testing.T) {
	t.Parallel()
	if got := get(t, web.Config{}, "/transform?text=%20%20", "HX-Request", "true").Header().Get("HX-Replace-Url"); got != "/" {
		t.Errorf("HX-Replace-Url = %q, want %q", got, "/")
	}
	if got := get(t, web.Config{}, "/transform?text=").Header().Get("HX-Replace-Url"); got != "" {
		t.Errorf("HX-Replace-Url = %q on a non-htmx request, want none", got)
	}
}

// Every fixed route answers, assets included: they are served from the binary because
// a CDN is one more thing to break.
func TestRoutes(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/static/app.css", "/static/htmx.min.js", "/static/favicon.svg",
		"/static/fonts/noto-sans-hebrew-400.woff2", "/static/fonts/noto-sans-hebrew-600.woff2",
	} {
		check(t, web.Config{}, path, http.StatusOK)
	}
	check(t, web.Config{}, "/about", http.StatusOK, "הכלל")        // the about page explains the rule
	check(t, web.Config{}, "/audio/deadbeef", http.StatusNotFound) // the .wav suffix is required
}

// Analytics are opt-in, and each server answers with its own configuration.
func TestPostHogIsOptIn(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/", "/about"} {
		if page := get(t, web.Config{}, path).Body.String(); strings.Contains(page, "posthog") {
			t.Errorf("%s mentions posthog with no key configured", path)
		}
	}

	check(t, web.Config{
		PostHogKey:    "phc_test",
		PostHogHost:   "https://m.example.com",
		PostHogUIHost: "https://eu.posthog.com",
	}, "/", http.StatusOK,
		`posthog.init("phc_test",{api_host:"https://m.example.com",ui_host:"https://eu.posthog.com",`+
			`defaults:'2026-05-30',person_profiles:'identified_only'})`)

	// The fragment htmx swaps in is body content: a second copy of the loader
	// there would re-init PostHog on every keystroke.
	if fragment := get(t, web.Config{PostHogKey: "phc_test"}, "/transform?text=").Body.String(); strings.Contains(fragment, "posthog") {
		t.Error("result fragment carries the analytics script")
	}

	// Without an explicit host the snippet still needs one, or it has nowhere to
	// send events and no assets host to derive.
	check(t, web.Config{PostHogKey: "phc_test"}, "/about", http.StatusOK,
		`posthog.init("phc_test",{api_host:"https://eu.i.posthog.com",defaults:`)
}
