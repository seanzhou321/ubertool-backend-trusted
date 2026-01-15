# Product Requirements Document - Trusted Group Backend

## 1. Executive Summary

The Trusted Group (Church) Backend provides a secure API service for a private tool-sharing community. It supports **gRPC for mobile clients** (Android/iOS) and uses **PostgreSQL** for persistence. Key pivots from the original design include **invitation-only access**, a **ledger system** for internal credits, removal of mapping/location services, and removal of the review system.

**Community Model:** "Trusted Group" (e.g., Church Members). Trust is established offline/externally.
**MVP Scope:** Mobile platforms (Android/iOS). No payment gateway (internal credits/ledger only).

## 2. Technical Vision

Build a secure, maintainable backend using **Go (Golang)** microservices.
- **API Style:** gRPC for mobile clients (Android, iOS) - Protocol Buffers.
- **Database:** PostgreSQL.
- **Architecture:** Layered microservices.

## 3. System Architecture Overview

### 3.1 Architecture Style
- **Pattern:** Microservices (Go).
- **API:** gRPC.
- **Database:** PostgreSQL.
- **Authentication:** JWT-based.

### 3.1 Multi-Org Tenancy & Onboarding
- **Organization Centricity:** Every interaction (Rental, Search, Billing) happens within the context of an Organization (Church/Group).
- **Membership:** Users can belong to multiple Orgs.
- **Invitation Flow:**
    - Admin creates Invite for `Org_A`.
    - Token identifies `Org_A`.
    - User registering with this token auto-joins `Org_A`.
- **Request to Join:** Users can request to join a specific Org via the app.

### 3.2 Authentication & User Profile
**Priority:** P0 (MVP)

**Diff from Original:**
- **NO open registration.**
- **Invitation Token:** Admins generate tokens sent via email. Token valid for 7 days.
- **Request to Join:** Users can submit a request (Name, Email, Note) -> Admin approves -> Invite Email sent.

**Requirements:**
- Validate Link/Token integrity.
- standard email/password setup after token validation.
- **2FA:** Login requires Email OTP verification.
- Password reset flow.

**API Endpoints (gRPC Definitions):**
- `ValidateInvite(invitation_code, email)`
- `UserSignup(invitation_code, name, email, phone, password)`
- `Login(email, password)`
- `Verify2FA(session_id, code)`
- `RequestToJoinOrganization(organization_id, name, email, message)`
- `RefreshToken(refresh_token)`
- `Logout(user_id)`
- **Admin Service:** `ApproveRequestToJoin(user_id, organization_id, applicant_email)`

### 3.3 Search & Discovery (Multi-Org Logic)
1.  **Initial Context:** User selects a "Current Org" (e.g., Church A) to start searching.
2.  **Auto-Metro Filter:** The search automatically filters for Tools in Church A's `metro`.
3.  **Cross-Org Results:** The search *also* includes tools from *other* Orgs the user belongs to (e.g., Church B), provided they are in the same/compatible Metro.
4.  **Ranking:**
    - Results are displayed as a **List**, ordered by `Price (Low -> High)`.
    - **No Map View.** Location is indicated by text (e.g., "North Metro").
5.  **Context Switching Alert:**
    - If a user selects a tool from "Church B" while browsing in "Church A" context:
    - **System Prompt:** *"The owner of this tool is in [Church B]. To rent it, we need to switch your dashboard context to that organization."*

### 3.4 Core Services
1.  **Auth Service:** Invitation handling, Login, Registration.
2.  **User Service:** Profile, Ledger/Balance management.
3.  **Tool Service:** Listing CRUD (List view only).
4.  **Ledger Service:** Transaction tracking (Debits/Credits).
5.  **Rental Service:** Request workflow + Owner confirmation.
6.  **Notification Service:** Email and In-app alerts (MVP).
7.  **Storage Service:** Image upload.

**Omitted Services:** Geolocation, Review.

## 4. Core Features & Requirements

### 4.1 User Management & Ledger
**Priority:** P0 (MVP)

**Diff from Original:**
- Added **Ledger System**.
- **Admin Reset:** Admins can reset balances.

**Requirements:**
- **User Balance:** Track current Credit/Debit.
- **Ledger History:** Record of all "rental cost" transactions.
- **Admin Ops:** Ability to zero out or adjust balances manually.

**Data Model (Ledger):**
- `user_id`
- `balance` (integer, cents)
- `transactions`: `[id, user_id, amount, type (DEBIT/CREDIT), related_rental_id, timestamp]`

**API Endpoints:**
- **Ledger Service:**
    - `GetBalance(user_id, organization_id)`
    - `GetTransactions(user_id, organization_id, page, page_size)`
    - `GetLedgerSummary(user_id, organization_id)`
- **User Service:**
    - `GetUser(user_id)`
    - `UpdateProfile(user_id, name, email, phone, avatar_url)`
- **Admin Service:**
    - `AdminAdjustBalance(user_id, organization_id, amount, reason)`
    - `AdminBlockUserAccount(user_id, organization_id, reason)`

### 4.2 Tool Management & Search (List View)
**Priority:** P0 (MVP)

**Diff from Original:**
- **NO Map/Distance Search.**
- **Search Logic:** Simple query + filters.
- **Sorting:** Default order is **Price: Low to High**.

**Requirements:**
- **List View:** Return tools matching query.
- **Display Fields:** Name, Description, **Price**, **Unit of Duration** (Day/Week), **Replacement Value**, **Condition**.
- **Filters:** Condition (New, Good, Fair), Max Price.

**API Endpoints:**
- `SearchTools(organization_id, query, categories, max_price, condition)`
- `GetTool(tool_id)`
- `ListTools(organization_id, user_id)`
- `AddTool(user_id, name, description, categories, price_per_day_cents, ...)`
- `UpdateTool(tool_id, ...)`
- `DeleteTool(tool_id)`
- `ListToolCategories()` (Returns all system categories)

**Note:** All prices are stored and transmitted in **cents** (USD) to avoid floating point errors.

### 4.4 Rental Workflow
**Priority:** P0 (MVP)

**Diff from Original:**
- **Owner Confirmation:** Mandatory for ALL requests.
- **Notification:** Sent to owner immediately upon request.

**Flow:**
1.  **User Selects Tool:** Views costs (Day/Week).
2.  **Request:** User submits dates (`CreateRentalRequest`).
3.  **Hold:** System notifies Owner (Email/In-app).
    *   **Alert Content:** *"John wants 'Drill' for Jan 15-17 (via [Org Name])".*
    *   Tool status shows "Pending".
4.  **Confirm (Owner):** Owner accepts (`ApproveRentalRequest`).
    *   **Note:** Owner adds "Pickup Instructions" (Time/Location) in confirmation.
5.  **Finalize (Renter):** Renter views approval and pickup instructions, then confirms intent to proceed (`FinalizeRentalRequest`).
    *   **Status Update:** Rental becomes "Scheduled" or "Active".
6.  **Ledger Update:** Upon "Completion", transaction recorded. Debit Renter / Credit Owner.
7.  **Return:** Owner marks as returned (`CompleteRental`). System checks for overdue.

**Reminders:**
- System cron job checks for overdue rentals daily. Sends email reminder to Renter.

**API Endpoints:**
- `CreateRentalRequest(tool_id, start_date, end_date, organization_id)`
- `ApproveRentalRequest(request_id, pickup_instructions)` [Owner]
- `RejectRentalRequest(request_id, reason)` [Owner]
- `FinalizeRentalRequest(request_id)` [Renter]
- `CompleteRental(request_id)` [Owner]
- `ListMyRentals(...)`
- `ListMyLendings(...)`

### 4.5 Notifications
**Priority:** P0 (MVP)

**Channels:**
- **Email:** Standard SMTP (SendGrid/AWS SES).
- **In-App:** Polling or simple push (Status screen alerts).
- **NO Mobile Push (FCM)** for MVP (as per request "Email and in-app alerts").

**Triggers:**
- Invite Received.
- New Rental Request (to Owner).
- Request Accepted (to Renter).
- Overdue Reminder (to Renter).

## 5. Technology Stack
- **Language:** Go (Golang).
- **Framework:** gRPC (Protobuf).
- **Database:** PostgreSQL.
- **Mobile:** Android / iOS (Native or Flutter/React Native - Client generated from Proto).

## 6. MVP Timeline (Adjusted)
- **Phase 1:** Auth (Invites) & Ledger Core.
- **Phase 2:** Tool CRUD & List Search.
- **Phase 3:** Rental Flow & Notifications.
- **Phase 4:** Mobile Integration.
