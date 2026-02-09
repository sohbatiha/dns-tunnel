package resolver

import (
	"sync"
	"time"
)

// cacheEntry represents a cached DNS result
type cacheEntry struct {
	result    *ResolveResult
	expiresAt time.Time
}

// Cache is a simple TTL-based LRU cache for DNS results
type Cache struct {
	items    map[string]*cacheEntry
	mu       sync.RWMutex
	maxItems int
	ttl      time.Duration
}

// NewCache creates a new DNS cache
func NewCache(maxItems int, ttl time.Duration) *Cache {
	c := &Cache{
		items:    make(map[string]*cacheEntry),
		maxItems: maxItems,
		ttl:      ttl,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a cached result
func (c *Cache) Get(key string) (*ResolveResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	// Return a copy to avoid data races
	result := *entry.result
	records := make([]DNSRecord, len(entry.result.Records))
	copy(records, entry.result.Records)
	result.Records = records

	return &result, true
}

// Set stores a result in the cache
func (c *Cache) Set(key string, result *ResolveResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at capacity
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Len returns the number of items in the cache
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// evictOldest removes the oldest entry (must be called with lock held)
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.items {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// cleanup periodically removes expired entries
func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.items {
			if now.After(entry.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
