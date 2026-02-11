# Ubertool Cronjob Scheduler Implementation

## Overview

The Ubertool backend now includes a complete cronjob scheduler system for automated maintenance tasks. The implementation follows a clean separation between scheduling logic and business logic.

## Architecture

### Design Principles
- **Same repository, separate binaries**: Both server and cronjob are built from the same codebase
- **Shared Docker image**: Single image contains both binaries, reducing build time and ensuring version consistency
- **Separate containers**: Cronjob runs in its own container for production topology matching
- **Clean separation**: `internal/scheduler/` handles cron setup, `internal/jobs/` contains business logic

### Directory Structure

```
ubertool-backend-trusted/
├── cmd/
│   ├── server/           # API server binary
│   │   └── main.go
│   └── cronjob/          # Cronjob runner binary (NEW)
│       └── main.go
├── internal/
│   ├── scheduler/        # Cron scheduling logic (NEW)
│   │   └── scheduler.go
│   ├── jobs/             # Business logic for jobs (NEW)
│   │   ├── job_runner.go
│   │   ├── rental_jobs.go
│   │   ├── billing_jobs.go
│   │   └── notification_jobs.go
│   └── ...
├── podman/
│   └── trusted-group/
│       ├── services/     # Backend server container
│       ├── cronjob/      # Cronjob scheduler container (NEW)
│       │   ├── docker-compose.yaml
│       │   └── README.md
│       └── postgres/
├── podman/
│   └── trusted-group/
│       └── Dockerfile_services_cronjobs  # Multi-stage build (NEW)
├── .dockerignore         # Optimized Docker build at repo root (NEW)
└── Makefile              # Build and deployment commands (UPDATED)
```

## Scheduled Jobs

### Nightly Jobs (UTC Timezone)

| Job | Schedule | Function | Description |
|-----|----------|----------|-------------|
| Mark Overdue Rentals | 2:00 AM | `MarkOverdueRentals()` | Updates rentals past end_date to OVERDUE status |
| Send Overdue Reminders | 3:00 AM | `SendOverdueReminders()` | Emails renters with overdue rentals |
| Send Bill Reminders | 4:00 AM | `SendBillReminders()` | Reminds debtors/creditors about unpaid bills |
| Check Overdue Bills | 5:00 AM (10th) | `CheckOverdueBills()` | Marks 10+ day old bills as DISPUTED |

### Monthly Jobs (UTC Timezone)

| Job | Schedule | Function | Description |
|-----|----------|----------|-------------|
| Resolve Disputed Bills | 11:00 PM (Last day) | `ResolveDisputedBills()` | Applies system default action to unresolved disputes |
| Take Balance Snapshots | 11:30 PM (Last day) | `TakeBalanceSnapshots()` | Captures user balances before bill splitting |
| Perform Bill Splitting | 12:00 AM (1st) | `PerformBillSplitting()` | Calculates and creates bills |

## Implementation Details

### Technology Stack
- **Cron Library**: `github.com/robfig/cron/v3` - Battle-tested Go cron scheduler
- **Database**: PostgreSQL with helper functions (`check_overdue_bills()`, `auto_resolve_disputed_bills()`)
- **Containerization**: Podman/Docker with multi-stage builds

### Key Features

1. **Panic Recovery**: All jobs wrapped with panic recovery to prevent scheduler crashes
2. **Structured Logging**: Comprehensive logging with job start/end and error tracking
3. **Manual Execution**: Jobs can be run on-demand via CLI
4. **Idempotent Operations**: Jobs are safe to run multiple times
5. **Graceful Shutdown**: Proper signal handling for clean container stops

## Usage

### Building

```bash
# Install dependencies
go mod download

# Build both binaries locally
make build

# Build Docker image
make docker-build
```

### Running Locally

```bash
# Run scheduler (all jobs on schedule)
go run cmd/cronjob/main.go --config=config/config.dev.yaml

# Run a specific job once
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once=mark-overdue-rentals

# Run all nightly jobs
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once=all-nightly

# Run all monthly jobs
go run cmd/cronjob/main.go --config=config/config.dev.yaml --run-once=all-monthly
```

### Deployment

```bash
# Build image and deploy cronjob container
make deploy-cronjob

# Deploy all services (server + cronjob)
make deploy-all

# View cronjob logs
make cronjob-logs

# Check cronjob status
make cronjob-status

# Restart cronjob
make cronjob-restart
```

### Container Management

```bash
# Start cronjob
cd podman/trusted-group/cronjob
podman-compose up -d

# View logs
podman logs -f ubertool-cronjob

# Execute job manually
podman exec ubertool-cronjob /app/cronjob --run-once mark-overdue-rentals

# Stop cronjob
podman-compose down
```

## Job Descriptions

### Rental Jobs

#### MarkOverdueRentals
- **Purpose**: Mark rentals as overdue when past their end date
- **Query**: Updates rentals with status='ACTIVE' and end_date < today
- **Side Effects**: Changes rental status to 'OVERDUE'
- **Notifications**: None (separate job handles notifications)

### Billing Jobs

#### CheckOverdueBills
- **Purpose**: Automatically dispute bills unpaid after 10 days
- **Logic**: Calls DB function `check_overdue_bills()`
- **Side Effects**: Creates dispute records, updates bill status to 'DISPUTED'
- **Logging**: Logs number of bills disputed

#### ResolveDisputedBills
- **Purpose**: Apply system default action to unresolved disputes before new settlement
- **Logic**: For each org, calls `auto_resolve_disputed_bills(org_id, settlement_month)`
- **Side Effects**: Blocks debtors from renting, blocks creditors from lending
- **Business Rule**: Both parties blocked if dispute unresolved by month end

#### TakeBalanceSnapshots
- **Purpose**: Capture point-in-time balances for auditing
- **Timing**: Last day of month before bill splitting
- **Logic**: Inserts current balance_cents from users_orgs into balance_snapshots
- **Idempotency**: ON CONFLICT DO NOTHING for duplicate snapshots

#### PerformBillSplitting
- **Purpose**: Calculate who owes whom for the past month
- **Algorithm**: Greedy matching (largest debtor → largest creditor)
- **Logic**: 
  1. Get all users with non-zero balances in each org
  2. Separate into debtors (negative balance) and creditors (positive balance)
  3. Match debtors to creditors optimally
  4. Create bills with notice_sent_at = NOW()
- **Side Effects**: Inserts records into `bills` table

### Notification Jobs

#### SendOverdueReminders
- **Purpose**: Email renters about overdue tool returns
- **Query**: Finds rentals with status='OVERDUE'
- **Email Content**: Rental ID, tool name, original due date
- **Error Handling**: Logs failures but continues processing other rentals

#### SendBillReminders
- **Purpose**: Remind debtors and creditors about pending payments
- **Query**: Finds bills with status='PENDING' and notice_sent > 3 days ago
- **Email Content**: Bill ID, amount, settlement month, counterparty name
- **Recipients**: Both debtor (to pay) and creditor (to expect payment)

## Configuration

### Environment Variables
- `TZ=UTC` - Ensure consistent timezone across containers
- Config file provides: DB connection, SMTP settings, log level

### Cron Schedule Format
Using `robfig/cron/v3` with seconds precision:
```
Seconds Minutes Hours Day Month DayOfWeek
0       0       2     *   *     *          # Daily at 2:00 AM
0       0       23    L   *     *          # Last day of month at 11:00 PM
```

## Monitoring & Troubleshooting

### Health Checks
```bash
# Check if scheduler is running
podman ps -a --filter name=ubertool-cronjob

# View recent logs
podman logs --tail 100 ubertool-cronjob

# Search for errors
podman logs ubertool-cronjob 2>&1 | grep ERROR
```

### Common Issues

**Problem**: Jobs not executing at expected times
- **Solution**: Verify timezone: `podman exec ubertool-cronjob date`
- **Check**: Look for "All cron jobs registered successfully" in logs

**Problem**: Database connection errors
- **Solution**: Ensure cronjob container can reach postgres
- **Check**: `podman network inspect ubertool-network`

**Problem**: Email not sending
- **Solution**: Verify SMTP configuration in config.yaml
- **Test**: Run job manually with verbose logging

## Testing

### Unit Tests
```bash
# Test job logic (mock database)
go test ./internal/jobs/...
```

### Integration Tests
```bash
# Test against real database
go test ./internal/jobs/... -config=config/config.test.yaml
```

### Manual Testing
```bash
# Test individual job
make run-cronjob-once JOB=mark-overdue-rentals

# Test nightly batch
make run-cronjob-once JOB=all-nightly

# Test monthly batch
make run-cronjob-once JOB=all-monthly
```

## Database Schema

The following tables support the cronjob system:

- `balance_snapshots` - Historical balance records
- `bills` - Payment obligations from bill splitting
- `bill_actions` - Audit log for all bill-related actions
- `users_orgs.renting_blocked` - Flag to prevent renting
- `users_orgs.lending_blocked` - Flag to prevent lending

Database functions:
- `check_overdue_bills()` - Mark 10+ day old bills as disputed
- `auto_resolve_disputed_bills(org_id, settlement_month)` - Force resolution with blocks

## Migration Path

### Current (Milestone 2): Separate Container, Same Repo
- ✅ Production topology in testing
- ✅ Simple deployment (same image)
- ✅ Version consistency

### Future (Milestone 3+): Kubernetes CronJob
When moving to cloud production:
1. Convert to Kubernetes CronJob resources
2. Each job becomes separate CronJob with `--run-once` flag
3. Better observability (each execution = separate pod)
4. Built-in retry and failure tracking

Example Kubernetes migration:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: mark-overdue-rentals
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cronjob
            image: ubertool-backend:latest
            command: ["/app/cronjob"]
            args: ["--config=/config/config.yaml", "--run-once=mark-overdue-rentals"]
          restartPolicy: OnFailure
```

## Contributing

When adding new jobs:

1. Add job function to appropriate file in `internal/jobs/`
2. Register in `internal/scheduler/scheduler.go`
3. Add case in `cmd/cronjob/main.go` for manual execution
4. Update this README with job description
5. Add unit tests in `internal/jobs/*_test.go`

## References

- Roadmap: See `Roadmap-milestone.md` - Milestone 2, item 2
- Bill Splitting Requirements: `docs/design/bill-split/requirement.md`
- Database Schema: `podman/trusted-group/postgres/ubertool_schema_trusted.sql`
- Cron Library: https://github.com/robfig/cron
