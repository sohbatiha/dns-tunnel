package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// RecordType represents DNS record types
type RecordType string

const (
	TypeA     RecordType = "A"
	TypeAAAA  RecordType = "AAAA"
	TypeCNAME RecordType = "CNAME"
	TypeMX    RecordType = "MX"
	TypeTXT   RecordType = "TXT"
	TypeNS    RecordType = "NS"
)

// DNSRecord represents a resolved DNS record
type DNSRecord struct {
	Name  string     `json:"name"`
	Type  RecordType `json:"type"`
	Value string     `json:"value"`
	TTL   uint32     `json:"ttl"`
}

// ResolveResult holds the result of a DNS resolution
type ResolveResult struct {
	Domain  string      `json:"domain"`
	Records []DNSRecord `json:"records"`
	Cached  bool        `json:"cached"`
}

// Resolver handles DNS resolution using upstream servers
type Resolver struct {
	upstreams  []string
	timeout    time.Duration
	maxRetries int
	cache      *Cache
	mu         sync.RWMutex
}

// Config holds resolver configuration
type Config struct {
	Upstreams     []string
	Timeout       time.Duration
	MaxRetries    int
	CacheEnabled  bool
	CacheTTL      time.Duration
	CacheMaxItems int
}

// New creates a new Resolver
func New(cfg Config) *Resolver {
	r := &Resolver{
		upstreams:  cfg.Upstreams,
		timeout:    cfg.Timeout,
		maxRetries: cfg.MaxRetries,
	}

	if cfg.CacheEnabled {
		r.cache = NewCache(cfg.CacheMaxItems, cfg.CacheTTL)
	}

	return r
}

// Resolve performs DNS resolution for the given domain and record type
func (r *Resolver) Resolve(ctx context.Context, domain string, recordType RecordType) (*ResolveResult, error) {
	domain = strings.TrimSuffix(domain, ".")
	cacheKey := fmt.Sprintf("%s:%s", domain, recordType)

	// Check cache
	if r.cache != nil {
		if result, ok := r.cache.Get(cacheKey); ok {
			result.Cached = true
			return result, nil
		}
	}

	// Try upstreams
	var lastErr error
	for attempt := 0; attempt < r.maxRetries; attempt++ {
		for _, upstream := range r.upstreams {
			result, err := r.resolveWithUpstream(ctx, domain, recordType, upstream)
			if err == nil {
				// Cache result
				if r.cache != nil {
					r.cache.Set(cacheKey, result)
				}
				return result, nil
			}
			lastErr = err
		}
	}

	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func (r *Resolver) resolveWithUpstream(ctx context.Context, domain string, recordType RecordType, upstream string) (*ResolveResult, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: r.timeout}
			return d.DialContext(ctx, "udp", upstream)
		},
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	result := &ResolveResult{
		Domain:  domain,
		Records: []DNSRecord{},
	}

	switch recordType {
	case TypeA:
		ips, err := resolver.LookupIP(ctx, "ip4", domain)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			result.Records = append(result.Records, DNSRecord{
				Name:  domain,
				Type:  TypeA,
				Value: ip.String(),
				TTL:   300, // Default TTL
			})
		}

	case TypeAAAA:
		ips, err := resolver.LookupIP(ctx, "ip6", domain)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			result.Records = append(result.Records, DNSRecord{
				Name:  domain,
				Type:  TypeAAAA,
				Value: ip.String(),
				TTL:   300,
			})
		}

	case TypeCNAME:
		cname, err := resolver.LookupCNAME(ctx, domain)
		if err != nil {
			return nil, err
		}
		result.Records = append(result.Records, DNSRecord{
			Name:  domain,
			Type:  TypeCNAME,
			Value: cname,
			TTL:   300,
		})

	case TypeMX:
		mxRecords, err := resolver.LookupMX(ctx, domain)
		if err != nil {
			return nil, err
		}
		for _, mx := range mxRecords {
			result.Records = append(result.Records, DNSRecord{
				Name:  domain,
				Type:  TypeMX,
				Value: fmt.Sprintf("%d %s", mx.Pref, mx.Host),
				TTL:   300,
			})
		}

	case TypeTXT:
		txtRecords, err := resolver.LookupTXT(ctx, domain)
		if err != nil {
			return nil, err
		}
		for _, txt := range txtRecords {
			result.Records = append(result.Records, DNSRecord{
				Name:  domain,
				Type:  TypeTXT,
				Value: txt,
				TTL:   300,
			})
		}

	case TypeNS:
		nsRecords, err := resolver.LookupNS(ctx, domain)
		if err != nil {
			return nil, err
		}
		for _, ns := range nsRecords {
			result.Records = append(result.Records, DNSRecord{
				Name:  domain,
				Type:  TypeNS,
				Value: ns.Host,
				TTL:   300,
			})
		}

	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}

	return result, nil
}

// Stats returns cache statistics
func (r *Resolver) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"upstreams": r.upstreams,
	}
	if r.cache != nil {
		stats["cache_size"] = r.cache.Len()
	}
	return stats
}
