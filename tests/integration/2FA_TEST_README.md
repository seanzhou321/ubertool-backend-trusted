# 2FA Integration Test

## Overview
This test demonstrates and validates the complete 2FA authentication flow:

1. **Login** with email/password → Receive 2FA token
2. **Verify 2FA** code with token → Receive access/refresh tokens
3. **Access Protected** endpoints with access token

## Prerequisites

1. **Server Running**: Ensure the backend server is running with debug logging:
   ```bash
   make run-test-debug
   ```

2. **Test User**: A user must exist in the database:
   - Email: `ubertool320@gmail.org`
   - Password: `h`
   - (This was already created and tested)

## Running the Test

### Run All 2FA Tests
```bash
go test -v ./tests/integration/... -run Test2FAFlow
```

### Run with Configuration
```bash
go test -v ./tests/integration/... -run Test2FAFlow -config=../../config/config.test.yaml
```

### Run Detailed Logging Version
```bash
go test -v ./tests/integration/... -run Test2FAFlow_WithLogs
```

## Test Steps

### Step 1: Login (Get 2FA Token)
**Request:**
```json
{
  "email": "ubertool320@gmail.org",
  "password": "h"
}
```

**Response:**
```json
{
  "success": true,
  "two_fa_token": "eyJhbGciOiJIUzI1NiIs...",
  "message": "2FA Required"
}
```

### Step 2: Verify Invalid Code (Should Fail)
**Headers:**
```
Authorization: Bearer <2fa_token>
```

**Request:**
```json
{
  "two_fa_code": "999999"
}
```

**Response:** Error - invalid 2FA code

### Step 3: Verify Without Token (Should Fail)
**Headers:** (none)

**Request:**
```json
{
  "two_fa_code": "123456"
}
```

**Response:** Error - unauthenticated

### Step 4: Verify Valid Code (Should Succeed)
**Headers:**
```
Authorization: Bearer <2fa_token>
```

**Request:**
```json
{
  "two_fa_code": "123456"
}
```

**Response:**
```json
{
  "success": true,
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

### Step 5: Access Protected Endpoint
**Headers:**
```
Authorization: Bearer <access_token>
```

**Request:**
```json
{}
```

**Response:** User profile data

## Debug Logs to Watch For

When running the test, you'll see these key log entries in the server output:

### Login Phase
```json
{"level":"info","msg":"=== API RequestToJoinOrganization called ==="}
{"level":"info","msg":"Password validated successfully","userID":4}
{"level":"debug","msg":"Generating 2FA token","userID":4}
{"level":"info","msg":"2FA code for testing (HARDCODED)","code":"123456"}
```

### Verify2FA Phase
```json
{"level":"info","msg":"=== API Verify2FA called ===","codeProvided":"123456"}
{"level":"debug","msg":"Extracting userID from context (2FA token)"}
{"level":"info","msg":"UserID extracted from 2FA token","userID":4}
{"level":"debug","msg":"Validating 2FA code","providedCode":"123456","expectedCode":"123456"}
{"level":"info","msg":"2FA code validated successfully","userID":4}
{"level":"info","msg":"=== API Verify2FA completed successfully ==="}
```

### Interceptor Logs
```json
{"level":"debug","msg":"Auth interceptor processing request","method":"/ubertool.v1.AuthService/Verify2FA"}
{"level":"debug","msg":"Token extracted","tokenPrefix":"eyJhbGciOiJIUzI1NiIs"}
{"level":"info","msg":"Token validated successfully","userID":4,"tokenType":"2fa_pending"}
{"level":"debug","msg":"Security level check passed"}
{"level":"debug","msg":"User ID injected into context","userID":4}
```

## Common Issues

### Issue: "metadata is not provided"
**Cause:** 2FA token not included in Authorization header
**Solution:** Ensure the test adds the token to metadata:
```go
md := metadata.New(map[string]string{
    "authorization": "Bearer " + twoFAToken,
})
ctx = metadata.NewOutgoingContext(ctx, md)
```

### Issue: "invalid 2FA code"
**Cause:** Code is not "123456" (hardcoded)
**Solution:** Use the exact code "123456" until random code generation is implemented

### Issue: "2fa pending token required"
**Cause:** Wrong token type used (access token instead of 2FA token)
**Solution:** Use the token from Login response, not from Verify2FA response

### Issue: "CRITICAL: Failed to get userID from context"
**Cause:** Interceptor couldn't extract userID from token
**Solution:** Check that the 2FA token is valid and not expired (10 minute expiry)

## Expected Output

```
=== RUN   Test2FAFlow
=== RUN   Test2FAFlow/Step1_Login_Should_Return_2FA_Token
    2fa_test.go:38: Testing login for: ubertool320@gmail.org
    2fa_test.go:49: Login Response: Success=true, Message=2FA Required
    2fa_test.go:55: ✅ 2FA Token received (length: 287)
    2fa_test.go:56:    Token prefix: eyJhbGciOiJIUzI1NiIsInR5cCI...
=== RUN   Test2FAFlow/Step2_Verify2FA_With_Invalid_Code_Should_Fail
    2fa_test.go:77: Testing 2FA verification with INVALID code
    2fa_test.go:90: ❌ Expected error received: rpc error: code = Unknown desc = invalid 2fa code
=== RUN   Test2FAFlow/Step3_Verify2FA_Without_Token_Should_Fail
    2fa_test.go:99: Testing 2FA verification WITHOUT 2FA token in header
    2fa_test.go:113: ❌ Expected error received: rpc error: code = Unauthenticated desc = metadata is not provided
=== RUN   Test2FAFlow/Step4_Verify2FA_With_Valid_Code_Should_Succeed
    2fa_test.go:125: Testing 2FA verification with VALID code: 123456
    2fa_test.go:126: Authorization header includes 2FA token
    2fa_test.go:139: ✅ 2FA Verification SUCCESSFUL!
    2fa_test.go:140:    Access Token (prefix): eyJhbGciOiJIUzI1NiIsInR5cCI...
    2fa_test.go:141:    Refresh Token (prefix): eyJhbGciOiJIUzI1NiIsInR5cCI...
=== RUN   Test2FAFlow/Step5_Use_Access_Token_To_Call_Protected_Endpoint
    2fa_test.go:154: Testing protected endpoint with access token
    2fa_test.go:163: ✅ Protected endpoint access SUCCESSFUL!
    2fa_test.go:165:    User: admin (ubertool320@gmail.org)
--- PASS: Test2FAFlow (0.15s)
    --- PASS: Test2FAFlow/Step1_Login_Should_Return_2FA_Token (0.03s)
    --- PASS: Test2FAFlow/Step2_Verify2FA_With_Invalid_Code_Should_Fail (0.02s)
    --- PASS: Test2FAFlow/Step3_Verify2FA_Without_Token_Should_Fail (0.01s)
    --- PASS: Test2FAFlow/Step4_Verify2FA_With_Valid_Code_Should_Succeed (0.04s)
    --- PASS: Test2FAFlow/Step5_Use_Access_Token_To_Call_Protected_Endpoint (0.05s)
PASS
```

## Notes

- The 2FA code is currently **hardcoded to "123456"**
- The 2FA token expires in **10 minutes**
- Access tokens expire in **1 hour**
- Refresh tokens expire in **7 days**
- All logs are in **JSON format** for easy parsing
