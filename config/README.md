# Configuration System

This directory contains YAML configuration files for the Ubertool Trusted Backend microservice.

## Configuration Files

- **`config.dev.yaml`** - Development environment configuration
- **`config.prod.yaml`** - Production environment configuration template
- **`config.test.yaml`** - Testing environment configuration

## Configuration Structure

### Server
- `host`: Server bind address (default: `0.0.0.0`)
- `port`: gRPC server port (default: `50051`)

### Database
- `host`: PostgreSQL host
- `port`: PostgreSQL port (default: `5432` for prod, `5454` for dev)
- `user`: Database user
- `password`: Database password
- `database`: Database name
- `ssl_mode`: SSL mode (`disable`, `require`, `verify-ca`, `verify-full`)

### SMTP
- `host`: SMTP server host (e.g., `smtp.gmail.com`)
- `port`: SMTP server port (default: `587` for TLS)
- `user`: SMTP username/email
- `password`: SMTP password (use app password for Gmail)
- `from`: From email address

### JWT
- `secret`: JWT signing secret (minimum 32 characters)
- `access_token_expiry_minutes`: Access token validity (default: 15 minutes)
- `refresh_token_expiry_minutes`: Refresh token validity (default: 7 days)
- `temp_token_expiry_minutes`: Temporary token validity for 2FA (default: 5 minutes)

### Storage
- `upload_dir`: Directory for uploaded files
- `max_file_size_mb`: Maximum file size in megabytes
- `allowed_types`: List of allowed MIME types for uploads

## Usage

### Running with Default Configuration
```bash
# Uses config/config.dev.yaml by default
go run ./cmd/server
```

### Running with Specific Configuration
```bash
# Development
go run ./cmd/server -config=config/config.dev.yaml

# Production
go run ./cmd/server -config=config/config.prod.yaml

# Testing
go run ./cmd/server -config=config/config.test.yaml
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

The configuration system automatically validates:
- Server port range (1-65535)
- Database host, user, and database name are not empty
- SMTP host and port are valid
- JWT secret is at least 32 characters
- Upload directory is specified

Invalid configurations will cause the application to fail at startup with a descriptive error message.

## Development Tips

### Quick Start for Development
1. Copy `config.dev.yaml` to `config.local.yaml`
2. Update with your local settings
3. Add `config.local.yaml` to `.gitignore`
4. Run with: `go run ./cmd/server -config=config/config.local.yaml`

### Testing Configuration
Use `config.test.yaml` for running tests:
```bash
go test ./... -config=../config/config.test.yaml
```
