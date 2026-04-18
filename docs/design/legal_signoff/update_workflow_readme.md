# Legal Document Update Workflow

This document describes the process for updating legal documents in LantanaShare.

---

## Key Concepts

| Term | Meaning |
|---|---|
| **Last Updated** | The date the doc content was last touched. Updated for both minor and major changes. |
| **Effective Date** | The date the new terms legally bind users. Only changes on major updates. Used as the version identifier. |
| `CURRENT_LEGAL_VERSION` | ISO-8601 date string in `LegalConsentViewModel.kt`. Equals the **Effective Date**. Only bumped on major changes. |

### Blocking behaviour

The app compares the user's stored consent version with `CURRENT_LEGAL_VERSION`:

| Condition | Result |
|---|---|
| Stored version == `CURRENT_LEGAL_VERSION` | No action — proceed to Dashboard |
| Versions differ AND today ≥ effective date | **Block** — user must re-consent before proceeding |
| Versions differ AND today < effective date | No block — new docs are live and readable in Settings > Legal, but consent is not yet required |

---

## Deciding the Type of Change

**Major change — bump `CURRENT_LEGAL_VERSION` and set a new Effective Date:**
- Any change to the meaning or legal effect of a doc
- New categories of data collected or shared
- Introduction of advertising or third-party tracking
- New biometric or sensitive data processing
- Changes to arbitration or dispute resolution terms
- Changes to payment or liability terms
- Contact details or company name changes

**Minor change — update `Last Updated` only, no version bump:**
- Typo or grammar fixes with no change in meaning
- Formatting changes only

---

## Workflow A — Minor Change

1. Edit the `.md` files on the website at `lantanaintelligence.com/lantanashare/legal/`.
2. In every edited doc, update `**Last Updated:**` to today. Leave `**Effective Date:**` unchanged.
   Also update the "last updated on" footer line.
3. Deploy the updated `.md` files to the website.
4. Update and redeploy `privacy.html` if the privacy policy changed.
5. No app build required. `CURRENT_LEGAL_VERSION` is not bumped. No re-consent is triggered.

---

## Workflow B — Major Change

1. Edit the `.md` files with the new content. Set the date fields to:
   ```
   **Last Updated:** <today>
   **Effective Date:** <effective date — today or a future date>
   ```
   Also update the "last updated on" footer line in each edited doc.
2. Deploy the updated `.md` files to the website at
   `lantanaintelligence.com/lantanashare/legal/`.
3. In `LegalConsentViewModel.kt`, set the constant to the effective date:
   ```kotlin
   const val CURRENT_LEGAL_VERSION = "YYYY-MM-DD"  // effective date
   ```
4. Update and redeploy `privacy.html` if the privacy policy changed.
5. Ship a new app build.
   - Users who log in **before** the effective date: new docs are visible in
     Settings > Legal but access is not blocked.
   - Users who log in **on or after** the effective date: blocked at login and
     must tap "I Accept" before reaching the Dashboard.
6. If the feature that depends on the new terms is ready, ship it on or after
   the effective date.

---

## Version Bump Checklist

For **major changes** only:

- [ ] Content changes made in all affected `.md` files in `assets/legals/`
- [ ] `**Last Updated:**` set to today in all edited docs
- [ ] `**Effective Date:**` set to the intended effective date in all edited docs
- [ ] "last updated on" footer line updated in all edited docs
- [ ] `CURRENT_LEGAL_VERSION` in `LegalConsentViewModel.kt` updated to the effective date
- [ ] Updated `.md` files deployed to `lantanaintelligence.com/lantanashare/legal/`
- [ ] `privacy.html` updated and redeployed (if privacy policy changed)
- [ ] New app build shipped

For **minor changes**, only deploy updated `.md` files to the website. No build required.

---

## Where Each Date Appears

| Location | Field | Purpose |
|---|---|---|
| `.md` doc headers | `**Last Updated:**` | Informational — changes on every edit (minor or major) |
| `.md` doc headers | `**Effective Date:**` | Legal — the date the terms bind users; changes on major updates only |
| `.md` doc footers | "last updated on …" | Mirrors `Last Updated` |
| `LegalConsentViewModel.kt` | `CURRENT_LEGAL_VERSION` | Version identifier; must equal Effective Date; bumped on major updates only |
| `LegalConsentScreen.kt` | Effective date footer | Derived automatically from `CURRENT_LEGAL_VERSION` — no manual edit needed |

---

## First Release (MVP) Note

For the initial app release there is no prior version to compare against — users
are consenting for the first time. Set `Last Updated`, `Effective Date`, and
`CURRENT_LEGAL_VERSION` all to the same date (the release date).
