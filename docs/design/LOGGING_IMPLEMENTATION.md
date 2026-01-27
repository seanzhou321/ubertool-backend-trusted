# Debug Logging Implementation

## Overview
Comprehensive debug logging has been implemented to track application actions during testing, specifically for diagnosing the join request notification issue.

## Configuration

### Test Configuration (`config/config.test.yaml`)
```yaml
log:
  level: "debug"  # debug, info, warn, error
  format: "json"  # json format for telemetry integration
```

### Environment Variables
You can override log settings via environment variables:
- `LOG_LEVEL`: debug, info, warn, error
- `LOG_FORMAT`: text, json

## Running Tests with Debug Mode

### Method 1: Using Makefile
```bash
make run-test        # Starts server with debug logging enabled
make run-test-debug  # Explicitly sets debug level via env var
```

### Method 2: Direct Command
```bash
go run ./cmd/server -config=config/config.test.yaml
```

### Method 3: Override via Environment
```bash
set LOG_LEVEL=debug
set LOG_FORMAT=json
go run ./cmd/server -config=config/config.test.yaml
```

## Logging Categories

### 1. Process Tracking Logs (Method Entry/Exit)
These logs track when methods are entered and exited:

**Example Output:**
```json
{"level":"debug","method":"authService.RequestToJoin","event":"enter","orgID":1,"name":"John Doe","email":"john@example.com","msg":"→ Method entered"}
{"level":"debug","method":"authService.RequestToJoin","event":"exit","requestID":123,"msg":"← Method exited"}
```

**Logged Methods:**
- `AuthHandler.RequestToJoinOrganization` - gRPC API entry point
- `authService.RequestToJoin` - Service layer logic
- `joinRequestRepository.Create` - Database join request creation
- `userRepository.ListMembersByOrg` - Fetching admin users
- `notificationRepository.Create` - **Critical for notification issue**

### 2. Debug Logs for External Resources

#### Database Calls
Logged before and after each database operation:

**Example Output:**
```json
{"level":"debug","operation":"INSERT","query":"notifications (user_id, org_id, title, message, is_read, attributes)","userID":5,"orgID":1,"msg":"→ Database call"}
{"level":"debug","operation":"INSERT","rows_affected":1,"notificationID":42,"msg":"← Database call succeeded"}
```

**OR if failed:**
```json
{"level":"error","operation":"INSERT","rows_affected":0,"error":"pq: duplicate key value violates unique constraint","msg":"← Database call failed"}
```

#### External Service Calls
Email service and other external calls:

**Example Output:**
```json
{"level":"debug","service":"email","operation":"SendAdminNotification","to":"admin@example.com","subject":"New Join Request","msg":"→ External service call"}
{"level":"debug","service":"email","operation":"SendAdminNotification","msg":"← External service call succeeded"}
```

### 3. Business Logic Information Logs
Key business events and summary information:

**Example Output:**
```json
{"level":"info","msg":"=== API RequestToJoinOrganization called ===","organizationID":1,"name":"John Doe","email":"john@example.com"}
{"level":"info","msg":"Join request created successfully","requestID":123,"orgID":1,"email":"john@example.com"}
{"level":"info","msg":"Notification created successfully","adminID":5,"notificationID":42}
{"level":"info","msg":"Join request processing completed","requestID":123,"adminsFound":3,"notificationsSent":3,"notificationsFailed":0}
{"level":"info","msg":"=== API RequestToJoinOrganization completed successfully ===","organizationID":1}
```

### 4. Error Logs
Critical errors with full context:

**Example Output:**
```json
{"level":"error","msg":"CRITICAL: Failed to create notification for admin","adminID":5,"adminEmail":"admin@example.com","error":"connection timeout"}
{"level":"error","method":"authService.RequestToJoin","event":"exit","error":"failed to list admins","msg":"← Method exited with error"}
```

## Tracking the Notification Issue

When a user sends a join request, you'll see this flow in the logs:

1. **API Entry**: `=== API RequestToJoinOrganization called ===`
2. **Organization Lookup**: `Fetching organization` → `Organization found`
3. **User Lookup**: `Searching for user by email` → `User found` or `User not found`
4. **Join Request Creation**: `Creating join request` → `Join request created successfully`
5. **Admin User Discovery**: `Fetching admin users` → `Members retrieved` with count
6. **For Each Admin**:
   - `Processing admin notification` with admin details
   - `→ External service call` for email
   - `← External service call succeeded/failed`
   - `Creating notification for admin`
   - `→ Database call` to notifications table
   - **← Database call succeeded** OR **CRITICAL: Failed to create notification**
7. **Summary**: `Join request processing completed` with counts
8. **API Exit**: `=== API RequestToJoinOrganization completed successfully ===`

## Key Features

### Structured JSON Logging
All logs are in JSON format for easy parsing by telemetry tools like:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Splunk
- DataDog
- New Relic
- CloudWatch Logs Insights

### Context Preservation
Each log entry includes relevant context:
- Method names
- Entity IDs (userID, orgID, requestID, notificationID)
- Operation types
- Error details
- Timestamps (automatic)

### Performance Impact
Debug logging has minimal performance impact:
- Only active when `log.level: "debug"` is set
- Structured logging is efficient
- No blocking I/O operations

## Log Levels

- **DEBUG**: Detailed flow tracking, method entry/exit, database calls
- **INFO**: Business events, successful operations, summaries
- **WARN**: Recoverable errors, best-effort operations that failed
- **ERROR**: Critical failures, unrecoverable errors

## Troubleshooting the Notification Issue

If notifications are not being created, check the logs for:

1. **Admin count**: Look for `"adminsFound"` in the summary log
   - If 0, no admins exist for the organization
   
2. **Database errors**: Look for `"CRITICAL: Failed to create notification"`
   - Check the `"error"` field for the specific database error
   - Common issues: foreign key violations, column constraints, connection timeouts

3. **Loop execution**: Count how many times `"Processing admin notification"` appears
   - Should match `"adminsFound"` count

4. **Role filtering**: Verify admins have correct roles (ADMIN or SUPER_ADMIN)
   - Check in `"Processing admin notification"` logs for `"role"` field

5. **Transaction issues**: Check if any prior database operations failed

## Example Complete Log Trace

```json
{"time":"2026-01-26T10:30:45Z","level":"info","msg":"=== API RequestToJoinOrganization called ===","organizationID":1,"name":"Jane Smith","email":"jane@example.com"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"AuthHandler.RequestToJoinOrganization","event":"enter","organizationID":1,"email":"jane@example.com","msg":"→ Method entered"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"authService.RequestToJoin","event":"enter","orgID":1,"name":"Jane Smith","email":"jane@example.com","msg":"→ Method entered"}
{"time":"2026-01-26T10:30:45Z","level":"debug","orgID":1,"msg":"Fetching organization"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"SELECT","query":"orgs WHERE id = $1","msg":"→ Database call"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"SELECT","rows_affected":1,"orgID":1,"msg":"← Database call succeeded"}
{"time":"2026-01-26T10:30:45Z","level":"debug","orgID":1,"orgName":"North Parish Church","msg":"Organization found"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"joinRequestRepository.Create","event":"enter","orgID":1,"email":"jane@example.com","msg":"→ Method entered"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"INSERT","query":"join_requests","msg":"→ Database call"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"INSERT","rows_affected":1,"requestID":456,"msg":"← Database call succeeded"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"joinRequestRepository.Create","event":"exit","requestID":456,"msg":"← Method exited"}
{"time":"2026-01-26T10:30:45Z","level":"info","requestID":456,"orgID":1,"email":"jane@example.com","msg":"Join request created successfully"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"userRepository.ListMembersByOrg","event":"enter","orgID":1,"msg":"→ Method entered"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"SELECT","query":"users JOIN users_orgs","msg":"→ Database call"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"SELECT","rows_affected":2,"membersFound":2,"msg":"← Database call succeeded"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"userRepository.ListMembersByOrg","event":"exit","count":2,"msg":"← Method exited"}
{"time":"2026-01-26T10:30:45Z","level":"debug","adminID":10,"adminEmail":"admin@church.org","role":"ADMIN","msg":"Processing admin notification"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"notificationRepository.Create","event":"enter","userID":10,"orgID":1,"title":"New Join Request","msg":"→ Method entered"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"INSERT","query":"notifications","userID":10,"orgID":1,"msg":"→ Database call"}
{"time":"2026-01-26T10:30:45Z","level":"debug","operation":"INSERT","rows_affected":1,"notificationID":789,"msg":"← Database call succeeded"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"notificationRepository.Create","event":"exit","notificationID":789,"msg":"← Method exited"}
{"time":"2026-01-26T10:30:45Z","level":"info","adminID":10,"notificationID":789,"msg":"Notification created successfully"}
{"time":"2026-01-26T10:30:45Z","level":"info","requestID":456,"adminsFound":1,"notificationsSent":1,"notificationsFailed":0,"msg":"Join request processing completed"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"authService.RequestToJoin","event":"exit","requestID":456,"msg":"← Method exited"}
{"time":"2026-01-26T10:30:45Z","level":"debug","method":"AuthHandler.RequestToJoinOrganization","event":"exit","organizationID":1,"msg":"← Method exited"}
{"time":"2026-01-26T10:30:45Z","level":"info","msg":"=== API RequestToJoinOrganization completed successfully ===","organizationID":1}
```

## Additional Notes

- Previous silent error: `_ = s.noteRepo.Create(ctx, notif)` has been replaced with full error handling and logging
- All critical operations now log errors instead of silently ignoring them
- Notification creation failures are now tracked and counted
- Summary logs provide at-a-glance status of operations
