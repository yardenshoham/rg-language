package web_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/internal/web"
)

// get serves path with no models loaded; headers are name, value pairs.
func get(t *testing.T, path string, headers ...string) *http.Response {
	t.Helper()
	return getWith(t, web.Config{}, path, headers...)
}

func getWith(t *testing.T, config web.Config, path string, headers ...string) *http.Response {
	t.Helper()
	srv := httptest.NewServer(web.NewServer(slog.New(slog.DiscardHandler), nil, config))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

func TestHealth(t *testing.T) {
	t.Parallel()
	resp := get(t, "/health")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := body(t, resp); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}
}

// Hebrew is mojibake unless the charset is in both header and document.
func TestHomeIsUTF8Hebrew(t *testing.T) {
	t.Parallel()
	resp := get(t, "/")
	if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	page := body(t, resp)
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
		resp := get(t, path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
		}
		if got := body(t, resp); got != "" {
			t.Errorf("GET %s: body = %q, want empty", path, got)
		}
	}
}

// htmx swaps the result without navigating, so the URL stays shareable only if the
// response says what it should be. A plain navigation is already at its URL.
func TestHtmxKeepsTheURLShareable(t *testing.T) {
	t.Parallel()
	if got := get(t, "/transform?text=%20%20", "HX-Request", "true").Header.Get("HX-Replace-Url"); got != "/" {
		t.Errorf("HX-Replace-Url = %q, want %q", got, "/")
	}
	if got := get(t, "/transform?text=").Header.Get("HX-Replace-Url"); got != "" {
		t.Errorf("HX-Replace-Url = %q on a non-htmx request, want none", got)
	}
}

func TestAudioNeedsTheWavSuffix(t *testing.T) {
	t.Parallel()
	if resp := get(t, "/audio/deadbeef"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestAbout(t *testing.T) {
	t.Parallel()
	resp := get(t, "/about")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if page := body(t, resp); !strings.Contains(page, "הכלל") {
		t.Error("about page does not explain the rule")
	}
}

// Analytics are opt-in, and each server answers with its own configuration.
func TestPostHogIsOptIn(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/", "/about"} {
		if page := body(t, get(t, path)); strings.Contains(page, "posthog") {
			t.Errorf("%s mentions posthog with no key configured", path)
		}
	}

	t.Run("configured", func(t *testing.T) {
		t.Parallel()
		page := body(t, getWith(t, web.Config{
			PostHogKey:    "phc_test",
			PostHogHost:   "https://m.example.com",
			PostHogUIHost: "https://eu.posthog.com",
		}, "/"))
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
		if fragment := body(t, getWith(t, web.Config{PostHogKey: "phc_test"}, "/transform?text=")); strings.Contains(fragment, "posthog") {
			t.Error("result fragment carries the analytics script")
		}
	})

	// Without an explicit host the snippet still needs one, or it has nowhere to
	// send events and no assets host to derive.
	t.Run("default host", func(t *testing.T) {
		t.Parallel()
		page := body(t, getWith(t, web.Config{PostHogKey: "phc_test"}, "/about"))
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
		if resp := get(t, path); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
		}
	}
}
