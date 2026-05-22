# Ubertool Backend — TLS & Domain Handoff Notes
# Status: COMPLETE — TLS live on api.marigoldshare.marigoldintelligence.us:50052 as of April 14, 2026

## Project Context

- **Backend:** Go gRPC microservice (`ubertool-backend-trusted`)
- **Project root (local):** `C:\Users\yixio\ubertool\ubertool-backend-trusted`
- **Android app root:** `H:\github\projects\ubertool\ubertool-android-trusted`
- **Future iOS app:** Planned, identical features to Android

---

## Infrastructure State

| Component | Details |
|---|---|
| EC2 instance | Ubuntu 24.04 (t3.micro), us-west-2 |
| EC2 user | `ubuntu` |
| SSH alias | `ubertool-ec2` |
| SSH key | `C:\Users\yixio\.ssh\ixorashare-ec2-key.pem` |
| Elastic IP | Attached and running (no charge while attached) |
| Domain | `marigoldintelligence.us` |
| API subdomain | `api.marigoldshare.marigoldintelligence.us` → A record → Elastic IP |
| RDS instance | `ubertool-db-mvp.cx66cg08ai98.us-west-2.rds.amazonaws.com` |
| RDS database | `ubertool_db_prod` |
| RDS user | `ubertool_db_prod` |
| gRPC port | `50052` |

---

## TLS Certificate State

| Item | Details |
|---|---|
| Provider | Let's Encrypt (via Certbot 2.9.0) |
| Certificate | `/etc/letsencrypt/live/api.marigoldshare.marigoldintelligence.us/fullchain.pem` |
| Private key | `/etc/letsencrypt/live/api.marigoldshare.marigoldintelligence.us/privkey.pem` |
| App cert copy | `/var/ubertool/certs/fullchain.pem` |
| App key copy | `/var/ubertool/certs/privkey.pem` |
| Expiry | 2026-07-13 (auto-renews via Certbot scheduled task) |
| Cert email | `yixiong.us@gmail.com` |

### Certificate File Permissions
```
-rw-r--r-- 1 root root     fullchain.pem   (readable by all)
-rw-r----- 1 root ubertool privkey.pem     (readable by root and ubertool only)
```

---

## AWS Security Group Rules

| Type | Protocol | Port | Source |
|---|---|---|---|
| SSH | TCP | 22 | Your IP only (`x.x.x.x/32`) |
| Custom TCP (gRPC) | TCP | 50052 | `0.0.0.0/0` |
| HTTPS | TCP | 443 | `0.0.0.0/0` |
| HTTP | TCP | 80 | `0.0.0.0/0` (required for Certbot renewal) |

> **Note:** SSH source should be locked to your specific IPv4 address with `/32`.
> Update it in the AWS Console if your home IP changes.

---

## Remaining Tasks

### 1. Add TLS Config Struct
In `internal/config/config.go`, add:

```go
type TLSConfig struct {
    Enabled  bool   `yaml:"enabled"`
    CertFile string `yaml:"cert_file"`
    KeyFile  string `yaml:"key_file"`
}

// Add to your existing Config struct:
type Config struct {
    // ... existing fields
    TLS TLSConfig `yaml:"tls"`
}
```

---

### 2. Update config_prod.yaml
Add the `tls` block after the `server` block:

```yaml
server:
  host: "0.0.0.0"
  port: 50052

tls:
  enabled: true
  cert_file: "/var/ubertool/certs/fullchain.pem"
  key_file: "/var/ubertool/certs/privkey.pem"

database:
  host: "ubertool-db-mvp.cx66cg08ai98.us-west-2.rds.amazonaws.com"
  port: 5432
  user: "ubertool_db_prod"
  password: "7ZKgLfxDMZsCJaC"
  database: "ubertool_db_prod"
  ssl_mode: "require"
# ... rest of config unchanged
```

---

### 3. Update main.go — Enable TLS on gRPC Server
Replace the plain gRPC server creation with TLS-enabled version:

```go
import (
    "google.golang.org/grpc/credentials"
    // ... existing imports
)

// Replace this:
// s := grpc.NewServer(
//     grpc.UnaryInterceptor(authInterceptor.Unary()),
// )

// With this:
creds, err := credentials.NewServerTLSFromFile(
    cfg.TLS.CertFile,
    cfg.TLS.KeyFile,
)
if err != nil {
    logger.Error("Failed to load TLS credentials", "error", err)
    log.Fatalf("Failed to load TLS credentials: %v", err)
}

s := grpc.NewServer(
    grpc.Creds(creds),
    grpc.UnaryInterceptor(authInterceptor.Unary()),
)
```

---

### 4. Build and Deploy Binary to EC2

On your Windows machine, cross-compile for Linux:

```powershell
# In your project root
cd C:\Users\yixio\ubertool\ubertool-backend-trusted

$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o ubertool-server ./cmd/main.go
```

> **Note:** Adjust `./cmd/main.go` to match your actual main entry point path.

Then SCP the binary and config to EC2:

```powershell
# Copy binary
scp ubertool-server ubertool-ec2:/home/ubuntu/

# Copy production config
scp deploy/ec2-mvp/config.yaml ubertool-ec2:/home/ubuntu/
```

On EC2, move files to the app directory:

```bash
sudo mv /home/ubuntu/ubertool-server /var/ubertool/
sudo mv /home/ubuntu/config.yaml /var/ubertool/
sudo chown ubertool:ubertool /var/ubertool/ubertool-server
sudo chmod +x /var/ubertool/ubertool-server
```

---

### 5. Create systemd Service

On EC2, create `/etc/systemd/system/ubertool.service`:

```ini
[Unit]
Description=Ubertool gRPC Backend
After=network.target

[Service]
Type=simple
User=ubertool
WorkingDirectory=/var/ubertool
ExecStart=/var/ubertool/ubertool-server -config /var/ubertool/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ubertool
sudo systemctl start ubertool
sudo systemctl status ubertool
```

---

### 6. Hook Certbot Renewal to Restart Service

Certbot already has a scheduled renewal task. Add a deploy hook so the
service restarts and loads the new cert after each renewal:

```bash
sudo nano /etc/letsencrypt/renewal-hooks/deploy/restart-ubertool.sh
```

Add this content:

```bash
#!/bin/bash
cp /etc/letsencrypt/live/api.marigoldshare.marigoldintelligence.us/fullchain.pem /var/ubertool/certs/
cp /etc/letsencrypt/live/api.marigoldshare.marigoldintelligence.us/privkey.pem /var/ubertool/certs/
chown root:ubertool /var/ubertool/certs/privkey.pem
chmod 640 /var/ubertool/certs/privkey.pem
systemctl restart ubertool
```

Make it executable:

```bash
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/restart-ubertool.sh
```

---

### 7. Test TLS Connection

```bash
# Install grpcurl on EC2
curl -L https://github.com/fullstorydev/grpcurl/releases/download/v1.8.7/grpcurl_1.8.7_linux_x86_64.tar.gz | tar -xz
sudo mv grpcurl /usr/local/bin/

# Test TLS connection
grpcurl api.marigoldshare.marigoldintelligence.us:50052 list
```

---

### 8. Android Kotlin Client Update

Since Let's Encrypt is trusted by all modern Android and iOS devices,
no certificate bundling is needed. Just update your gRPC channel:

```kotlin
// In your Android gRPC client setup
val channel = ManagedChannelBuilder
    .forAddress("api.marigoldshare.marigoldintelligence.us", 50052)
    .useTransportSecurity()  // enables TLS
    .build()
```

No changes needed for the future iOS client beyond the equivalent
`useTransportSecurity()` call in Swift.

---

## Notes

- The `ubertool` Linux user already exists and owns `/var/ubertool`
- RDS connection is unaffected by any of these changes
- Let's Encrypt certs auto-renew every 90 days — no manual action needed
  as long as port 80 remains open in the security group
- Do NOT close port 80 — Certbot needs it for domain ownership verification
  during renewal
