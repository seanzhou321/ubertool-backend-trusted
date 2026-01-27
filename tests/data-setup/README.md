# Test Data Setup

This directory contains tools to populate test data into the database from YAML configuration.

## Files

- `user_org.yaml` - Test data configuration
- `setup.go` - Script to populate data from YAML into database

## Usage

### From project root (recommended):

```bash
make setup-test-data
```

Or:

```bash
go run ./tests/data-setup/setup.go
```

### From this directory:

```bash
go run setup.go
```

## YAML Structure

The `user_org.yaml` file contains:

- **config_file**: Path to config file (relative to project root: `config/config.test.yaml`)
- **organization**: Organization details to create
- **users**: Array of users to create with their roles and settings

Each user will be:
1. Created in the `users` table with bcrypt-hashed password
2. Linked to the organization in the `users_orgs` table with role, status, and balance

## Example

```yaml
config_file: "config/config.test.yaml"

organization:
  name: "Test Community Church"
  description: "A test organization for automated testing"
  address: "123 Test Street, Test City, TS 12345"
  metro: "Test Metro Area"
  admin_phone_number: "+1234567890"
  admin_email: "admin@testchurch.org"

users:
  - email: "superadmin@testchurch.org"
    phone_number: "+1234567890"
    password: "TestPassword123!"
    name: "Super Admin User"
    avatar_url: "https://example.com/avatar/superadmin.jpg"
    role: "SUPER_ADMIN"
    status: "ACTIVE"
    balance_cents: 0

  - email: "admin1@testchurch.org"
    phone_number: "+1234567891"
    password: "TestPassword123!"
    name: "Admin User 1"
    avatar_url: "https://example.com/avatar/admin1.jpg"
    role: "ADMIN"
    status: "ACTIVE"
    balance_cents: 10000
```

## Notes

- The script is idempotent - you can run it multiple times, but it will create duplicate entries if run repeatedly
- All passwords are hashed with bcrypt (cost 10) before storage
- The script runs in a transaction - if any step fails, all changes are rolled back
- The config path can be relative to project root or absolute
