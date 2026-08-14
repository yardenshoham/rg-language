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

// get serves path with no models loaded — most of the site needs none. headers
// are set on the request as name, value pairs.
func get(t *testing.T, path string, headers ...string) *http.Response {
	t.Helper()
	srv := httptest.NewServer(web.NewServer(slog.New(slog.DiscardHandler), nil))
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
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := get(t, path)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
			if got := body(t, resp); got != "" {
				t.Errorf("body = %q, want empty", got)
			}
		})
	}
}

// htmx swaps the result without navigating, so the URL only stays shareable if
// the response says what it should be. Emptying the box must clear the query
// rather than leave ?text= behind, and a plain navigation is already at its URL.
// The Hebrew round trip needs the models, so the browser suite covers that.
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

// Served from the binary: a CDN is one more thing to break, and Hebrew needs a
// font with real mark positioning.
func TestStaticAssets(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/static/app.css",
		"/static/htmx.min.js",
		"/static/favicon.svg",
		"/static/fonts/noto-sans-hebrew-400.woff2",
		"/static/fonts/noto-sans-hebrew-600.woff2",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := get(t, path)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
		})
	}
}
