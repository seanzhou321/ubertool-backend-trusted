# Cronjob Quick Start Guide

This guide helps you get the cronjob scheduler up and running quickly.

## Prerequisites

- Go 1.24+
- Podman or Docker
- PostgreSQL (via podman-compose)

## Quick Setup (5 minutes)

### 1. Install Dependencies

```bash
# Download the new cron library
go mod download
```

### 2. Build Locally (Optional - for testing)

```bash
# Build both server and cronjob binaries
make build

# Verify binaries created
ls bin/
# Should see: server.exe, cronjob.exe
```

### 3. Test a Single Job

```bash
# Run mark-overdue-rentals once
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once=mark-overdue-rentals

# Run all nightly jobs
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once=all-nightly
```

### 4. Build Docker Image

```bash
# Build image with both server and cronjob binaries
make docker-build

# Verify image created
podman images | grep ubertool-backend
```

### 5. Deploy Cronjob Container

```bash
# Deploy cronjob (will also start postgres if not running)
make deploy-cronjob

# Check if running
podman ps | grep ubertool-cronjob
```

### 6. Verify It's Working

```bash
# View logs
make cronjob-logs

# You should see:
# "Starting Ubertool Cronjob Runner..."
# "All cron jobs registered successfully"
# "Cronjob scheduler is running..."
```

## Common Commands

### Development

```bash
# Run scheduler locally (terminal stays open)
go run cmd/cronjob/main.go --config=config/config.dev.yaml

# Run with specific job
make run-cronjob-once JOB=mark-overdue-rentals
make run-cronjob-once JOB=take-balance-snapshots
```

### Production/Testing

```bash
# Build and deploy everything
make deploy-all

# Just deploy cronjob
make deploy-cronjob

# View logs
make cronjob-logs

# Check status
make cronjob-status

# Restart
make cronjob-restart
```

### Manual Job Execution (in container)

```bash
# Execute inside running container
podman exec ubertool-cronjob /app/cronjob --run-once=mark-overdue-rentals

# Or stop scheduler and run job
podman stop ubertool-cronjob
podman run --rm \
  -v ./config:/config:ro \
  --network ubertool-network \
  ubertool-backend:latest \
  /app/cronjob --config=/config/config.yaml --run-once=all-monthly
```

## Available Jobs

### Individual Jobs
- `mark-overdue-rentals` - Mark rentals as overdue
- `send-overdue-reminders` - Email overdue rental reminders
- `send-bill-reminders` - Email bill payment reminders
- `check-overdue-bills` - Mark 10+ day old bills as disputed
- `resolve-disputed-bills` - Force resolve unresolved disputes
- `take-balance-snapshots` - Snapshot balances before bill splitting
- `perform-bill-splitting` - Calculate and create bills

### Batch Jobs
- `all-nightly` - Run all nightly jobs in sequence
- `all-monthly` - Run all monthly jobs in sequence

## Cron Schedule

| Time | Job |
|------|-----|
| 2:00 AM | Mark overdue rentals |
| 3:00 AM | Send overdue reminders |
| 4:00 AM | Send bill reminders |
| 5:00 AM (10th) | Check overdue bills |
| 11:00 PM (Last day) | Resolve disputed bills |
| 11:30 PM (Last day) | Take balance snapshots |
| 12:00 AM (1st) | Perform bill splitting |

All times in UTC.

## Troubleshooting

### Problem: "Cannot connect to database"

```bash
# Check postgres is running
podman ps | grep postgres

# Start postgres
cd podman/trusted-group/services
podman-compose up -d postgres

# Test connection
podman exec ubertool-postgres psql -U ubertool -d ubertool_trusted -c "SELECT 1"
```

### Problem: "Failed to send email"

```bash
# Check SMTP config
cat config/config.dev.yaml | grep -A 5 smtp

# Test email service separately
go run cmd/server/main.go --config=config/config.dev.yaml
# Then use API to send test email
```

### Problem: Jobs not running on schedule

```bash
# Check logs for registration
podman logs ubertool-cronjob | grep "registered"

# Verify timezone
podman exec ubertool-cronjob date

# Should show UTC time
```

### Problem: Container keeps restarting

```bash
# Check logs for errors
podman logs ubertool-cronjob

# Common causes:
# 1. Database not accessible
# 2. Config file not mounted
# 3. Binary failed to build

# Check config mount
podman inspect ubertool-cronjob | grep Mounts -A 10
```

## Development Tips

### Adding a New Job

1. Add function to `internal/jobs/*.go`
```go
func (jr *JobRunner) MyNewJob() {
    jr.runWithRecovery("MyNewJob", func() {
        // Your logic here
    })
}
```

2. Register in `internal/scheduler/scheduler.go`
```go
_, err = s.cron.AddFunc("0 0 6 * * *", s.jobs.MyNewJob) // 6 AM daily
```

3. Add case in `cmd/cronjob/main.go`
```go
case "my-new-job":
    jobRunner.MyNewJob()
```

4. Rebuild and deploy
```bash
make docker-build
make deploy-cronjob
```

### Testing Jobs

```bash
# Unit test with mock DB
go test ./internal/jobs/... -v

# Integration test with real DB
go test ./internal/jobs/... -config=config/config.test.yaml -v

# Manual test
go run cmd/cronjob/main.go --config=config/config.test.yaml --run-once=my-new-job
```

## Architecture Summary

```
┌─────────────┐         ┌─────────────┐
│   Server    │         │   Cronjob   │
│  Container  │         │  Container  │
└──────┬──────┘         └──────┬──────┘
       │                       │
       │   Same Docker Image   │
       │   Different CMD       │
       │                       │
       └───────┬───────────────┘
               │
        ┌──────▼──────┐
        │  PostgreSQL │
        │  Container  │
        └─────────────┘
```

- **Same repo**: ubertool-backend-trusted
- **Same image**: Built once with both binaries
- **Separate containers**: Different resource limits
- **Shared DB**: Both connect to same postgres

## Next Steps

1. ✅ Cronjob running
2. Monitor logs for first job execution
3. Set up monitoring/alerting (future)
4. Test monthly jobs at month end
5. Deploy to production (Milestone 3)

For detailed information, see [CRONJOB_README.md](../CRONJOB_README.md)
