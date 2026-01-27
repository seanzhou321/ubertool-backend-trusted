# 2FA Authentication Header Specification

## Overview
The backend uses a standard JWT-based 2FA authentication flow. The client must send the appropriate JWT token in the **Authorization** header for different authentication stages.

## Expected Header Format

### Standard Format
```
Authorization: Bearer <JWT_TOKEN>
```

**Important Notes:**
- Header name MUST be: `authorization` (lowercase in gRPC metadata)
- Format MUST include "Bearer " prefix (case-insensitive)
- The JWT token is the raw token string without additional encoding

### What NOT to Use
❌ Custom header names like `two_fa_token`
❌ Sending token without "Bearer " prefix
❌ Sending token in request body instead of header

---

## Authentication Flow

### Step 1: Login
**Endpoint:** `/ubertool.trusted.api.v1.AuthService/Login`

**Request:**
- Headers: None required (public endpoint)
- Body:
  ```protobuf
  {
    "email": "user@example.com",
    "password": "userpassword"
  }
  ```

**Response:**
```protobuf
{
  "success": true,
  "two_fa_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "message": "2FA Required"
}
```

**What happens:**
- Backend validates email/password
- Backend generates a **2FA JWT token** (type: `2fa_pending`)
- Backend sends hardcoded 2FA code "123456" via email
- Client receives the 2FA JWT token in the response body

---

### Step 2: Verify 2FA Code
**Endpoint:** `/ubertool.trusted.api.v1.AuthService/Verify2FA`

**Request:**
- **Headers (REQUIRED):**
  ```
  Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  ```
  ☝️ This MUST be the `two_fa_token` received from Step 1

- **Body:**
  ```protobuf
  {
    "two_fa_code": "123456"
  }
  ```

**Response:**
```protobuf
{
  "success": true,
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**What happens:**
1. Backend extracts JWT from `Authorization` header
2. Backend validates the JWT token (must be type `2fa_pending`)
3. Backend extracts `user_id` from the JWT
4. Backend verifies the 2FA code matches "123456"
5. Backend generates and returns access/refresh tokens

---

## Token Types & Usage

| Token Type | Received From | Used For | Sent In Header |
|------------|---------------|----------|----------------|
| **2FA Token** | Login response (`two_fa_token` field) | Verify2FA endpoint | `Authorization: Bearer <2fa_token>` |
| **Access Token** | Verify2FA response (`access_token` field) | All protected endpoints | `Authorization: Bearer <access_token>` |
| **Refresh Token** | Verify2FA response (`refresh_token` field) | RefreshToken endpoint | `refresh-token` header |

---

## gRPC Metadata Example (Android/Kotlin)

```kotlin
// Step 1: Login (no auth required)
val loginResponse = authServiceStub.login(
    LoginRequest.newBuilder()
        .setEmail(email)
        .setPassword(password)
        .build()
)

// Save the 2FA token from response
val twoFaToken = loginResponse.twoFaToken

// Step 2: Verify 2FA with token in header
val metadata = Metadata()
metadata.put(
    Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER),
    "Bearer $twoFaToken"
)

val stubWithAuth = MetadataUtils.attachHeaders(authServiceStub, metadata)

val verifyResponse = stubWithAuth.verify2FA(
    Verify2FARequest.newBuilder()
        .setTwoFaCode("123456")
        .build()
)

// Save access and refresh tokens
val accessToken = verifyResponse.accessToken
val refreshToken = verifyResponse.refreshToken
```

---

## Common Issues

### Issue 1: "authorization token is not provided"
**Cause:** Client is not sending the Authorization header
**Fix:** Add `Authorization: Bearer <token>` to request headers

### Issue 2: Client sending custom header `two_fa_token`
**Cause:** Misunderstanding - the response field name is not the header name
**Fix:** 
- Response field: `two_fa_token` (contains the JWT)
- Request header: `Authorization: Bearer <value_of_two_fa_token>`

### Issue 3: "2fa pending token required"
**Cause:** Sending wrong token type (e.g., access token instead of 2FA token)
**Fix:** Ensure you're sending the JWT received from Login, not from a previous Verify2FA

### Issue 4: "invalid 2fa code"
**Cause:** Wrong verification code sent in body
**Fix:** Currently hardcoded to "123456" - send exactly this value

---

## Backend Validation Logic

```go
// 1. Extract token from Authorization header
token := extractFromHeader("authorization")  // Must be "authorization"

// 2. Remove "Bearer " prefix if present
if strings.HasPrefix(token, "Bearer ") {
    token = token[7:]
}

// 3. Validate JWT structure and signature
claims := validateJWT(token)

// 4. Check token type
if claims.Type != "2fa_pending" {
    return error("2fa pending token required")
}

// 5. Extract user_id from JWT claims
userID := claims.UserID

// 6. Verify 2FA code from request body
if request.TwoFaCode != "123456" {
    return error("invalid 2fa code")
}

// 7. Generate access & refresh tokens
accessToken := generateAccessToken(userID)
refreshToken := generateRefreshToken(userID)

return {accessToken, refreshToken}
```

---

## Summary for Client Implementation

✅ **DO:**
- Send 2FA token in `Authorization` header with "Bearer " prefix
- Use the exact JWT string from login response's `two_fa_token` field
- Send 2FA code "123456" in request body

❌ **DON'T:**
- Create custom headers like `two_fa_token`
- Send token without "Bearer " prefix
- Confuse response field names with header names
- Send wrong token type (access token for 2FA verification)
