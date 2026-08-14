// Package corpustest reads the differential corpus that several packages' tests
// replay: 5,012 Hebrew words and sentences run once through the original Python
// implementation, recording every stage. Hebrew -> niqqud -> IPA -> RG is
// deterministic, so this pins the port byte-for-byte, and the fork is frozen so it
// stays valid. It lives in its own package because an external test package cannot
// be imported.
package corpustest

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

// Item is one row. IPAExpander is only present where phonikud's number-and-date
// expander would have changed the result; this port skips it, so it is recorded
// but never asserted on.
type Item struct {
	Text        string `json:"text"`
	Raw         string `json:"raw"`
	Vocalized   string `json:"vocalized"`
	IPA         string `json:"ipa"`
	RG          string `json:"rg"`
	HebRG       string `json:"heb_rg"`
	Latin       string `json:"latin"`
	IPAExpander string `json:"ipa_exp,omitempty"`
}

// Load reads the corpus at path.
func Load(tb testing.TB, path string) []Item {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("opening corpus: %v", err)
	}
	defer f.Close()

	var items []Item
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var item Item
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			tb.Fatalf("decoding corpus line %d: %v", len(items)+1, err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		tb.Fatalf("reading corpus: %v", err)
	}
	if len(items) == 0 {
		tb.Fatal("corpus is empty")
	}
	return items
}

// ModelsDir is where the checkpoints live; they are not in the repo.
func ModelsDir() string {
	if dir := os.Getenv("RG_MODELS_DIR"); dir != "" {
		return dir
	}
	return "/models"
}
