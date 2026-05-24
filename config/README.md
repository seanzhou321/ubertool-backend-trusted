# Configuration System

This directory contains YAML configuration files for the Ubertool Trusted Backend microservice.

## Configuration Files

The service supports six run scenarios split across two deployment targets.

### Desktop (server runs on a developer's machine)

| File | Scenario | TLS | FCM | 2FA | SMTP | Run command |
|---|---|---|---|---|---|---|
| `config.precommit.yaml` | A1 — Automated pre-commit coding tests | off | off | off (fixed code) | mock | `make run-precommit` |
| `config.desktop.uitest.yaml` | A2 — UI automated tests from emulator/device | off | on | off (fixed code) | mock | `make run-desktop-uitest` |
| `config.desktop.manual.yaml` | A3 — UI manual tests from emulator/device | off | on | on (live email) | real | `make run-desktop-manual` |

### EC2 (server runs on EC2 with RDS and Let's Encrypt TLS)

| File | Scenario | TLS | FCM | 2FA | SMTP | Notes |
|---|---|---|---|---|---|---|
| `config.ec2.uitest.yaml` | B1 — API automated tests (smoke) | on | on | off (fixed code) | mock | Test scripts use `config.ec2.apitest.yaml` |
| `config.ec2.uitest.yaml` | B2 — UI automated tests from emulator | on | on | off (fixed code) | mock | Shared with B3 |
| `config.ec2.uitest.yaml` | B3 — UI automated tests from Play Store app | on | on | off (fixed code) | mock | Identical server config to B2 |
| `config.prod.yaml` | B4 — UI manual tests / production-equivalent | on | on | on (live email) | real | Template — fill in CHANGE_ME values |

> **B1** runs `make test-smoke-ec2` from a local machine — the server uses `config.ec2.uitest.yaml`
> and the test scripts use `config.ec2.apitest.yaml` (gitignored; contains the live EC2 hostname).

> **B2 and B3** are identical from the server's perspective (same TLS, FCM, no 2FA, no email).
> They share a single config file; the difference is which client runs the tests.

> **B4 actual credentials** live in `deploy/ec2-mvp/config.yaml` (gitignored).
> `config.prod.yaml` in this directory is the committed template with `CHANGE_ME` placeholders.

### Config template files and client config file

| File | Purpose |
|---|---|
| `config.yaml` | Default config loaded when no `-config` flag is supplied (`make run-dev`) |
| `config.yaml.template` | Documented production template (predates `config.prod.yaml`) |
| `mail_config.test.yaml.template` | Documented test template for mail service testing |
| `config.ec2.apitest.yaml` | Client-side config used by `make test-smoke-ec2` (server host + TLS only) |

## 2FA Toggle

Every config file includes a `two_fa` block that controls whether two-factor
authentication is enforced:

```yaml
two_fa:
  enabled: true          # true = random code delivered via email (production)
  fixed_passcode: ""     # required only when enabled=false; must be 5 numeric digits
```

| `enabled` | Behaviour |
|---|---|
| `true` | A random 5-digit code is generated on each login and emailed to the user. `fixed_passcode` is ignored. |
| `false` | The `fixed_passcode` value from config is used. No email is sent. A `WARN` log entry is emitted at login to make bypass mode visible. |

Use `enabled: false` for all automated test scenarios (A1, A2, B1, B2, B3) so the
test framework can supply the known code without intercepting an email.

The toggle can also be overridden at runtime without editing the file:
```powershell
$env:TWO_FA_ENABLED = "true"
$env:TWO_FA_FIXED_PASSCODE = "47291"
```

## Configuration Structure

### Server
- `host`: Server bind address (default: `0.0.0.0`)
- `port`: gRPC server port (default: `50052`)
- `grpc_reflection`: Enable gRPC server reflection — allows `grpcurl` to enumerate services. Set `true` for all non-production scenarios, `false` in `config.prod.yaml`. Env: `GRPC_REFLECTION`

### Database
- `host`: PostgreSQL host
- `port`: PostgreSQL port (default: `5432` for prod/EC2, `5454` for desktop)
- `user`: Database user
- `password`: Database password
- `database`: Database name
- `ssl_mode`: SSL mode (`disable`, `require`, `verify-ca`, `verify-full`)

### TLS
- `enabled`: Whether gRPC TLS is active (`true` on EC2, `false` on desktop)
- `cert_file`: Path to the certificate chain (Let's Encrypt `fullchain.pem`)
- `key_file`: Path to the private key (`privkey.pem`)

### SMTP
- `host`: SMTP server host — set to `"mock"` to suppress all email delivery
- `port`: SMTP server port (default: `587`)
- `user`: SMTP username/email
- `password`: SMTP app password
- `from`: From email address

### JWT
- `secret`: JWT signing secret (minimum 32 characters)
- `access_token_expiry_minutes`: Access token validity (default: 15 minutes)
- `refresh_token_expiry_minutes`: Refresh token validity (default: 7 days = 10080 min)
- `temp_token_expiry_minutes`: 2FA pending token validity (default: 5 minutes)

### 2FA
- `enabled`: See [2FA Toggle](#2fa-toggle) above
- `fixed_passcode`: 5-digit numeric code used when `enabled: false`

### Storage
- `type`: `"mock"` (local filesystem) or `"s3"` (AWS S3)
- `upload_dir`: Local upload directory (mock mode)
- `base_url`: Base URL for serving uploaded files (mock mode)
- `s3_bucket`: S3 bucket name (s3 mode)
- `s3_region`: AWS region (s3 mode)
- `max_file_size_mb`: Maximum upload size in megabytes
- `allowed_types`: Allowed MIME types

### Firebase (FCM)
- `firebase_key_path`: Path to the Firebase Admin SDK JSON key.
  Leave empty (`""`) to disable push notifications.

## Usage

### Running the server

```powershell
# A1 — Desktop pre-commit coding tests (no FCM, 2FA bypassed, mock email)
make run-precommit

# A2 — Desktop UI automated tests (FCM on, 2FA bypassed, mock email)
make run-desktop-uitest

# A3 — Desktop UI manual tests (FCM on, live 2FA email)
make run-desktop-manual

# Legacy dev target (real SMTP, 2FA on)
make run-dev

# Arbitrary config
go run ./cmd/server -config=config/config.precommit.yaml
```

### Running tests

```powershell
# Full pre-commit suite: unit + integration + e2e  (Scenario A1)
make test-precommit

# Unit tests only
make test-unit

# Integration tests (uses config.precommit.yaml)
make test-integration

# Smoke tests against live EC2
make test-smoke-ec2
```

### Environment Variable Overrides

Configuration values can be overridden using environment variables:

#### Database
- `DB_HOST` - Database host
- `DB_PORT` - Database port
- `DB_USER` - Database user
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name
- `DB_SSL_MODE` - SSL mode

#### SMTP
- `SMTP_HOST` - SMTP host
- `SMTP_PORT` - SMTP port
- `SMTP_USER` - SMTP username
- `SMTP_PASSWORD` - SMTP password
- `SMTP_FROM` - From email address

#### JWT
- `JWT_SECRET` - JWT signing secret

#### 2FA
- `TWO_FA_ENABLED` - `"true"` or `"false"`
- `TWO_FA_FIXED_PASSCODE` - 5-digit numeric string (used when `TWO_FA_ENABLED=false`)

#### TLS
- `TLS_CERT` - Path to certificate file
- `TLS_KEY` - Path to private key file

#### Server
- `SERVER_HOST` - Server bind address
- `SERVER_PORT` - Server port

#### Storage
- `UPLOAD_DIR` - Upload directory path

### Example with Environment Variables

**PowerShell:**
```powershell
$env:DB_PASSWORD="secure_password"
$env:SMTP_PASSWORD="app_password"
$env:JWT_SECRET="your-super-secret-key-min-32-chars"
go run ./cmd/server -config=config/config.prod.yaml
```

**Bash:**
```bash
export DB_PASSWORD="secure_password"
export SMTP_PASSWORD="app_password"
export JWT_SECRET="your-super-secret-key-min-32-chars"
go run ./cmd/server -config=config/config.prod.yaml
```

## Production Deployment

### Security Best Practices

1. **Never commit sensitive values** to version control
2. **Use environment variables** for secrets in production
3. **Rotate secrets regularly** (JWT secret, database passwords)
4. **Use strong passwords** (minimum 32 characters for JWT secret)
5. **Enable SSL** for database connections in production
6. **Use app passwords** for Gmail SMTP (not account password)

### Production Checklist

Before deploying to production:

- [ ] Update `config.prod.yaml` with production values
- [ ] Set `DB_PASSWORD` environment variable
- [ ] Set `SMTP_PASSWORD` environment variable
- [ ] Set `JWT_SECRET` environment variable (min 32 chars)
- [ ] Change `database.ssl_mode` to `require`
- [ ] Update `database.host` to production database
- [ ] Update `smtp.user` and `smtp.from` to production email
- [ ] Update `storage.upload_dir` to production path
- [ ] Verify all configuration with `Validate()` method

### Docker/Kubernetes

For containerized deployments, mount configuration as:

**Docker:**
```bash
docker run -v /path/to/config:/app/config \
  -e DB_PASSWORD=secret \
  -e SMTP_PASSWORD=secret \
  -e JWT_SECRET=secret \
  ubertool-backend -config=/app/config/config.prod.yaml
```

**Kubernetes ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ubertool-config
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 50051
    # ... rest of config
```

**Kubernetes Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ubertool-secrets
type: Opaque
stringData:
  DB_PASSWORD: "your-db-password"
  SMTP_PASSWORD: "your-smtp-password"
  JWT_SECRET: "your-jwt-secret-min-32-chars"
```

## Configuration Validation

The configuration system validates at startup:
- Server port range (1–65535)
- Database host, user, and database name are not empty
- SMTP host and port are valid
- JWT secret is at least 32 characters
- Storage: `upload_dir` required for mock mode; `s3_bucket` and `s3_region` required for S3 mode
- `two_fa.fixed_passcode` must be exactly 5 numeric digits when `two_fa.enabled: false`

Invalid configurations cause the application to exit at startup with a descriptive error.

## Development Tips

### Quick Start for Development
1. Copy `config.desktop.manual.yaml` to `config.local.yaml`
2. Update with your local settings
3. Add `config.local.yaml` to `.gitignore`
4. Run with: `go run ./cmd/server -config=config/config.local.yaml`

### Testing Configuration
Use `config.precommit.yaml` for running tests:
```bash
go test ./... -config=../config/config.precommit.yaml
```
