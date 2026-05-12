# TLS Handshake Contract — Server ↔ Android Client

**Date:** May 11, 2026  
**Endpoint:** `api.ixorashare.com:50052`  
**Protocol:** gRPC over TLS 1.2/1.3 (HTTP/2)

---

## 1. Server-Side Configuration (this repo)

| Property | Value |
|---|---|
| TLS enabled | `true` (set in `deploy/ec2-mvp/config.yaml`) |
| Certificate | Let's Encrypt — issued to `api.ixorashare.com` |
| Cert path on EC2 | `/etc/ubertool/certs/fullchain.pem` |
| Key path on EC2 | `/etc/ubertool/certs/privkey.pem` |
| Cert expiry | 2026-07-13 (auto-renews via `certbot.timer`) |
| Issuer CA | Let's Encrypt R3 (ISRG Root X1 chain) |
| gRPC credential type | `credentials.NewServerTLSFromFile(certFile, keyFile)` |
| Minimum TLS version | TLS 1.2 (Go's `crypto/tls` default; TLS 1.3 preferred) |

The server presents a **standard public CA certificate**. It does **not** use self-signed certs, mTLS, or certificate pinning.

---

## 2. What the Server Expects from the Android Client

### 2.1 Channel Setup

The client MUST connect using transport security. The expected Kotlin channel setup is:

```kotlin
val channel = ManagedChannelBuilder
    .forAddress("api.ixorashare.com", 50052)
    .useTransportSecurity()   // ← REQUIRED — do NOT use usePlaintext()
    .build()
```

- `useTransportSecurity()` tells gRPC-Java/Okio to perform a standard TLS handshake using the device's system CA trust store.
- **No custom `TrustManager`** is needed or expected.
- **No certificate bundling** in the APK is needed or expected.
- **No `network_security_config.xml`** changes are needed.
- `cleartextTrafficPermitted` must NOT be set to `true`.

### 2.2 Trust Anchor

The server's cert is signed by **Let's Encrypt (ISRG Root X1)**. This CA is included in the Android system trust store for **API level 24+ (Android 7.0+)**. The client MUST target `minSdk >= 24`.

If the client runs on API level < 24, Let's Encrypt is NOT trusted by default and the connection will fail with an `SSLHandshakeException`. This is a known limitation — MVP targets API 24+.

### 2.3 Hostname Verification

The client MUST connect to the hostname `api.ixorashare.com` (not the raw IP `35.160.176.235`). The certificate's CN/SAN is bound to the hostname. Connecting by IP will fail hostname verification.

---

## 3. TLS Handshake Sequence

```
Android Client                          EC2 Server (api.ixorashare.com:50052)
     │                                            │
     │──── TCP SYN ──────────────────────────────▶│
     │◀─── TCP SYN-ACK ──────────────────────────│
     │                                            │
     │──── TLS ClientHello ──────────────────────▶│
     │     (supported cipher suites, TLS version) │
     │                                            │
     │◀─── TLS ServerHello ──────────────────────│
     │     (selected cipher suite)                │
     │                                            │
     │◀─── Certificate (fullchain.pem) ──────────│
     │     (api.ixorashare.com, Let's Encrypt)    │
     │                                            │
     │  [Client validates cert:]                  │
     │  - Chain → ISRG Root X1 (system CA store)  │
     │  - CN/SAN matches api.ixorashare.com       │
     │  - Cert not expired                        │
     │                                            │
     │──── TLS Finished (session keys agreed) ───▶│
     │                                            │
     │  [All subsequent traffic is encrypted]     │
     │                                            │
     │──── HTTP/2 SETTINGS (gRPC handshake) ─────▶│
     │◀─── HTTP/2 SETTINGS ACK ──────────────────│
     │                                            │
     │──── gRPC request (with JWT in metadata) ──▶│
     │◀─── gRPC response ────────────────────────│
```

---

## 4. Post-Handshake: JWT in gRPC Metadata

After TLS is established, every protected gRPC call must include the JWT access token in metadata:

```
Key:   authorization
Value: Bearer <access_token>
```

For refresh token calls specifically, the metadata must include **both**:

```
Key:   authorization
Value: Bearer <refresh_token>

Key:   refresh-token        ← hyphen, NOT underscore
Value: <refresh_token>
```

The server auth interceptor checks the `authorization` header first. The handler then reads `refresh-token` for the refresh endpoint.

---

## 5. What Constitutes a Successful Handshake

The server considers the TLS layer complete and hands off to gRPC when:
- A valid TLS session is established (no `SSLException` on either side)
- The client sends a valid HTTP/2 SETTINGS frame
- The first gRPC request arrives on the encrypted channel

The server does **not** validate the client's identity at the TLS layer (no mTLS). Client identity is established at the application layer via JWT.

---

## 6. Expected Failure Modes (Android Side)

| Symptom | Likely Cause |
|---|---|
| `SSLHandshakeException: Trust anchor for certification path not found` | `minSdk < 24`, or custom TrustManager rejecting Let's Encrypt |
| `SSLPeerUnverifiedException: Hostname not verified` | Client connected to raw IP `35.160.176.235` instead of `api.ixorashare.com` |
| `io.grpc.StatusException: UNAVAILABLE` | Used `usePlaintext()` instead of `useTransportSecurity()` |
| `CERTIFICATE_VERIFY_FAILED` | Cert expired (check expiry 2026-07-13; certbot should auto-renew) |
| Connection hangs indefinitely | Port 50052 blocked by Android emulator firewall or corporate network |

---

## 7. Smoke Test Verification (Server-Side)

The server-side smoke test `TestTLSConnectivity` (in `tests/smoke/`) performs a raw TLS dial to `api.ixorashare.com:50052` and verifies:
- Cert chain is valid and trusted
- CN matches the hostname
- Expiry is > 7 days out

Run via:
```
make test-smoke-ec2
```

This confirms the server is presenting a valid cert before any Android testing begins.
