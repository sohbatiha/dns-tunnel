package resolver

import (
	"context"
	"testing"
	"time"
)

func TestResolver(t *testing.T) {
	cfg := Config{
		Upstreams:     []string{"8.8.8.8:53", "1.1.1.1:53"},
		Timeout:       5 * time.Second,
		MaxRetries:    2,
		CacheEnabled:  true,
		CacheTTL:      5 * time.Minute,
		CacheMaxItems: 100,
	}

	resolver := New(cfg)

	t.Run("resolve_a_record", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := resolver.Resolve(ctx, "google.com", TypeA)
		if err != nil {
			t.Skipf("Network test skipped: %v", err)
		}

		if result.Domain != "google.com" {
			t.Errorf("Expected domain google.com, got %s", result.Domain)
		}

		if len(result.Records) == 0 {
			t.Error("Expected at least one record")
		}

		for _, rec := range result.Records {
			if rec.Type != TypeA {
				t.Errorf("Expected A record, got %s", rec.Type)
			}
		}
	})

	t.Run("cache_hit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// First request
		_, err := resolver.Resolve(ctx, "example.com", TypeA)
		if err != nil {
			t.Skipf("Network test skipped: %v", err)
		}

		// Second request should be cached
		result, err := resolver.Resolve(ctx, "example.com", TypeA)
		if err != nil {
			t.Fatalf("Second resolve failed: %v", err)
		}

		if !result.Cached {
			t.Error("Expected cache hit")
		}
	})
}

func TestCache(t *testing.T) {
	cache := NewCache(10, time.Minute)

	t.Run("set_get", func(t *testing.T) {
		result := &ResolveResult{
			Domain: "test.com",
			Records: []DNSRecord{
				{Name: "test.com", Type: TypeA, Value: "1.2.3.4", TTL: 300},
			},
		}

		cache.Set("test.com:A", result)

		got, ok := cache.Get("test.com:A")
		if !ok {
			t.Fatal("Expected cache hit")
		}

		if got.Domain != result.Domain {
			t.Errorf("Domain mismatch: got %s, want %s", got.Domain, result.Domain)
		}
	})

	t.Run("miss", func(t *testing.T) {
		_, ok := cache.Get("nonexistent:A")
		if ok {
			t.Error("Expected cache miss")
		}
	})

	t.Run("expiry", func(t *testing.T) {
		shortCache := NewCache(10, time.Millisecond)

		result := &ResolveResult{Domain: "expire.com"}
		shortCache.Set("expire.com:A", result)

		time.Sleep(10 * time.Millisecond)

		_, ok := shortCache.Get("expire.com:A")
		if ok {
			t.Error("Expected cache miss after expiry")
		}
	})
}
