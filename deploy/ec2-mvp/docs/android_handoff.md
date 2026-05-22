# Android MVP Release - Handoff Notes
# Date: April 14, 2026
# Purpose: EC2 backend configuration for Android app MVP release to Google Play

## Backend Status
The gRPC backend is fully deployed and smoke-tested. The Android app can connect
to the live production endpoint immediately.

## gRPC Endpoint
- Host : api.marigoldshare.marigoldintelligence.us
- Port : 50052
- TLS  : enabled (Let's Encrypt — trusted natively on Android, no cert bundling needed)

## Android Channel Configuration
```kotlin
val channel = ManagedChannelBuilder
    .forAddress("api.marigoldshare.marigoldintelligence.us", 50052)
    .useTransportSecurity()
    .build()
```
No custom `TrustManager` or CA certificate is required. Let's Encrypt is in the
Android system trust store for API level 24+ (Android 7.0+).

## Proto Package
All service stubs are under: `com.ubertool.trusted.api.v1`

## TLS Certificate Details
- Issued by  : Let's Encrypt
- Common name: api.marigoldshare.marigoldintelligence.us
- Expires    : 2026-07-13 (auto-renews via certbot.timer on EC2)
- No pinning required for MVP; standard system trust store validation is sufficient

## Network Security Config
No `network_security_config.xml` changes needed — Let's Encrypt is trusted by the
Android system CA store out of the box. Do NOT add `cleartextTrafficPermitted` or
custom trust anchors; the endpoint is TLS-only.

## Android Paths (local dev machine)
- Android project: H:\github\projects\ubertool\ubertool-android-trusted
- Backend root   : C:\Users\yixio\ubertool\ubertool-backend-trusted

## Firebase / Push Notifications
- The FCM service account key is deployed to EC2 at `/etc/ubertool/firebase-admin-key.json`.
- Push notifications are fully enabled for MVP.

## Google Play Release Checklist (for Android AI agent)
The following are the backend-relevant prerequisites — all complete:
- [x] gRPC endpoint live: api.marigoldshare.marigoldintelligence.us:50052
- [x] TLS certificate valid and auto-renewing
- [x] Smoke tests passing (make test-smoke-ec2 in backend repo)
- [x] Google Play Developer account registered ($25 one-time fee paid)

The Android repository AI should handle:
- Release build signing (keystore, upload key)
- build.gradle / proguard configuration
- Google Play Console submission (AAB, store listing, screenshots, content rating)
- Internal track → production promotion workflow
