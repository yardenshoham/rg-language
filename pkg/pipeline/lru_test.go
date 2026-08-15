package pipeline

import (
	"strings"
	"sync"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

func byteCost(s string) int64 { return int64(len(s)) }

func TestLRUEvictsByCost(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	c.put("a", "12345")
	c.put("b", "12345")

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

// Overwriting must adjust the running total, not add to it, or the cache starves.
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

// An oversized entry must still be readable once, or a long sentence would be
// re-synthesized on every request.
func TestLRUKeepsAnOversizedEntry(t *testing.T) {
	t.Parallel()
	c := newLRU(10, byteCost)
	c.put("big", strings.Repeat("x", 100))

	if _, ok := c.get("big"); !ok {
		t.Error("an entry bigger than the budget was evicted immediately")
	}
}

func TestLRUConcurrent(t *testing.T) {
	t.Parallel()
	c := newLRU(1000, byteCost)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 200 {
				key := string(rune('a' + (i+j)%26))
				c.put(key, key)
				c.get(key)
			}
		})
	}
	wg.Wait()
}

// The cost must track what a Result retains, or the byte budget means nothing.
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
