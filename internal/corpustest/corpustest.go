// Package corpustest reads the differential corpus several packages' tests replay:
// 5,012 Hebrew items run once through the original Python implementation with every
// stage recorded. The port is deterministic and the fork frozen, so this pins it
// byte-for-byte. Its own package because an external test package cannot be imported.
package corpustest

import (
	"cmp"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type Item struct {
	Text      string `json:"text"`
	Raw       string `json:"raw"`
	Vocalized string `json:"vocalized"`
	IPA       string `json:"ipa"`
	RG        string `json:"rg"`
	HebRG     string `json:"heb_rg"`
	Latin     string `json:"latin"`
}

func Load(tb testing.TB, path string) []Item {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("opening corpus: %v", err)
	}
	defer f.Close()

	var items []Item
	for dec := json.NewDecoder(f); dec.More(); {
		var item Item
		if err := dec.Decode(&item); err != nil {
			tb.Fatalf("decoding corpus item %d: %v", len(items)+1, err)
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		tb.Fatal("corpus is empty")
	}
	return items
}

// Model opens file from $RG_MODELS_DIR, skipping the test when it is not there — the
// checkpoints are a download, not a checkout — and closing it when the test ends.
func Model[T interface{ Close() error }](tb testing.TB, file string, open func(context.Context, string) (T, error)) T {
	tb.Helper()
	dir := cmp.Or(os.Getenv("RG_MODELS_DIR"), "/models")
	m, err := open(tb.Context(), filepath.Join(dir, file))
	if err != nil {
		tb.Skipf("no %s in %s, set RG_MODELS_DIR: %v", file, dir, err)
	}
	tb.Cleanup(func() {
		if err := m.Close(); err != nil {
			tb.Errorf("closing %s: %v", file, err)
		}
	})
	return m
}
