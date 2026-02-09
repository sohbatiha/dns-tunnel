# DNS Proxy System Architecture

A secure DNS proxy system designed to bypass DNS hijacking by routing DNS queries through an encrypted HTTPS tunnel to a server outside the country.

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Inside Iran                               │
│  ┌──────────────┐     ┌─────────────────────┐                   │
│  │  Application │────▶│  Local DNS Server   │                   │
│  │  (Browser)   │     │  (port 53)          │                   │
│  └──────────────┘     └──────────┬──────────┘                   │
│                                  │                               │
│                           DNS Cache                              │
│                                  │                               │
└──────────────────────────────────┼───────────────────────────────┘
                                   │
                          HTTPS POST (Encrypted)
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────┐
│                       Outside Iran                               │
│  ┌───────────────────────────────┐     ┌──────────────────────┐ │
│  │     Remote DNS API Server     │────▶│  Upstream Resolvers  │ │
│  │     (port 443)                │     │  8.8.8.8, 1.1.1.1    │ │
│  └───────────────────────────────┘     └──────────────────────┘ │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Components

### Local DNS Server (`local/`)

Runs on your machine inside Iran and acts as the system's DNS server.

**Responsibilities:**
- Listen for DNS queries on port 53 (UDP/TCP)
- Cache DNS responses to reduce latency
- Convert DNS queries to HTTPS API calls
- Encrypt payloads (optional) for additional security
- Handle multiple API endpoints with failover

**Key Features:**
- Concurrent request handling
- LRU cache with TTL support
- Round-robin/failover load balancing
- Automatic health checks
- Retry with exponential backoff

### Remote DNS API Server (`remote/`)

Runs on a server outside Iran and performs actual DNS resolution.

**Responsibilities:**
- Receive encrypted DNS requests via HTTPS
- Resolve domains using trusted upstream servers
- Return results in JSON format
- Rate limit requests per API key
- Log requests securely

**Key Features:**
- TLS 1.2+ with strong cipher suites
- API key authentication
- Token bucket rate limiting
- Response caching
- Multiple upstream resolvers

## Data Flow

1. **Application** makes DNS query (e.g., `google.com`)
2. **OS** forwards query to Local DNS Server (127.0.0.1:53)
3. **Local DNS Server** checks cache
   - If hit: return cached response
   - If miss: continue
4. **Local DNS Server** creates API request:
   ```json
   {"domain": "google.com", "type": "A"}
   ```
5. **Optional:** Encrypt payload with AES-256-GCM
6. **Send** HTTPS POST to Remote API Server
7. **Remote API Server** validates API key
8. **Remote API Server** resolves DNS via 8.8.8.8/1.1.1.1
9. **Response** sent back:
   ```json
   {"domain": "google.com", "records": [{"type": "A", "value": "142.250.x.x", "ttl": 300}]}
   ```
10. **Local DNS Server** converts to DNS response format
11. **Local DNS Server** caches and returns to application

## Security Layers

```
┌─────────────────────────────────────────┐
│  Layer 3: DNS Query Obfuscation         │  Look like normal web traffic
├─────────────────────────────────────────┤
│  Layer 2: Payload Encryption (AES-GCM)  │  Optional, encrypts request/response
├─────────────────────────────────────────┤
│  Layer 1: Transport Encryption (TLS)    │  HTTPS with TLS 1.2+
└─────────────────────────────────────────┘
```

## Project Structure

```
dns-proxy/
├── local/                          # Local DNS Server
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── cache/cache.go          # DNS response cache
│   │   ├── client/client.go        # API client with LB
│   │   ├── config/config.go        # Configuration
│   │   ├── crypto/crypto.go        # AES encryption
│   │   └── server/server.go        # DNS server
│   ├── config.example.yaml
│   └── go.mod
│
├── remote/                         # Remote API Server
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── config/config.go        # Configuration
│   │   ├── crypto/crypto.go        # AES decryption
│   │   ├── handler/handler.go      # HTTP handlers
│   │   ├── middleware/             # Auth, rate limiting
│   │   ├── resolver/resolver.go    # DNS resolution
│   │   └── server/server.go        # HTTPS server
│   ├── config.example.yaml
│   └── go.mod
│
└── docs/                           # Documentation
    ├── ARCHITECTURE.md
    ├── DEPLOYMENT.md
    └── SECURITY.md
```

## Performance Considerations

| Component | Optimization |
|-----------|--------------|
| DNS Queries | Concurrent handling via goroutines |
| Caching | LRU cache with configurable size and TTL |
| Connection Pool | HTTP client with persistent connections |
| Response | JSON with minimal payload |

## Failover Strategy

```
Primary API ─── fail ───▶ Secondary API ─── fail ───▶ Tertiary API
     │                          │                          │
     └────── health check ──────┴────── health check ──────┘
```

- Health checks every 30 seconds
- Automatic failover on connection errors
- Round-robin distribution across healthy endpoints
