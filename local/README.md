# Local DNS Server

A local DNS server that runs on your machine and forwards DNS queries to a remote API server via HTTPS.

## Features

- 🌐 UDP/TCP DNS server on port 53
- ⚡ High-performance DNS caching
- 🔄 Multiple API endpoint support
- ⚖️ Load balancing (round-robin/failover)
- 🏥 Automatic health checks
- 🔁 Retry with exponential backoff
- 🔐 Optional payload encryption (AES-256-GCM)

## Quick Start

```bash
# Build
go build -o dns-local-server ./cmd/server

# Configure
cp config.example.yaml config.yaml
# Edit config.yaml with your API endpoint

# Run (requires root for port 53)
sudo ./dns-local-server -config config.yaml
```

## Configuration

See `config.example.yaml` for all options.

Key settings:

| Setting | Description |
|---------|-------------|
| `server.port` | DNS port (default: 53) |
| `server.protocol` | udp, tcp, or both |
| `api.endpoints` | List of remote API servers |
| `api.load_balancing` | round_robin or failover |
| `cache.enabled` | Enable DNS caching |

### Multiple Endpoints (Failover)

```yaml
api:
  endpoints:
    - url: "https://primary.example.com/api/v1/resolve"
      api_key: "key1"
    - url: "https://backup.example.com/api/v1/resolve"
      api_key: "key2"
  load_balancing: "failover"
```

## System DNS Setup

### macOS

```bash
# Set DNS to localhost
sudo networksetup -setdnsservers Wi-Fi 127.0.0.1

# Revert to automatic
sudo networksetup -setdnsservers Wi-Fi Empty
```

### Linux

```bash
echo "nameserver 127.0.0.1" | sudo tee /etc/resolv.conf
```

## Testing

```bash
# Test DNS resolution
dig @127.0.0.1 google.com

# Or
nslookup google.com 127.0.0.1
```

## Deployment

See [DEPLOYMENT.md](../docs/DEPLOYMENT.md) for full deployment guide.

## Security

See [SECURITY.md](../docs/SECURITY.md) for security best practices.
