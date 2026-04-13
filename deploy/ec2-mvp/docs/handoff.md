# Ubertool Backend Deployment - Handoff Notes
# Date: April 13, 2026

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
- Public IP     : 44.237.82.249 (TEMPORARY - changes on EC2 restart)
                  Elastic IP has NOT been allocated yet.
- RDS db.t3.micro PostgreSQL 17.6 : ubertool-backend-trusted-db
- EC2 SG and RDS SG IDs : see infra_state.env
- SSH alias     : ubertool-ec2 -> ubuntu@44.237.82.249
- SSH key       : ~/.ssh/ixorashare-ec2-key.pem

## Deployment Scripts (PowerShell, deploy/ec2-mvp/)
- 01_create_infra.ps1  DONE - infrastructure created
- 02_init_db.ps1       DONE - schema applied to RDS
- 03_setup_tls.ps1     PENDING - run in Phase 2 after domain and Elastic IP
- 04_deploy.ps1        IN PROGRESS - see pending fixes below
- 99_teardown.ps1      destroys all AWS resources
- Load-Env.ps1         shared helper, dot-sourced by all scripts
- config.env           local secrets (gitignored)
- infra_state.env      auto-generated AWS resource IDs (gitignored)

## Current Status
Infrastructure and database are up and healthy. The binary is deployed
but the service is crash-looping with this error:
  Failed to load configuration: failed to read config file:
  open config/config.dev.yaml: no such file or directory

Three fixes are needed before redeploying.

## Pending Fix 1 - Upload config.prod.yaml to EC2
The binary accepts a -config flag (cmd/server/main.go line 36).
It defaults to config/config.dev.yaml which does not exist on EC2.
config.prod.yaml must be uploaded and the service pointed at it.

In 04_deploy.ps1, add config.prod.yaml to the scp command in step 3:
  scp ... "$PROJECT_ROOT\config\config.prod.yaml" "ubuntu@${ELASTIC_IP}:/tmp/"

Add these lines to the install script in step 4:
  sudo mv /tmp/config.prod.yaml /etc/ubertool/config.prod.yaml
  sudo chown root:ubertool /etc/ubertool/config.prod.yaml
  sudo chmod 640 /etc/ubertool/config.prod.yaml

## Pending Fix 2 - Update ubertool-api.service ExecStart
File location: deploy/ec2-mvp/ubertool-api.service
Change ExecStart from:
  ExecStart=/usr/local/bin/ubertool-api
To:
  ExecStart=/usr/local/bin/ubertool-api -config /etc/ubertool/config.prod.yaml

## Pending Fix 3 - Port mismatch
config.prod.yaml has server.port=50052 (correct for plaintext gRPC).
The EC2 security group currently only opens port 443, not 50052.
Run this once to open the port:
  aws ec2 authorize-security-group-ingress \
    --group-id <EC2_SG_ID from infra_state.env> \
    --protocol tcp --port 50052 --cidr "0.0.0.0/0" \
    --region us-west-2

Also update GRPC_PORT=50052 in config.env.

## config.prod.yaml Notes
Located at: config/config.prod.yaml
- server.port = 50052 (changes to 443 in Phase 2 after TLS)
- database.host must equal the RDS endpoint from infra_state.env
  (the current value is a stale hostname from a previous project)
- smtp configured with Gmail app password
- jwt secret is set
- storage uses local filesystem mock (upload_dir: /var/ubertool/uploads)
  The directory /var/ubertool/uploads must exist on EC2
- firebase-admin-key.json is at config/firebase-admin-key.json
  InitFirebase() logs a warning and continues if it cannot find the key
  so push notifications will be disabled but the server will still start

## Deployment Plan - Remaining Steps

### Phase 1 - Fix and verify microservice (current phase, no TLS)
1. Apply the three pending fixes above
2. Redeploy with .\04_deploy.ps1
3. Verify service starts cleanly:
     ssh ubertool-ec2 "sudo journalctl -u ubertool-api -n 50 --no-pager"
4. Verify port is listening:
     ssh ubertool-ec2 "sudo ss -tlnp | grep 50052"
5. Test gRPC connectivity from Android app using plaintext over temporary IP

### Phase 2 - Elastic IP + Domain + TLS (before Android app release)
TLS via Let's Encrypt. Trusted natively by Android and iOS with no extra
app configuration. No cert bundling needed.

Steps:
1. Allocate Elastic IP and associate with EC2:
     aws ec2 allocate-address --domain vpc --region us-west-2
     aws ec2 associate-address --instance-id i-023402e9ce97c0c86 \
       --allocation-id <alloc-id> --region us-west-2
   Update ELASTIC_IP and ALLOC_ID in infra_state.env
2. Register a domain (~$10/yr, e.g. Namecheap)
3. Point domain A-record to the Elastic IP (TTL 300)
4. Wait for DNS propagation (a few minutes)
5. Run .\03_setup_tls.ps1
6. Update config.prod.yaml server.port from 50052 to 443
7. Redeploy with .\04_deploy.ps1
8. Update Android client channel to use domain and useTransportSecurity()

### Phase 3 - Android App Release
1. Final testing against TLS endpoint
2. Submit to Google Play Store (developer account already registered, $25 paid)

## Android Client Connection (Phase 1, plaintext, temporary IP)
  val channel = ManagedChannelBuilder
      .forAddress("44.237.82.249", 50052)
      .usePlaintext()
      .build()

## Android Client Connection (Phase 2, Let's Encrypt TLS, domain)
  val channel = ManagedChannelBuilder
      .forAddress("api.yourdomain.com", 443)
      .useTransportSecurity()
      .build()
  // No custom TrustManager needed - Let's Encrypt trusted natively on Android and iOS

## iOS Client Connection (future, same as Android approach)
  let channel = ClientConnection
      .usingTLSBackedByNIOSSL(on: group)
      .connect(host: "api.yourdomain.com", port: 443)
  // No custom CA needed - Let's Encrypt trusted natively

## Key Architecture Decisions
- EC2 t3.micro + RDS db.t3.micro handles up to ~10,000 users
- Elastic IP not yet allocated - current IP is temporary, changes on EC2 restart
- Domain will be registered before Android app release (~$10/yr)
- TLS via Let's Encrypt - trusted natively on Android and iOS, no cert bundling
- Port 50052 for Phase 1 plaintext, port 443 for Phase 2 TLS
- RDS not publicly accessible - only reachable from EC2 security group
- RDS backup retention = 0 (free tier requirement)
- PostgreSQL 17.6, Ubuntu 24.04 LTS
- AWS $200 promotional credits cover ~8 months at ~$25/month
- Cron job (cmd/cronjob/main.go) deferred until after MVP
