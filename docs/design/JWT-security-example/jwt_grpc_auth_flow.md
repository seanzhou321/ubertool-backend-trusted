# JWT Authentication Design Document for gRPC Microservice

**Version:** 1.0  
**Date:** January 17, 2026  
**Author:** System Architecture Team  
**Status:** Design Document

---

## 7. Authentication Flow

### 7.1 Login Without 2FA Flow

```
Client                          Server
  |                               |
  |----(1) Login Request--------->|
  |    (email, password)          |
  |                               |
  |                               |---(2) Validate credentials
  |                               |
  |                               |---(3) Check 2FA status
  |                               |      (2FA disabled)
  |                               |
  |                               |---(4) Generate tokens
  |                               |      - Access Token
  |                               |      - Refresh Token
  |                               |
  |<---(5) Login Response---------|
  |    (access_token,             |
  |     refresh_token)            |
  |                               |
  |---(6) Store tokens            |
  |                               |
  |----(7) API Request----------->|
  |    Authorization: Bearer      |
  |    <access_token>             |
  |                               |
  |                               |---(8) Validate access token
  |                               |
  |<---(9) API Response-----------|
  |                               |
```

### 7.2 Login With 2FA Flow

```
Client                          Server
  |                               |
  |----(1) Login Request--------->|
  |    (email, password)          |
  |                               |
  |                               |---(2) Validate credentials
  |                               |
  |                               |---(3) Check 2FA status
  |                               |      (2FA enabled)
  |                               |
  |                               |---(4) Send 2FA code
  |                               |      (SMS/Email/TOTP)
  |                               |
  |                               |---(5) Generate 2FA token
  |                               |
  |<---(6) Login Response---------|
  |    (requires_2fa: true,       |
  |     temp_token)               |
  |                               |
  |---(7) Store 2FA token         |
  |                               |
  |----(8) Verify2FA Request----->|
  |    Authorization: Bearer      |
  |    <2fa_token>                |
  |    (code: "123456")           |
  |                               |
  |                               |---(9) Validate 2FA token
  |                               |
  |                               |---(10) Verify code
  |                               |
  |                               |---(11) Generate tokens
  |                               |       - Access Token
  |                               |       - Refresh Token
  |                               |
  |<---(12) Token Response--------|
  |     (access_token,            |
  |      refresh_token)           |
  |                               |
  |---(13) Clear 2FA token        |
  |       Store full tokens       |
  |                               |
```

### 7.3 Token Refresh Flow

```
Client                          Server
  |                               |
  |---(1) Access token expiring   |
  |                               |
  |---(2) Check token expiry      |
  |                               |
  |----(3) RefreshToken Request-->|
  |    Authorization: Bearer      |
  |    <refresh_token>            |
  |                               |
  |                               |---(4) Validate refresh token
  |                               |
  |                               |---(5) Generate new tokens
  |                               |      - New Access Token
  |                               |      - New Refresh Token
  |                               |
  |<---(6) Token Response---------|
  |    (access_token,             |
  |     refresh_token)            |
  |                               |
  |---(7) Store new tokens        |
  |                               |
  |----(8) Retry original request-|
  |    with new access token      |
  |                               |
```

### 7.4 Complete Authentication State Machine

```
┌─────────────┐
│ Unauthenti- │
│   cated     │
└─────┬───────┘
      │
      │ Login (no 2FA)
      ├──────────────────────┐
      │                      │
      │ Login (with 2FA)     │
      │                      ▼
      │              ┌───────────────┐
      │              │  2FA Pending  │
      │              │ (has 2FA token│
      │              └───────┬───────┘
      │                      │
      │                      │ Verify 2FA
      │                      │
      │                      ▼
      └─────────────►┌───────────────┐
                     │ Authenticated │
                     │ (has tokens)  │
                     └───────┬───────┘
                             │
                             │ Token expires
                             ├──────────────┐
                             │              │
                             │              ▼
                             │      ┌──────────────┐
                             │      │ Auto-refresh │
                             │      └──────┬───────┘
                             │             │
                             │◄────────────┘
                             │
                             │ Logout / Token invalid
                             ▼
                     ┌───────────────┐
                     │ Unauthenti-   │
                     │   cated       │
                     └───────────────┘
```

---

