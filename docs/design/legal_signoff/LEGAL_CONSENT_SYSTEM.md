# Legal Consent System — Design Specification

## Overview

This document covers:
1. Answers to the 4 design questions raised during session
2. Android client implementation (this session — see code)
3. PostgreSQL schema additions (backend — future sprint)
4. gRPC API additions (backend — future sprint)

---

## Q1 — Duplicate: in-app vs. hosted privacy docs?

**Not duplicates — they serve different audiences and different legal purposes.**

| Artifact | Location | Purpose |
|---|---|---|
| Markdown source | `app/src/main/assets/legals/*.md` | Offline fallback + consent-version snapshot inside app |
| HTML web page | `docs/legal/s3-privacy-policy/privacy.html` (→ S3) | Play Console required URL, public facing, Google indexable |
| Raw markdown on S3 | same S3 bucket, `.md` extension | App web-first loader fetches this for in-app rendering |

**Rule:** The 8 markdown files in `assets/legals/` are the single source of truth. The HTML file for S3 is derived from `02_privacy_policy.md`. Keep them in sync manually (or automate with a Pandoc build step).

---

## Q2 — URL structure for hosted docs

Host all legal docs at:

```
https://lantanaintelligence.com/lantanashare/legal/
```

| Doc | URL |
|---|---|
| User Agreement | `/lantanashare/legal/user-agreement.md` |
| Terms of Service | `/lantanashare/legal/terms-of-service.md` |
| Privacy Policy | `/lantanashare/legal/privacy-policy.md` |
| Cookie Policy | `/lantanashare/legal/cookie-policy.md` |
| Community Bylaws | `/lantanashare/legal/community-bylaws.md` |
| Liability Waiver | `/lantanashare/legal/liability-waiver.md` |
| Acceptable Use | `/lantanashare/legal/acceptable-use-policy.md` |
| EULA | `/lantanashare/legal/eula.md` |
| Privacy (HTML) | `/lantanashare/legal/privacy-policy.html` ← Play Console URL |

**S3 setup (separate task):**
1. Create bucket `lantanaintelligence-legal`
2. Enable static website hosting
3. Upload files from `docs/legal/`
4. Set Namecheap CNAME: `legal.lantanaintelligence.com` → S3 bucket URL
5. (Optional) CloudFront for HTTPS

---

## Q3 — Legal Sign-off Mechanism

### Android side (implemented this session)

**Consent version constant:** `LegalConsentViewModel.CURRENT_LEGAL_VERSION = "2026-04-15"`

**Signup flow:**
- User must check consent checkboxes in signup form before "Create Account" is enabled
- On signup success, `tokenManager.saveLegalConsentVersion("2026-04-15")` is called
- Stored in DataStore `"user_preferences"` file

**Login/re-consent flow:**
- After successful Verify2FA, navigate to `Screen.LegalConsent`
- `LegalConsentScreen` reads stored version from DataStore
  - If `storedVersion == CURRENT_LEGAL_VERSION` → auto-navigate to Dashboard (no visible dialog)
  - If different → show full re-consent screen with all 8 docs
- User must scroll through and tap "I Accept" per document group
- On accept, version saved and user proceeds to Dashboard

**Version bump process (when docs change):**
1. Update the markdown files in `assets/legals/`
2. Update `LegalConsentViewModel.CURRENT_LEGAL_VERSION` to the new date
3. Ship new app build → users see re-consent screen on next open

---

### PostgreSQL schema additions (backend — future sprint)

Add after the existing `users` table:

```sql
-- Record of each user's acceptance of each document version.
-- doc_name = filename without .md extension, e.g. '00_user_agreement_and_consent'
-- version  = ISO date string matching CURRENT_LEGAL_VERSION, e.g. '2026-04-15'
CREATE TABLE user_legal_consents (
    user_id      INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    doc_name     TEXT        NOT NULL,
    version      TEXT        NOT NULL,
    consented_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, doc_name, version)
);
```

**No seed data required.** Rows are inserted when users consent; the table starts empty.

**To query whether a user has consented to a specific doc version:**
```sql
SELECT EXISTS (
    SELECT 1 FROM user_legal_consents
    WHERE user_id = $1 AND doc_name = $2 AND version = $3
);
```

**To query all consented docs for a user (audit trail):**
```sql
SELECT doc_name, version, consented_at
FROM user_legal_consents
WHERE user_id = $1
ORDER BY doc_name, version;
```

---

### gRPC API additions (backend — future sprint)

Add to `auth_service.proto`:

```protobuf
// Record user consent for a set of document versions
// access_token required (called after account exists or after re-consent)
rpc RecordLegalConsent(RecordLegalConsentRequest) returns (VanilaResponse);

// Check if user has consented to all docs at the current version
// access_token required
rpc GetUserConsentStatus(GetUserConsentStatusRequest) returns (GetUserConsentStatusResponse);
```

Add proto messages to `auth_service.proto`:

```protobufmessage RecordLegalConsentRequest {
  repeated string doc_names = 1;  // filenames without .md, e.g. "00_user_agreement_and_consent"
  string          version   = 2;  // ISO date, matches CURRENT_LEGAL_VERSION
}

message GetUserConsentStatusRequest {
  string current_version = 1;  // CURRENT_LEGAL_VERSION from the app build
}

message GetUserConsentStatusResponse {
  bool            all_current  = 1;  // false = at least one doc not yet consented
  repeated string pending_docs = 2;  // doc_names missing consent for current_version
}
```

**Backend business logic:**
- `RecordLegalConsent`: `INSERT INTO user_legal_consents (user_id, doc_name, version) VALUES (...) ON CONFLICT DO NOTHING`
- `GetUserConsentStatus`: for each doc_name in the known set, check if a row exists with the given version for the current user

---

## Q4 — Web-first loading with offline fallback

Implemented in `LegalDocumentScreen.kt`:
- Accepts optional `webUrl: String?` parameter
- If `webUrl != null`: OkHttp GET to fetch raw markdown text (2-second timeout)
- On success: renders fetched markdown using existing `MarkdownLine` renderer
- On failure / timeout / offline: silently falls back to `assets/legals/{assetPath}`
- Loading indicator shown while fetching

**URL pattern (for future web-first loading):**
```kotlin
const val LEGAL_BASE_URL = "https://lantanaintelligence.com/lantanashare/legal/"
// Access via: LEGAL_BASE_URL + "privacy-policy.md"
```

The `ProfileNavGraph` already passes `webUrl = null`; update it to pass the real URL
after the S3 bucket is set up.
