package pipeline

import (
	"container/list"
	"sync"
)

// lru is a bounded, concurrency-safe cache keyed by string. Entries carry a cost,
// so one type can bound both the small maps and the audio cache.
type lru[V any] struct {
	mu      sync.Mutex
	limit   int64
	cost    int64
	costOf  func(V) int64
	order   list.List // most recently used at the front
	entries map[string]*list.Element
}

type entry[V any] struct {
	key   string
	value V
	cost  int64
}

func newLRU[V any](limit int64, costOf func(V) int64) *lru[V] {
	return &lru[V]{limit: limit, costOf: costOf, entries: map[string]*list.Element{}}
}

func (c *lru[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		c.order.MoveToFront(element)
		return element.Value.(*entry[V]).value, true
	}
	var zero V
	return zero, false
}

func (c *lru[V]) put(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[key]; ok {
		c.cost -= c.order.Remove(element).(*entry[V]).cost
	}
	cost := c.costOf(value)
	c.entries[key] = c.order.PushFront(&entry[V]{key: key, value: value, cost: cost})
	c.cost += cost

	// Never evict the entry just added, however big it is.
	for c.cost > c.limit && c.order.Len() > 1 {
		evicted := c.order.Remove(c.order.Back()).(*entry[V])
		delete(c.entries, evicted.key)
		c.cost -= evicted.cost
	}
}
