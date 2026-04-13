# Ubertool Backend — EC2 MVP Deployment

Complete deployment for the Ubertool gRPC API microservice on AWS EC2 + RDS PostgreSQL, with Let's Encrypt TLS for secure Android (and future iOS) connectivity.

---

## Architecture

```
Android App (Kotlin)                   iOS App (Swift, future)
      │  gRPC + TLS (Let's Encrypt)          │
      └──────────────┬───────────────────────┘
                     ▼
            EC2 t3.micro (Ubuntu 24.04)
            ┌─────────────────────────┐
            │  ubertool-api (Go/gRPC) │  ← systemd service
            │  Port 443 (TLS)         │  ← Let's Encrypt cert
            │  Elastic IP + Domain    │
            └─────────┬───────────────┘
                       │ private VPC (port 5432)
                       ▼
            RDS db.t3.micro (PostgreSQL 15)
```

---

## Prerequisites

### 1. Install AWS CLI v2
```bash
# macOS
brew install awscli

# Windows (PowerShell)
winget install Amazon.AWSCLI

# Verify
aws --version
```

### 2. Configure AWS credentials
```bash
aws configure
# AWS Access Key ID:     <from IAM Console>
# AWS Secret Access Key: <from IAM Console>
# Default region:        us-east-1
# Default output format: json
```

### 3. Create an EC2 Key Pair 
- already created ixorashare-ec2-key.pem; no need to have another
```bash
aws ec2 create-key-pair \
  --key-name ubertool-keypair \
  --query "KeyMaterial" \
  --output text > ~/.ssh/ubertool-keypair.pem

chmod 400 ~/.ssh/ubertool-keypair.pem
```

```powershell
aws ec2 create-key-pair `
    --key-name ubertool-keypair `
    --query "KeyMaterial" `
    --output text | Set-Content -Encoding ascii "$env:USERPROFILE\.ssh\ubertool-keypair.pem"
```

### 4. Register a domain
Buy a domain (e.g., Namecheap ~$10/yr). You'll point its A-record to your Elastic IP before TLS setup.

### 5. Install Go (for cross-compilation)
```bash
# Verify Go is installed
go version  # requires 1.21+
```

---

## Deployment Steps

### Step 1 — Configure
```bash
cd deploy/ec2-mvp
cp config.env.template config.env
# Edit config.env with your values (domain, DB password, key pair name, etc.)
nano config.env
```

### Step 2 — Create AWS Infrastructure
```bash
chmod +x *.sh
./01_create_infra.sh
```
This creates:
- EC2 t3.micro (Ubuntu 24.04)
- RDS db.t3.micro (PostgreSQL 15)
- Security groups (EC2 open on 443/22; RDS only accepts from EC2)
- Elastic IP attached to EC2

**After this step:** Copy the printed Elastic IP and set your domain's A-record:
```
Type: A
Host: api          (or @ for root domain)
Value: <Elastic IP>
TTL: 300
```

### Step 3 — Initialize Database
```bash
./02_init_db.sh
```
Waits for RDS to be ready, then applies `ubertool_schema_trusted.sql` through the EC2 instance.

### Step 4 — Set Up TLS
```bash
./03_setup_tls.sh
```
Installs Certbot, obtains a Let's Encrypt certificate for your domain. DNS must have propagated first.

> **Why Let's Encrypt?** Certificates are trusted by Android and iOS without any extra configuration or certificate pinning. Auto-renews every 90 days via a systemd timer + deploy hook that restarts the service.

### Step 5 — Build and Deploy
```bash
./04_deploy.sh
```
- Cross-compiles Go binary for `linux/amd64`
- SCPs binary + env config to EC2
- Installs + starts `ubertool-api` as a systemd service

---

## Redeployment (code updates)

For every subsequent code update, just run:
```bash
./04_deploy.sh
```

---

## Operations

### Logon to EC2 after ~\.ssh\config file
```powershell
ssh ubertool-ec2
```

### View logs
```bash
ssh -i ~/.ssh/ubertool-keypair.pem ubuntu@<ELASTIC_IP>
sudo journalctl -u ubertool-api -f
```

### Restart service
```bash
ssh -i ~/.ssh/ubertool-keypair.pem ubuntu@<ELASTIC_IP> \
  "sudo systemctl restart ubertool-api"
```

### Check RDS status
```bash
aws rds describe-db-instances \
  --db-instance-identifier ubertool-db \
  --query "DBInstances[0].DBInstanceStatus" \
  --output text
```

### Verify TLS certificate
```bash
# From your local machine (requires grpcurl)
grpcurl -v <your-domain>:443 list
```

---

## Android Client Configuration

In your Kotlin gRPC client, connect using the domain (not IP). Let's Encrypt certs are trusted by default — no custom TrustManager needed:

```kotlin
val channel = ManagedChannelBuilder
    .forAddress("api.yourdomain.com", 443)
    .useTransportSecurity()   // TLS on — Let's Encrypt trusted natively
    .build()
```

## iOS Client Configuration (future)

Same approach in Swift with SwiftGRPC or grpc-swift:

```swift
let channel = ClientConnection
    .usingTLSBackedByNIOSSL(on: group)
    .connect(host: "api.yourdomain.com", port: 443)
```

No custom certificates or trust anchors required.

---

## File Reference

| File | Purpose |
|------|---------|
| `config.env.template` | Copy to `config.env`; fill in your values |
| `01_create_infra.sh` | Create EC2, RDS, SGs, Elastic IP |
| `02_init_db.sh` | Apply PostgreSQL schema to RDS |
| `03_setup_tls.sh` | Let's Encrypt TLS certificate |
| `04_deploy.sh` | Build Go binary, deploy + start service |
| `ec2_userdata.sh` | EC2 first-boot setup (runs automatically) |
| `ubertool-api.service` | systemd unit (deployed by 04_deploy.sh) |
| `99_teardown.sh` | Destroy all AWS resources |
| `infra_state.env` | Auto-generated; stores IDs from step 1 |

---

## Cost Estimate (AWS Credits)

| Resource | Type | Est. Cost/month |
|----------|------|----------------|
| EC2 | t3.micro | ~$8.50 |
| RDS | db.t3.micro | ~$14.00 |
| Elastic IP | (attached, running) | $0.00 |
| Storage | 20 GB gp2 RDS | ~$2.30 |
| **Total** | | **~$25/month** |

Your $200 AWS promotional credits cover ~8 months of this configuration.
