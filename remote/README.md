# Remote DNS API Server

A secure DNS resolution API server designed to run outside Iran, providing DNS resolution as a service over HTTPS.

## Features

- 🔒 HTTPS with TLS 1.2+ and strong cipher suites
- 🔑 API key authentication
- ⚡ Rate limiting (token bucket)
- 📦 Response caching
- 🌐 Multiple upstream resolvers (8.8.8.8, 1.1.1.1)
- 🔐 Optional payload encryption (AES-256-GCM)
- 📊 Health monitoring endpoint

## Quick Start

```bash
# Build
go build -o dns-api-server ./cmd/server

# Configure
cp config.example.yaml config.yaml
# Edit config.yaml with your settings

# Run
./dns-api-server -config config.yaml
```

## API Endpoints

### POST /api/v1/resolve

Resolve a domain name.

**Request:**
```json
{
  "domain": "google.com",
  "type": "A"
}
```

**Response:**
```json
{
  "domain": "google.com",
  "records": [
    {"name": "google.com", "type": "A", "value": "142.250.185.78", "ttl": 300}
  ],
  "cached": false
}
```

**Headers:**
- `X-API-Key`: Your API key (required)
- `Content-Type`: application/json

### GET /health

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "time": "2024-01-01T12:00:00Z",
  "stats": {"upstreams": ["8.8.8.8:53"], "cache_size": 42}
}
```

## Configuration

See `config.example.yaml` for all options.

Key settings:

| Setting | Description |
|---------|-------------|
| `server.port` | HTTPS port (default: 8443) |
| `server.tls_cert_file` | Path to TLS certificate |
| `server.tls_key_file` | Path to TLS private key |
| `security.api_keys` | List of valid API keys |
| `security.encryption_enabled` | Enable payload encryption |

## Deployment

See [DEPLOYMENT.md](../docs/DEPLOYMENT.md) for full deployment guide.

```bash
# Systemd service
sudo systemctl enable dns-api
sudo systemctl start dns-api
```

## Security

See [SECURITY.md](../docs/SECURITY.md) for security best practices.
