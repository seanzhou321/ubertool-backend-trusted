# End-to-End (E2E) Tests

This directory contains comprehensive end-to-end tests that verify the complete flow from gRPC API handlers through business logic services to the database layer.

## Prerequisites

1. **PostgreSQL Database**: The tests require a running PostgreSQL instance
   - Default connection: `localhost:5454`
   - Database: `ubertool_db`
   - User: `ubertool_trusted`
   - Password: `ubertool123`

2. **gRPC Server**: Tests require a running gRPC server
   - Default address: `localhost:50051`
   - Start the server with: `go run ./cmd/server`

## Test Structure

### Helper Utilities (`helpers.go`)
- `PrepareDB()`: Database connection and setup
- `TestDB`: Wrapper with helper methods for test data creation
- `GRPCClient`: gRPC client connection management
- Context helpers for user authentication

### Test Files

#### `auth_test.go` - Authentication Service
- ✅ Signup with valid invitation
- ✅ Request to join organization with admin notifications
- ✅ Login flow

#### `org_test.go` - Organization Service
- ✅ Create organization with SUPER_ADMIN assignment
- ✅ List my organizations with user-specific data
- ✅ Search organizations by metro

#### `tool_test.go` - Tool Service
- ✅ Add tool workflow
- ✅ Search tools with organization membership verification
- ✅ List my tools
- ✅ Update tool
- ✅ Delete tool (soft delete)

#### `rental_test.go` - Rental Service
- ✅ Full rental lifecycle (create → approve → finalize → complete)
- ✅ Reject rental request
- ✅ Cancel rental request
- ✅ Create rental with insufficient balance (error case)
- ✅ Balance debit on finalization
- ✅ Balance credit on completion
- ✅ Tool status updates (AVAILABLE → RENTED → AVAILABLE)
- ✅ Notifications at each step

#### `admin_test.go` - Admin Service
- ✅ Approve join request for existing user
- ✅ Approve join request for new user (sends invitation)
- ✅ Block user with reason and date
- ✅ List members
- ✅ List join requests

#### `ledger_test.go` - Ledger Service
- ✅ Get balance
- ✅ Get transactions with pagination
- ✅ Get ledger summary with rental counts
- ✅ Ledger updates through rental workflow

#### `user_test.go` - User Service
- ✅ Get user profile with organizations
- ✅ Update profile (name, email, phone, avatar)
- ✅ Get user with no organizations
- ✅ Email uniqueness validation

#### `notification_test.go` - Notification Service
- ✅ Get notifications with pagination
- ✅ Mark notification as read
- ✅ Authorization (user can only access own notifications)
- ✅ Notifications created through rental workflow

#### `image_storage_test.go` - Image Storage Service
- ✅ Upload image with streaming
- ✅ Download image with streaming
- ✅ Download thumbnail
- ✅ Get tool images
- ✅ Set primary image
- ✅ Delete image (soft delete)
- ✅ Authorization (only tool owner can upload/delete)

## Business Logic Coverage

All business logic defined in `docs/design/business_logic.md` is tested:

### Authentication
- ✅ Validate invite
- ✅ Request to join organization
- ✅ User signup
- ✅ Login
- ✅ Verify 2FA

### Administration
- ✅ Approve request to join
- ✅ Block user account
- ✅ List members
- ✅ Search users
- ✅ List join requests

### Organizations
- ✅ List my organizations
- ✅ Get organization
- ✅ Create organization
- ✅ Search organizations

### Tools
- ✅ List my tools
- ✅ Get tool
- ✅ Add tool
- ✅ Update tool
- ✅ Delete tool
- ✅ Search tools (with org membership verification)

### Rentals
- ✅ Create rental request (with balance check)
- ✅ Approve rental request
- ✅ Reject rental request
- ✅ Finalize rental request (payment/debit)
- ✅ Cancel rental request
- ✅ Complete rental (credit owner)
- ✅ Inclusive date calculation
- ✅ Tool status management

### Ledger
- ✅ Get balance
- ✅ Get transactions
- ✅ Get ledger summary
- ✅ Transaction creation through rental workflow

### Users
- ✅ Get user profile
- ✅ Update profile

### Notifications
- ✅ Get notifications
- ✅ Mark notification read

### Image Storage
- ✅ Upload image
- ✅ Download image
- ✅ Get tool images
- ✅ Set primary image
- ✅ Delete image

## Running the Tests

### Run All E2E Tests
```bash
make test-e2e
```

### Run Specific Test File
```bash
go test -v ./tests/e2e/auth_test.go ./tests/e2e/helpers.go
go test -v ./tests/e2e/rental_test.go ./tests/e2e/helpers.go
```

### Run Specific Test Case
```bash
go test -v ./tests/e2e -run TestRentalService_E2E/Full_Rental_Lifecycle
```

## Test Data Management

All test data uses the `e2e-test-` prefix for emails and `E2E-Test-` prefix for organization names. The `TestDB.Cleanup()` method removes all test data after each test run.

## Notes

- Tests use gRPC metadata to simulate authenticated requests (`user-id` header)
- Database transactions are verified directly via SQL queries
- Notifications and email triggers are verified through database records
- Tests are designed to be idempotent and can run in parallel (with proper cleanup)

## Troubleshooting

**Database connection failed**: Ensure PostgreSQL is running on port 5454
```bash
cd podman/trusted-group/postgres
.\install.ps1
```

**gRPC connection failed**: Ensure the server is running
```bash
go run ./cmd/server
```

**Tests fail with "user not found"**: Ensure cleanup is running properly and test data prefixes are correct
