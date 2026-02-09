# Security Guide

This document covers security best practices, traffic obfuscation techniques, and hardening recommendations.

## Security Overview

The DNS proxy system implements multiple layers of security:

| Layer | Protection | Implementation |
|-------|------------|----------------|
| Transport | TLS 1.2+ | HTTPS with strong cipher suites |
| Payload | AES-256-GCM | Optional encryption layer |
| Authentication | API Keys | Per-request validation |
| Rate Limiting | Token Bucket | Per-key request limits |

---

## 1. TLS Configuration

### Strong Cipher Suites

The server uses only secure cipher suites:

```go
// Enabled ciphers (in order of preference)
TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
```

### Certificate Best Practices

- Use Let's Encrypt for free, trusted certificates
- Enable auto-renewal (`certbot renew --quiet`)
- Consider using Cloudflare with strict SSL mode

---

## 2. Payload Encryption

Even with HTTPS, an additional encryption layer protects against:
- Compromised TLS termination (e.g., Cloudflare inspection)
- MITM with rogue certificates
- Log analysis by intermediaries

### How It Works

```
Client                                Server
  │                                     │
  ├─── AES-256-GCM Encrypt ────────────▶│
  │    (domain + type)                  │
  │                                     ├─── Decrypt
  │                                     │
  │◀─── AES-256-GCM Encrypt ────────────┤
  │    (response)                       │
  ├─── Decrypt                          │
```

### Key Management

```bash
# Generate a secure key
openssl rand -hex 32

# Store securely - never commit to git!
# Use environment variables or secrets manager
export DNS_ENCRYPTION_KEY="your-key-here"
```

---

## 3. Traffic Obfuscation

### Make Traffic Look Normal

The API is designed to look like normal web traffic:

| Aspect | Obfuscation |
|--------|-------------|
| Endpoint | `/api/v1/data` (generic name) |
| Headers | Standard User-Agent |
| Method | POST (common for APIs) |
| Payload | JSON (common format) |

### Advanced Obfuscation Options

**1. Domain Fronting (if needed)**

Use a CDN like Cloudflare to hide the actual backend:

```
Client → cloudflare.com → Your actual server
```

**2. Custom Headers**

Add normal-looking headers:

```go
req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; API-Client)")
req.Header.Set("Accept", "application/json")
req.Header.Set("Accept-Language", "en-US,en;q=0.9")
```

**3. Random Padding**

Add random data to requests to vary size:

```json
{
  "domain": "google.com",
  "type": "A",
  "_pad": "random-data-here"
}
```

---

## 4. API Key Security

### Generation

```bash
# Generate strong API key
openssl rand -base64 32

# Or hex format
openssl rand -hex 32
```

### Rotation

Regularly rotate API keys:

1. Generate new key
2. Add to remote config
3. Update local config
4. Remove old key

### Protection

- Never commit keys to version control
- Use environment variables
- Consider a secrets manager (Vault, AWS Secrets Manager)

---

## 5. Rate Limiting

Protects against:
- Brute force attacks
- Denial of Service
- API abuse

### Configuration

```yaml
security:
  rate_limit_enabled: true
  rate_limit_per_sec: 100   # Requests per second
  rate_limit_burst: 200     # Max burst size
```

### Response to Rate Limit

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1
{"error": "rate_limit_exceeded"}
```

---

## 6. Logging Security

### Safe Logging Practices

**Do:**
- Log request counts and timing
- Log errors without sensitive data
- Use structured logging (JSON)

**Don't:**
- Log domain names in production
- Log API keys
- Log full request/response bodies

### Configuration

```yaml
logging:
  level: "info"          # Use "warn" in production
  format: "json"
  output_file: "/var/log/dns-api/access.log"
```

---

## 7. Network Hardening

### Firewall Rules

```bash
# Only allow HTTPS
sudo ufw default deny incoming
sudo ufw allow 443/tcp

# Optional: restrict to known IPs
sudo ufw allow from YOUR_IP to any port 443
```

### Fail2Ban (Optional)

```ini
# /etc/fail2ban/jail.local
[dns-api]
enabled = true
port = 443
filter = dns-api
logpath = /var/log/dns-api/access.log
maxretry = 10
bantime = 3600
```

---

## 8. Server Hardening

### System Updates

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install unattended-upgrades
```

### Run as Non-Root

```yaml
# systemd service
[Service]
User=nobody
Group=nogroup
```

### File Permissions

```bash
chmod 600 config.yaml
chmod 700 /opt/dns-proxy
```

---

## 9. Monitoring

### Health Checks

```bash
# Check server health
curl https://your-server.com/health

# Expected response
{"status": "ok", "time": "...", "stats": {...}}
```

### Metrics to Monitor

- Request count per minute
- Error rate
- Response latency
- Cache hit rate
- Endpoint health status

---

## 10. Incident Response

### If API Key Compromised

1. Immediately rotate key on remote server
2. Update local client with new key
3. Review logs for unauthorized access
4. Consider IP restrictions

### If Server Compromised

1. Take server offline
2. Rotate all credentials
3. Review access logs
4. Re-deploy from clean state
5. Enable additional IP restrictions

---

## Security Checklist

- [ ] TLS certificate from trusted CA
- [ ] Strong API keys generated
- [ ] API keys not in version control
- [ ] Rate limiting enabled
- [ ] Logging configured (not logging sensitive data)
- [ ] Firewall configured
- [ ] Server running as non-root
- [ ] Encryption enabled (optional)
- [ ] Regular key rotation schedule
- [ ] Monitoring setup
