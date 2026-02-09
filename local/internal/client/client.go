package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mahdi/dns-proxy-local/internal/config"
	"github.com/mahdi/dns-proxy-local/internal/crypto"
)

// DNSRecord represents a resolved DNS record
type DNSRecord struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl"`
}

// ResolveResponse represents the API response
type ResolveResponse struct {
	Domain  string      `json:"domain"`
	Records []DNSRecord `json:"records"`
	Cached  bool        `json:"cached"`
	Error   string      `json:"error,omitempty"`
}

// EncryptedRequest represents an encrypted request payload
type EncryptedRequest struct {
	Data string `json:"data"`
}

// Endpoint represents a single API endpoint with health status
type Endpoint struct {
	URL     string
	APIKey  string
	Weight  int
	Healthy atomic.Bool
}

// Client handles communication with remote DNS API servers
type Client struct {
	endpoints     []*Endpoint
	httpClient    *http.Client
	cipher        *crypto.Cipher
	timeout       time.Duration
	maxRetries    int
	retryDelay    time.Duration
	loadBalancing string
	currentIndex  atomic.Uint32
	mu            sync.RWMutex
}

// NewClient creates a new API client
func NewClient(cfg config.APIConfig, cipher *crypto.Cipher) *Client {
	endpoints := make([]*Endpoint, len(cfg.Endpoints))
	for i, ep := range cfg.Endpoints {
		endpoints[i] = &Endpoint{
			URL:    ep.URL,
			APIKey: ep.APIKey,
			Weight: ep.Weight,
		}
		endpoints[i].Healthy.Store(true)
	}

	client := &Client{
		endpoints: endpoints,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
		cipher:        cipher,
		timeout:       cfg.Timeout,
		maxRetries:    cfg.MaxRetries,
		retryDelay:    cfg.RetryDelay,
		loadBalancing: cfg.LoadBalancing,
	}

	// Start health check
	go client.healthCheck(cfg.HealthCheckFreq)

	return client
}

// Resolve sends a DNS resolution request to the remote API
func (c *Client) Resolve(ctx context.Context, domain string, recordType string) (*ResolveResponse, error) {
	// Build request body
	reqBody := map[string]string{
		"domain": domain,
		"type":   recordType,
	}

	var body []byte

	if c.cipher != nil {
		// Encrypt the request
		jsonData, _ := json.Marshal(reqBody)
		encrypted, err := c.cipher.Encrypt(jsonData)
		if err != nil {
			return nil, fmt.Errorf("encryption failed: %w", err)
		}
		body, _ = json.Marshal(EncryptedRequest{Data: encrypted})
	} else {
		body, _ = json.Marshal(reqBody)
	}

	// Try endpoints with retry logic
	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		endpoint := c.selectEndpoint()
		if endpoint == nil {
			return nil, fmt.Errorf("no healthy endpoints available")
		}

		resp, err := c.doRequest(ctx, endpoint, body)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		endpoint.Healthy.Store(false)

		// Wait before retry
		if attempt < c.maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay * time.Duration(attempt+1)):
			}
		}
	}

	return nil, fmt.Errorf("all attempts failed: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, endpoint *Endpoint, body []byte) (*ResolveResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", endpoint.APIKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DNS-Client/1.0)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result ResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) selectEndpoint() *Endpoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch c.loadBalancing {
	case "round_robin":
		return c.selectRoundRobin()
	case "failover":
		return c.selectFailover()
	default:
		return c.selectRoundRobin()
	}
}

func (c *Client) selectRoundRobin() *Endpoint {
	for i := 0; i < len(c.endpoints); i++ {
		idx := int(c.currentIndex.Add(1)-1) % len(c.endpoints)
		if c.endpoints[idx].Healthy.Load() {
			return c.endpoints[idx]
		}
	}
	// All unhealthy, try first one anyway
	if len(c.endpoints) > 0 {
		return c.endpoints[0]
	}
	return nil
}

func (c *Client) selectFailover() *Endpoint {
	for _, ep := range c.endpoints {
		if ep.Healthy.Load() {
			return ep
		}
	}
	// All unhealthy, try first one anyway
	if len(c.endpoints) > 0 {
		return c.endpoints[0]
	}
	return nil
}

func (c *Client) healthCheck(freq time.Duration) {
	ticker := time.NewTicker(freq)
	for range ticker.C {
		for _, ep := range c.endpoints {
			go c.checkEndpoint(ep)
		}
	}
}

func (c *Client) checkEndpoint(ep *Endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Parse base URL to get health endpoint
	healthURL := ep.URL[:len(ep.URL)-len("/api/v1/resolve")] + "/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		ep.Healthy.Store(false)
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		ep.Healthy.Store(false)
		return
	}
	defer resp.Body.Close()

	ep.Healthy.Store(resp.StatusCode == http.StatusOK)
}

// Stats returns client statistics
func (c *Client) Stats() map[string]interface{} {
	healthy := 0
	for _, ep := range c.endpoints {
		if ep.Healthy.Load() {
			healthy++
		}
	}
	return map[string]interface{}{
		"endpoints_total":   len(c.endpoints),
		"endpoints_healthy": healthy,
		"load_balancing":    c.loadBalancing,
	}
}
