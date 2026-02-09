# Deployment Guide

This guide covers deploying both the Remote DNS API Server (on a VPS outside Iran) and the Local DNS Server (on your machine).

## Prerequisites

- Go 1.21+ installed
- A VPS outside Iran with a public IP
- Domain name (optional but recommended)
- TLS certificate (Let's Encrypt recommended)

---

## Part 1: Remote DNS API Server

### 1.1 Build the Server

```bash
# On your development machine or VPS
cd dns-proxy/remote

# Download dependencies
go mod tidy

# Build
go build -o dns-api-server ./cmd/server
```

### 1.2 Obtain TLS Certificate

Using Let's Encrypt with certbot:

```bash
# Install certbot
sudo apt install certbot

# Obtain certificate
sudo certbot certonly --standalone -d your-domain.com

# Certificates will be at:
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem
```

### 1.3 Configure

```bash
# Copy example config
cp config.example.yaml config.yaml

# Edit configuration
nano config.yaml
```

**Key settings to change:**

```yaml
server:
  port: 443
  tls_cert_file: "/etc/letsencrypt/live/your-domain.com/fullchain.pem"
  tls_key_file: "/etc/letsencrypt/live/your-domain.com/privkey.pem"

security:
  api_keys:
    - "generate-a-secure-key-here"  # Use: openssl rand -hex 32
```

### 1.4 Create Systemd Service

```bash
sudo nano /etc/systemd/system/dns-api.service
```

```ini
[Unit]
Description=DNS API Server
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/opt/dns-proxy/remote
ExecStart=/opt/dns-proxy/remote/dns-api-server -config /opt/dns-proxy/remote/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl enable dns-api
sudo systemctl start dns-api

# Check status
sudo systemctl status dns-api
```

### 1.5 Firewall Configuration

```bash
# Allow HTTPS
sudo ufw allow 443/tcp

# Optional: restrict to specific IPs
sudo ufw allow from YOUR_IP to any port 443
```

---

## Part 2: Local DNS Server

### 2.1 Build the Server

```bash
cd dns-proxy/local

# Download dependencies
go mod tidy

# Build
go build -o dns-local-server ./cmd/server
```

### 2.2 Configure

```bash
cp config.example.yaml config.yaml
nano config.yaml
```

**Key settings:**

```yaml
server:
  listen_addr: "127.0.0.1"
  port: 53

api:
  endpoints:
    - url: "https://your-domain.com/api/v1/resolve"
      api_key: "your-api-key-here"
```

### 2.3 Run the Server

**Option A: Run directly (requires root for port 53)**

```bash
sudo ./dns-local-server -config config.yaml
```

**Option B: Use higher port and redirect**

```bash
# Run on port 5353
./dns-local-server -config config.yaml  # with port: 5353 in config

# Redirect port 53 to 5353
sudo iptables -t nat -A OUTPUT -p udp --dport 53 -j REDIRECT --to-port 5353
```

### 2.4 Configure System DNS

**macOS:**

```bash
# Set DNS to localhost
sudo networksetup -setdnsservers Wi-Fi 127.0.0.1

# Verify
scutil --dns
```

**Linux:**

```bash
# Edit resolv.conf
echo "nameserver 127.0.0.1" | sudo tee /etc/resolv.conf

# Prevent overwrite
sudo chattr +i /etc/resolv.conf
```

### 2.5 Test

```bash
# Test DNS resolution
dig @127.0.0.1 google.com

# Or
nslookup google.com 127.0.0.1
```

---

## Part 3: Enable Encryption (Optional)

For additional security, enable payload encryption:

### 3.1 Generate Shared Key

```bash
openssl rand -hex 32
# Output: a1b2c3d4e5f6...  (64 characters)
```

### 3.2 Configure Both Servers

**Remote (config.yaml):**
```yaml
security:
  encryption_enabled: true
  encryption_key: "a1b2c3d4e5f6..."
```

**Local (config.yaml):**
```yaml
security:
  encryption_enabled: true
  encryption_key: "a1b2c3d4e5f6..."  # Same key!
```

---

## Troubleshooting

### Connection Refused

```bash
# Check if server is running
sudo netstat -tlnp | grep 53

# Check firewall
sudo ufw status
```

### TLS Errors

```bash
# Verify certificate
openssl s_client -connect your-domain.com:443

# Check certificate expiry
openssl x509 -enddate -noout -in /path/to/cert.pem
```

### DNS Not Working

```bash
# Test local server directly
dig @127.0.0.1 -p 53 example.com

# Check logs
journalctl -u dns-api -f  # Remote
./dns-local-server 2>&1   # Local
```

---

## Quick Start Summary

```bash
# Remote Server (VPS)
cd remote && go build -o dns-api ./cmd/server
./dns-api -config config.yaml

# Local Server (Your Machine)
cd local && go build -o dns-local ./cmd/server
sudo ./dns-local -config config.yaml

# Set system DNS
sudo networksetup -setdnsservers Wi-Fi 127.0.0.1  # macOS

# Test
dig @127.0.0.1 google.com
```
