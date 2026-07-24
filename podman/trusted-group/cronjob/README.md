# Ubertool Cronjob Container

This directory contains the deployment configuration for the Ubertool cronjob scheduler.

## Overview

The cronjob container runs scheduled tasks including:
- **Nightly jobs** (2-5 AM UTC):
  - Mark overdue rentals
  - Send overdue reminders
  - Send bill reminders
  - Check overdue bills (10th of each month)
  
- **Monthly jobs** (End of month):
  - Resolve disputed bills (11 PM UTC last day)
  - Take balance snapshots (11:30 PM UTC last day)
  - Perform bill splitting (12 AM UTC 1st of month)

## Architecture

- Dedicated image (`ubertool-cronjob:latest`, built from `Dockerfile` in this directory) —
  the server has its own separate image, see `podman/trusted-group/grpc_service/`
- Singleton container (do not scale)
- Shares configuration and database with the backend server

## Deployment

### Start the cronjob container:
```bash
cd podman/trusted-group/cronjob
podman-compose up -d
```

### View logs:
```bash
podman-compose logs -f cronjob
```

### Stop the container:
```bash
podman-compose down
```

## Running Jobs Manually

You can run individual jobs on-demand:

```bash
# Run a specific job
podman exec ubertool-cronjob /app/cronjob --run-once mark-overdue-rentals

# Run all nightly jobs
podman exec ubertool-cronjob /app/cronjob --run-once all-nightly

# Run all monthly jobs
podman exec ubertool-cronjob /app/cronjob --run-once all-monthly
```

Available job names:
- `mark-overdue-rentals`
- `send-overdue-reminders`
- `send-bill-reminders`
- `check-overdue-bills`
- `resolve-disputed-bills`
- `take-balance-snapshots`
- `perform-bill-splitting`
- `all-nightly`
- `all-monthly`

## Configuration

The cronjob uses the same configuration file as the backend server:
- Located at: `config/config.yaml`
- Mounted as: `/app/config/config.yaml` in the container

**Prerequisite**: `make db-deploy` must already be running (`ubertool-postgres` on
`localhost:5454`) — this compose file does not start its own Postgres. The cronjob container
reaches it via `host.containers.internal:5454` (`DB_HOST`/`DB_PORT` env overrides in
`docker-compose.yaml`), the same pattern used by `podman/trusted-group/grpc_service/`.

## Monitoring

### Check if cronjob is running:
```bash
podman ps | grep ubertool-cronjob
```

### View recent job executions:
```bash
podman logs --tail 100 ubertool-cronjob
```

### Check for errors:
```bash
podman logs ubertool-cronjob 2>&1 | grep ERROR
```

## Troubleshooting

### Container keeps restarting
- Check logs: `podman-compose logs cronjob`
- Verify database connection in config
- Ensure `ubertool-postgres` is running (`make db-deploy`, not part of this compose file)

### Jobs not executing
- Verify timezone is UTC: `podman exec ubertool-cronjob date`
- Check cron registration in logs: Look for "All cron jobs registered successfully"
- Test job manually: `podman exec ubertool-cronjob /app/cronjob --run-once <job-name>`

### Database connection errors
- Ensure `ubertool-postgres` is running: `podman ps --filter name=ubertool-postgres`
- Confirm the container can reach the host: `podman exec ubertool-cronjob getent hosts host.containers.internal`
- `DB_HOST`/`DB_PORT` are overridden via environment variables in `docker-compose.yaml` —
  they take precedence over `config/config.yaml`'s `database.host: localhost` (see
  `internal/config/config.go`'s env-var overrides)

## Development

### Build the image:
```bash
make cronjob-build
# equivalent to:
#   podman build -f podman/trusted-group/cronjob/Dockerfile -t ubertool-cronjob:latest .
```

### Test locally:
```bash
# Run a single job
go run cmd/cronjob/main.go --config=config/config.desktop.manual.yaml --run-once mark-overdue-rentals

# Start the scheduler
go run cmd/cronjob/main.go --config=config/config.desktop.manual.yaml
```

## Production Considerations

- Set resource limits in docker-compose.yaml
- Configure log rotation
- Set up monitoring/alerting for job failures
- Consider using Kubernetes CronJob for better observability
