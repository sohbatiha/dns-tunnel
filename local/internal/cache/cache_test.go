package cache

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestCache(t *testing.T) {
	cache := New(100, 5*time.Minute, time.Minute, 24*time.Hour)

	t.Run("set_get", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("test.com.", dns.TypeA)
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   "test.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: []byte{1, 2, 3, 4},
		})

		key := Key(msg.Question[0])
		cache.Set(key, msg)

		got, ok := cache.Get(key)
		if !ok {
			t.Fatal("Expected cache hit")
		}

		if len(got.Answer) != 1 {
			t.Errorf("Expected 1 answer, got %d", len(got.Answer))
		}
	})

	t.Run("miss", func(t *testing.T) {
		_, ok := cache.Get("nonexistent:A")
		if ok {
			t.Error("Expected cache miss")
		}
	})

	t.Run("ttl_adjustment", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("ttl.com.", dns.TypeA)
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   "ttl.com.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: []byte{1, 2, 3, 4},
		})

		key := Key(msg.Question[0])
		cache.Set(key, msg)

		time.Sleep(100 * time.Millisecond)

		got, ok := cache.Get(key)
		if !ok {
			t.Fatal("Expected cache hit")
		}

		// TTL should be reduced
		if got.Answer[0].Header().Ttl >= 300 {
			t.Error("TTL should be reduced after time passes")
		}
	})

	t.Run("clear", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("clear.com.", dns.TypeA)

		cache.Set("clear.com:A", msg)
		cache.Clear()

		if cache.Len() != 0 {
			t.Errorf("Expected empty cache, got %d items", cache.Len())
		}
	})
}

func TestKey(t *testing.T) {
	q := dns.Question{
		Name:  "example.com.",
		Qtype: dns.TypeA,
	}

	key := Key(q)
	if key != "example.com.:A" {
		t.Errorf("Unexpected key: %s", key)
	}
}
