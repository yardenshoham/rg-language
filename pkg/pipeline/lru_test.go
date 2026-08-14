package pipeline

import (
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

func byteCost(s string) int64 { return int64(len(s)) }

func TestLRUEvictsByCost(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	c.put("a", "12345")
	c.put("b", "12345")

	// Both fit exactly.
	if _, ok := c.get("a"); !ok {
		t.Error("a was evicted while it still fit")
	}

	// One more pushes the least recently used out. "a" was just read, so "b" goes.
	c.put("c", "12345")
	if _, ok := c.get("b"); ok {
		t.Error("b should have been evicted as the least recently used")
	}
	for _, key := range []string{"a", "c"} {
		if _, ok := c.get(key); !ok {
			t.Errorf("%s should have survived", key)
		}
	}
}

// Overwriting a key has to adjust the running total, not add to it, or the cache
// slowly starves itself.
func TestLRUOverwriteAdjustsCost(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	for range 20 {
		c.put("a", "1234567890")
	}
	c.put("b", "1")

	if _, ok := c.get("b"); !ok {
		t.Error("b was evicted immediately, so the cost of a was counted more than once")
	}
	if c.cost > 11 {
		t.Errorf("running cost drifted to %d after repeated overwrites", c.cost)
	}
}

// An entry larger than the whole budget still has to be readable once, or a long
// sentence would be uncacheable and re-synthesized on every request.
func TestLRUKeepsAnOversizedEntry(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	c.put("big", strings.Repeat("x", 100))

	if _, ok := c.get("big"); !ok {
		t.Error("an entry bigger than the budget was evicted immediately")
	}
}

func TestLRUMissing(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	if got, ok := c.get("nope"); ok || got != "" {
		t.Errorf("get on an empty cache = (%q, %v), want the zero value", got, ok)
	}
}

func TestLRUConcurrent(t *testing.T) {
	t.Parallel()
	c := newLRU(1000, byteCost)
	done := make(chan struct{})
	for i := range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range 200 {
				key := string(rune('a' + (i+j)%26))
				c.put(key, key)
				c.get(key)
			}
		}()
	}
	for range 8 {
		<-done
	}
}

// The cost function has to count what a Result actually retains, or the byte
// budget means nothing.
func TestResultCostFollowsSize(t *testing.T) {
	t.Parallel()
	small := Result{Input: "שלום", IPA: "ʃalˈom"}
	large := Result{
		Input:  strings.Repeat("שלום ", 100),
		IPA:    strings.Repeat("ʃalˈom ", 100),
		Hebrew: make([]rg.Segment, 200),
	}
	if resultCost(large) <= resultCost(small)*10 {
		t.Errorf("a much larger Result costs %d against %d, which is too close",
			resultCost(large), resultCost(small))
	}
}
