# Setting Up Caddy as a Reverse Proxy for gRPC and HTTP/REST Services

## Overview

This guide covers setting up Caddy as a TLS-terminating reverse proxy in front of two Go microservices: a gRPC service and an HTTP/REST image service. Caddy automatically provisions and renews TLS certificates from Let's Encrypt, requiring minimal configuration.

### Architecture

```
Android App
    │  TLS on port 443
    │  ├── gRPC requests (JWT in metadata)     → localhost:50051
    │  └── HTTP/REST /images/* requests        → localhost:50053
    ▼
Caddy (reverse proxy)  ←── Auto-managed Let's Encrypt cert
    ├── h2c://localhost:50051  (gRPC service)
    └── localhost:50053        (Image HTTP/REST service)
```

---

## Prerequisites

- A Linux server (Ubuntu/Debian recommended)
- A **public domain name** with an **A record** pointing to your server's IP
- Port **80** and **443** open in your firewall
- Your Go gRPC service running on `localhost:50051`
- Your Go image HTTP/REST service running on `localhost:50053`

> **Note:** Let's Encrypt requires a valid public domain. It cannot issue certs for bare IP addresses. If you don't have a domain, free options like [DuckDNS](https://www.duckdns.org) work fine.

---

## Step 1: Install Caddy

```bash
# Ubuntu / Debian
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl

curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
  | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg

curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
  | sudo tee /etc/apt/sources.list.d/caddy-stable.list

sudo apt update && sudo apt install caddy
```

Verify installation:

```bash
caddy version
```

---

## Step 2: Configure the Caddyfile

Edit the default Caddyfile:

```bash
sudo nano /etc/caddy/Caddyfile
```

Replace its contents with:

```
your.domain.com {
    # gRPC service — matched by Content-Type header
    @grpc {
        header Content-Type application/grpc*
    }
    reverse_proxy @grpc h2c://localhost:50051

    # Image HTTP/REST service — matched by path prefix
    @images {
        path /images/*
    }
    reverse_proxy @images localhost:50053
}
```

- `your.domain.com` — replace with your actual domain
- `@grpc` matcher — catches all gRPC requests by their `Content-Type: application/grpc` header
- `h2c://` — forwards gRPC traffic as cleartext HTTP/2 (the gRPC wire protocol)
- `@images` matcher — routes all `/images/*` paths to the image service
- `localhost:50053` — your image HTTP/REST service address

Caddy evaluates matchers in order, so gRPC requests are caught first, then `/images/*` paths. Caddy will automatically:
- Obtain a TLS certificate from Let's Encrypt on first request
- Renew it before expiry
- Redirect HTTP → HTTPS

---

## Step 3: Start and Enable Caddy

```bash
# Reload config if Caddy is already running
sudo systemctl reload caddy

# Or start it fresh
sudo systemctl start caddy
sudo systemctl enable caddy   # auto-start on reboot

# Check status
sudo systemctl status caddy
```

View live logs to confirm cert issuance:

```bash
sudo journalctl -u caddy -f
```

You should see a line like:
```
certificate obtained successfully  domain=your.domain.com
```

---

## Step 4: Update Your Go Services

Neither service needs TLS configured — Caddy handles that. Ensure both are listening on localhost only.

**gRPC Service:**

```go
listener, err := net.Listen("tcp", "127.0.0.1:50051")
if err != nil {
    log.Fatalf("failed to listen: %v", err)
}

// No TLS credentials needed here — Caddy terminates TLS
server := grpc.NewServer(
    grpc.UnaryInterceptor(authInterceptor), // your existing JWT interceptor
)
```

**Image HTTP/REST Service:**

```go
// Bind to localhost only — Caddy handles TLS termination
http.ListenAndServe("127.0.0.1:50053", yourRouter)
```

> **Security tip:** Binding to `127.0.0.1` instead of `0.0.0.0` ensures neither service port is ever directly reachable from the internet — only Caddy can reach them.

---

## Step 5: Update Your Android Client

With a CA-signed Let's Encrypt cert, Android trusts it natively — no cert bundling needed.

**gRPC channel (unchanged):**

```kotlin
val channel = ManagedChannelBuilder
    .forAddress("your.domain.com", 443)
    .build() // TLS enabled by default on port 443

val stub = YourServiceGrpc.newBlockingStub(channel)
```

**Image upload/download via HTTP/REST:**

```kotlin
// Upload
val request = Request.Builder()
    .url("https://your.domain.com/images/upload")
    .post(imageBody)
    .build()

// Download
val request = Request.Builder()
    .url("https://your.domain.com/images/filename.jpg")
    .get()
    .build()
```

Your existing JWT metadata attachment on the gRPC channel stays unchanged.

---

## Firewall Configuration

Ensure ports 80 and 443 are open (port 80 is needed for the ACME HTTP challenge):

```bash
# UFW (Ubuntu)
sudo ufw allow 80
sudo ufw allow 443
sudo ufw reload

# iptables
sudo iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```

Close direct access to both service ports from outside:

```bash
sudo ufw deny 50051
sudo ufw deny 50053
```

---

## Verifying the Setup

**Test gRPC** using `grpcurl`:

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List services
grpcurl your.domain.com:443 list

# Call a method
grpcurl -d '{"key": "value"}' your.domain.com:443 your.package.YourService/YourMethod
```

**Test the image service** using `curl`:

```bash
# Upload an image
curl -X POST https://your.domain.com/images/upload \
  -H "Authorization: Bearer <your_jwt>" \
  -F "file=@/path/to/image.jpg"

# Download an image
curl -O https://your.domain.com/images/filename.jpg
```

---

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| Cert not issuing | Port 80 blocked | Open port 80 in firewall |
| Cert not issuing | DNS not propagated | Wait for DNS TTL, verify with `dig your.domain.com` |
| Android connection refused | Wrong port | Ensure client uses port 443 |
| `transport: not an HTTP/2 frame` | Proxy not forwarding as h2c | Confirm `h2c://` prefix in Caddyfile for gRPC |
| 502 Bad Gateway on gRPC | gRPC service not running | Check `systemctl status` of your Go gRPC service |
| 502 Bad Gateway on /images/* | Image service not running | Check `systemctl status` of your Go image service |
| Images routing to gRPC service | Missing `@images` matcher | Confirm `path /images/*` block is in Caddyfile |

---

## Caddyfile Reference (Advanced Options)

```
your.domain.com {
    # gRPC service
    @grpc {
        header Content-Type application/grpc*
    }
    reverse_proxy @grpc h2c://localhost:50051

    # Image HTTP/REST service
    @images {
        path /images/*
    }
    reverse_proxy @images localhost:50053

    # Optional: custom TLS settings
    tls {
        protocols tls1.2 tls1.3
    }

    # Optional: access logging
    log {
        output file /var/log/caddy/access.log
    }
}
```

---

## Summary

| Step | Action |
|---|---|
| 1 | Install Caddy via apt |
| 2 | Write Caddyfile with `@grpc` and `@images` matchers for path-based routing |
| 3 | Start and enable the Caddy service |
| 4 | Bind gRPC service to `127.0.0.1:50051` and image service to `127.0.0.1:50053` (no TLS needed on either) |
| 5 | Update Android gRPC channel to port 443; use `https://your.domain.com/images/*` for image calls |

Once complete, all traffic between your Android app and both backend services is encrypted with a trusted TLS certificate, with zero manual cert management required.
