# Ubertool Backend Deployment - Handoff Notes
# Date: April 14, 2026
# Status: COMPLETE — all phases through Phase 2b (TLS + domain) are done.
#          Next: Phase 3 (Android app release)

## Project Overview
Ubertool is a mobile app backend consisting of:
- Go gRPC API microservice (cmd/server/main.go)
- PostgreSQL database
- Cron engine (cmd/cronjob/main.go) - deferred, not part of MVP deployment
- Android Kotlin client (future iOS Swift client planned, same gRPC backend)

## Project Paths
- Backend root  : C:\Users\yixio\ubertool\ubertool-backend-trusted
- Android app   : H:\github\projects\ubertool\ubertool-android-trusted
- Deploy scripts: C:\Users\yixio\ubertool\ubertool-backend-trusted\deploy\ec2-mvp

## AWS Infrastructure (deployed, us-west-2)
- EC2 t3.micro  : i-023402e9ce97c0c86
- Elastic IP    : 35.160.176.235 (allocated and associated)
- Domain        : api.ixorashare.com → 35.160.176.235 (A record)
- RDS db.t3.micro PostgreSQL 17.6 : ubertool-backend-trusted-db
- EC2 SG and RDS SG IDs : see infra_state.env
- SSH alias     : ubertool-ec2 -> ubuntu@35.160.176.235
- SSH key       : ~/.ssh/ixorashare-ec2-key.pem

## Deployment Scripts (PowerShell, deploy/ec2-mvp/)
- 01_create_infra.ps1  DONE - infrastructure created
- 02_init_db.ps1       DONE - schema applied to RDS
- 03_setup_tls.ps1     DONE - Let's Encrypt cert issued for api.ixorashare.com
- 04_deploy.ps1        DONE - binary deployed, service running, DB data verified
- 99_teardown.ps1      destroys all AWS resources
- Load-Env.ps1         shared helper, dot-sourced by all scripts
- config.env           local secrets (gitignored)
- infra_state.env      auto-generated AWS resource IDs (gitignored)

## Current Status
Phase 1 complete. The gRPC microservice is running on EC2 and healthy.
- 04_deploy.ps1 executed successfully
- ec2_userdata.sh executed successfully  
- Database population verified from EC2 terminal
- Service is listening on port 50052 (plaintext gRPC)

## Applied Fixes (all done, for reference)

### Fix 1 - Config file upload to EC2 (DONE)
Both main.go files updated to default to config/config.yaml instead of
config/config.dev.yaml. Production config moved from config/config.prod.yaml
to deploy/ec2-mvp/config.yaml (kept alongside deploy scripts, gitignored).

04_deploy.ps1 step 3 now uploads deploy/ec2-mvp/config.yaml:
  scp ... "$PSScriptRoot\config.yaml" "ubuntu@${ELASTIC_IP}:/tmp/"

04_deploy.ps1 step 4 installs it as /etc/ubertool/config.yaml:
  sudo mv /tmp/config.yaml /etc/ubertool/config.yaml
  sudo chown root:ubertool /etc/ubertool/config.yaml
  sudo chmod 640 /etc/ubertool/config.yaml

Step 4 also creates /var/ubertool/uploads (required for local file storage).

## Fix 2 - ubertool-api.service ExecStart (DONE)
File: deploy/ec2-mvp/ubertool-api.service
  ExecStart=/usr/local/bin/ubertool-api -config /etc/ubertool/config.yaml

## Fix 3 - Port mismatch (DONE)
config.yaml has server.port=50052 (correct for plaintext gRPC).
EC2 security group port 50052 opened. GRPC_PORT=50052 confirmed in config.env.

## config.yaml Notes
Located at: deploy/ec2-mvp/config.yaml (gitignored, contains secrets)
Installed on EC2 at: /etc/ubertool/config.yaml
- server.port = 50052 (changes to 443 in Phase 2 after TLS)
- database.host must equal the RDS endpoint from infra_state.env
  (the current value is a stale hostname from a previous project)
- smtp configured with Gmail app password
- jwt secret is set
- storage uses local filesystem mock (upload_dir: /var/ubertool/uploads)
  04_deploy.ps1 now creates /var/ubertool/uploads on EC2 automatically
- firebase-admin-key.json is at config/firebase-admin-key.json
  InitFirebase() logs a warning and continues if it cannot find the key
  so push notifications will be disabled but the server will still start

## Deployment Plan - Remaining Steps

### Phase 1 - COMPLETE
All deployment scripts executed successfully. Service running on 35.160.176.235:50052
(plaintext gRPC). Database data verified from EC2 terminal.

### Phase 2 - Domain + Elastic IP + TLS - COMPLETE
Domain: api.ixorashare.com | Elastic IP: 35.160.176.235 | TLS: Let's Encrypt
Cert: /etc/ubertool/certs/fullchain.pem (expires 2026-07-13, auto-renews via certbot.timer)
gRPC endpoint: api.ixorashare.com:50052 (TLS, useTransportSecurity())

### Phase 2b - Smoke Tests - COMPLETE

Smoke tests (tests/smoke/) verify TLS, API liveness, and DB connectivity via
the live gRPC endpoint — no SSH tunnel required.

Config: config/config.smoke.ec2.yaml (gitignored)
  server:
    host: api.ixorashare.com
    port: 50052
  tls:
    enabled: true

Run smoke tests:
  make test-smoke-ec2

Tests covered:
  TestTLSConnectivity       - raw TLS handshake, cert chain valid, expiry checked
  TestAPILiveness           - gRPC Login probe, server reachable and responding
  TestDatabaseConnectivity  - SearchOrganizations (orgs table), Login (users table),
                              ValidateInvite (invitations table)

All three tests pass against api.ixorashare.com:50052 as of April 14, 2026.

### Phase 3 - Android App Release
1. Final testing against TLS endpoint
2. Submit to Google Play Store (developer account already registered, $25 paid)

## Android Client Connection (active — TLS, api.ixorashare.com)
  val channel = ManagedChannelBuilder
      .forAddress("api.ixorashare.com", 50052)
      .useTransportSecurity()
      .build()
  // No custom TrustManager needed - Let's Encrypt trusted natively on Android and iOS

## iOS Client Connection (future, same approach)
  let channel = ClientConnection
      .usingTLSBackedByNIOSSL(on: group)
      .connect(host: "api.ixorashare.com", port: 50052)
  // No custom CA needed - Let's Encrypt trusted natively

## Key Architecture Decisions
- EC2 t3.micro + RDS db.t3.micro handles up to ~10,000 users
- Elastic IP: 35.160.176.235 (allocated, attached to EC2)
- Domain: api.ixorashare.com (Namecheap, A record → Elastic IP)
- TLS via Let's Encrypt - trusted natively on Android and iOS, no cert bundling
- Port 50052 with TLS (Let's Encrypt cert at /etc/ubertool/certs/)
- RDS not publicly accessible - only reachable from EC2 security group
- E2E tests connect to RDS via SSH tunnel (ssh -L 5454:<RDS>:5432 ubertool-ec2 -N)
- RDS backup retention = 0 (free tier requirement)
- PostgreSQL 17.6, Ubuntu 24.04 LTS
- AWS $200 promotional credits cover ~8 months at ~$25/month
- Cron job (cmd/cronjob/main.go) deferred until after MVP
