package cache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Entry represents a cached DNS response
type Entry struct {
	Msg       *dns.Msg
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Cache is a thread-safe DNS response cache
type Cache struct {
	items      map[string]*Entry
	mu         sync.RWMutex
	maxItems   int
	defaultTTL time.Duration
	minTTL     time.Duration
	maxTTL     time.Duration
}

// New creates a new DNS cache
func New(maxItems int, defaultTTL, minTTL, maxTTL time.Duration) *Cache {
	c := &Cache{
		items:      make(map[string]*Entry),
		maxItems:   maxItems,
		defaultTTL: defaultTTL,
		minTTL:     minTTL,
		maxTTL:     maxTTL,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Key generates a cache key from a DNS question
func Key(q dns.Question) string {
	return q.Name + ":" + dns.TypeToString[q.Qtype]
}

// Get retrieves a cached DNS response
func (c *Cache) Get(key string) (*dns.Msg, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	// Return a copy of the message
	msg := entry.Msg.Copy()

	// Adjust TTLs based on elapsed time
	elapsed := uint32(time.Since(entry.CreatedAt).Seconds())
	for _, rr := range msg.Answer {
		if rr.Header().Ttl > elapsed {
			rr.Header().Ttl -= elapsed
		} else {
			rr.Header().Ttl = 1
		}
	}

	return msg, true
}

// Set stores a DNS response in the cache
func (c *Cache) Set(key string, msg *dns.Msg) {
	if msg == nil || len(msg.Question) == 0 {
		return
	}

	// Determine TTL from response
	ttl := c.defaultTTL
	if len(msg.Answer) > 0 {
		minAnswerTTL := msg.Answer[0].Header().Ttl
		for _, rr := range msg.Answer {
			if rr.Header().Ttl < minAnswerTTL {
				minAnswerTTL = rr.Header().Ttl
			}
		}
		ttl = time.Duration(minAnswerTTL) * time.Second
	}

	// Clamp TTL
	if ttl < c.minTTL {
		ttl = c.minTTL
	}
	if ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = &Entry{
		Msg:       msg.Copy(),
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

// SetNegative stores a negative (NXDOMAIN) cache entry
func (c *Cache) SetNegative(key string, msg *dns.Msg, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) >= c.maxItems {
		c.evictOldest()
	}

	c.items[key] = &Entry{
		Msg:       msg.Copy(),
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

// Len returns the number of items in the cache
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*Entry)
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.items {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.items {
			if now.After(entry.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}
