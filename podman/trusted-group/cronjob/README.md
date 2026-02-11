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

- Same Docker image as the backend server (`ubertool-backend:latest`)
- Different command: `/app/cronjob` instead of `/app/server`
- Singleton container (do not scale)
- Shares configuration and database with backend

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
- Mounted as: `/config/config.yaml` in the container

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
- Ensure postgres container is running

### Jobs not executing
- Verify timezone is UTC: `podman exec ubertool-cronjob date`
- Check cron registration in logs: Look for "All cron jobs registered successfully"
- Test job manually: `podman exec ubertool-cronjob /app/cronjob --run-once <job-name>`

### Database connection errors
- Ensure cronjob container can reach postgres
- Check network: `podman network inspect ubertool-network`
- Verify database credentials in config

## Development

### Build the image:
```bash
cd ../../../  # Back to repo root
podman build -t ubertool-backend:latest .
```

### Test locally:
```bash
# Run a single job
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once mark-overdue-rentals

# Start the scheduler
go run cmd/cronjob/main.go --config=config/config.dev.yaml
```

## Production Considerations

- Set resource limits in docker-compose.yaml
- Configure log rotation
- Set up monitoring/alerting for job failures
- Consider using Kubernetes CronJob for better observability
