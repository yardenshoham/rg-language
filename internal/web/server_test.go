package web_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/internal/web"
)

// get serves path with no models loaded; headers are name, value pairs. A recorder
// rather than a real listener, so the whole suite needs no ports and no bodies to close.
func get(t *testing.T, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	return getWith(t, web.Config{}, path, headers...)
}

func getWith(t *testing.T, config web.Config, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	web.NewServer(slog.New(slog.DiscardHandler), nil, config).ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	t.Parallel()
	rec := get(t, "/health")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}
}

// Hebrew is mojibake unless the charset is in both header and document.
func TestHomeIsUTF8Hebrew(t *testing.T) {
	t.Parallel()
	rec := get(t, "/")
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	page := rec.Body.String()
	for _, want := range []string{
		`<meta charset="utf-8">`,
		`lang="he"`,
		`dir="rtl"`,
		"שפת הריש גימל",
		`<textarea`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("home page is missing %q", want)
		}
	}
}

// Emptying the textarea asks for an empty transform, which must come back as an
// empty fragment, not an error, or the stale result stays on screen.
func TestEmptyTransformClearsTheResult(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/transform?text=", "/transform", "/transform?text=%20%20"} {
		rec := get(t, path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, rec.Code)
		}
		if got := rec.Body.String(); got != "" {
			t.Errorf("GET %s: body = %q, want empty", path, got)
		}
	}
}

// htmx swaps the result without navigating, so the URL stays shareable only if the
// response says what it should be. A plain navigation is already at its URL.
func TestHtmxKeepsTheURLShareable(t *testing.T) {
	t.Parallel()
	if got := get(t, "/transform?text=%20%20", "HX-Request", "true").Header().Get("HX-Replace-Url"); got != "/" {
		t.Errorf("HX-Replace-Url = %q, want %q", got, "/")
	}
	if got := get(t, "/transform?text=").Header().Get("HX-Replace-Url"); got != "" {
		t.Errorf("HX-Replace-Url = %q on a non-htmx request, want none", got)
	}
}

func TestAudioNeedsTheWavSuffix(t *testing.T) {
	t.Parallel()
	if rec := get(t, "/audio/deadbeef"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAbout(t *testing.T) {
	t.Parallel()
	rec := get(t, "/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if page := rec.Body.String(); !strings.Contains(page, "הכלל") {
		t.Error("about page does not explain the rule")
	}
}

// Analytics are opt-in, and each server answers with its own configuration.
func TestPostHogIsOptIn(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/", "/about"} {
		if page := get(t, path).Body.String(); strings.Contains(page, "posthog") {
			t.Errorf("%s mentions posthog with no key configured", path)
		}
	}

	t.Run("configured", func(t *testing.T) {
		t.Parallel()
		page := getWith(t, web.Config{
			PostHogKey:    "phc_test",
			PostHogHost:   "https://m.example.com",
			PostHogUIHost: "https://eu.posthog.com",
		}, "/").Body.String()
		want := `posthog.init("phc_test",{api_host:"https://m.example.com",ui_host:"https://eu.posthog.com",` +
			`defaults:'2026-05-30',person_profiles:'identified_only'})`
		if !strings.Contains(page, want) {
			t.Errorf("home page is missing %q", want)
		}
	})

	// The fragment htmx swaps in is body content: a second copy of the loader
	// there would re-init PostHog on every keystroke.
	t.Run("fragment", func(t *testing.T) {
		t.Parallel()
		if fragment := getWith(t, web.Config{PostHogKey: "phc_test"}, "/transform?text=").Body.String(); strings.Contains(fragment, "posthog") {
			t.Error("result fragment carries the analytics script")
		}
	})

	// Without an explicit host the snippet still needs one, or it has nowhere to
	// send events and no assets host to derive.
	t.Run("default host", func(t *testing.T) {
		t.Parallel()
		page := getWith(t, web.Config{PostHogKey: "phc_test"}, "/about").Body.String()
		if !strings.Contains(page, `posthog.init("phc_test",{api_host:"https://eu.i.posthog.com",defaults:`) {
			t.Error("about page does not fall back to the EU cloud host")
		}
	})
}

// Served from the binary: a CDN is one more thing to break.
func TestStaticAssets(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/static/app.css",
		"/static/htmx.min.js",
		"/static/favicon.svg",
		"/static/fonts/noto-sans-hebrew-400.woff2",
		"/static/fonts/noto-sans-hebrew-600.woff2",
	} {
		if rec := get(t, path); rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, rec.Code)
		}
	}
}
