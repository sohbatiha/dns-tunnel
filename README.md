# DNS Proxy System

A secure DNS proxy system to bypass DNS hijacking by routing DNS queries through an encrypted HTTPS tunnel.

```
┌──────────────────┐        HTTPS         ┌──────────────────┐
│  Local DNS       │ ──────────────────▶  │  Remote API      │
│  (Your Machine)  │                      │  (VPS Abroad)    │
│  port 53         │  ◀──────────────────  │  port 443        │
└──────────────────┘                      └──────────────────┘
         ▲                                         │
         │                                         ▼
    DNS Query                              8.8.8.8, 1.1.1.1
```

## Features

- **Bypass DNS Hijacking**: Route DNS through encrypted tunnel
- **High Performance**: Local caching, connection pooling
- **Reliable**: Failover, health checks, retry logic
- **Secure**: TLS 1.2+, optional payload encryption
- **Scalable**: Multiple API endpoints, load balancing

## Project Structure

```
dns-proxy/
├── local/          # Local DNS Server (runs on your machine)
├── remote/         # Remote API Server (runs on VPS)
└── docs/           # Documentation
```

## Quick Start

### 1. Deploy Remote Server (VPS)

```bash
cd remote
go build -o dns-api ./cmd/server
./dns-api -config config.yaml
```

### 2. Run Local Server

```bash
cd local
go build -o dns-local ./cmd/server
sudo ./dns-local -config config.yaml
```

### 3. Configure System DNS

```bash
# macOS
sudo networksetup -setdnsservers Wi-Fi 127.0.0.1

# Linux
echo "nameserver 127.0.0.1" | sudo tee /etc/resolv.conf
```

### 4. Test

```bash
dig @127.0.0.1 google.com
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) - System design and data flow
- [Deployment](docs/DEPLOYMENT.md) - Step-by-step deployment guide
- [Security](docs/SECURITY.md) - Security best practices

## Requirements

- Go 1.21+
- VPS outside Iran (for remote server)
- TLS certificate (Let's Encrypt)

## License

MIT
