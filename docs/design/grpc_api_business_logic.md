# Business Logic of Ubertool Trusted Backend

The business logic should be implemented in the service layer.

## Architectural Note: Bill Status Type Constraint Strategy

**Bill Status Values**: `PENDING`, `PAID`, `DISPUTED`, `ADMIN_RESOLVED`, `SYSTEM_DEFAULT_ACTION`

**Constraint Location**: 
- ✅ **Domain Model** (`internal/domain/bill.go`): Defines `BillStatus` type with constants - **enforced here**
- ❌ **Proto** (`bill_split_service.proto`): Uses `string` type - **not constrained**
- ❌ **Database** (`ubertool_schema_trusted.sql`): Uses `TEXT` type with comment - **not constrained**

**Rationale**:
1. **Domain Model Constraint**: Application code must use type-safe constants (`domain.BillStatusPending`, etc.) preventing typos and invalid values at compile time.
2. **Proto/DB Flexibility**: Keeping proto and database as `string`/`TEXT` allows future status additions without:
   - Database migrations for enum type changes
   - Proto breaking changes forcing client updates
   - API version bumps
3. **Migration Path**: New statuses can be added to domain model first, implemented in service layer, then deployed without coordinated DB/client updates.
4. **Validation**: Service layer validates incoming proto string values against domain model constants before using them.

**Implementation**: All service layer code must use `domain.BillStatus*` constants, never hardcoded strings. 

## Architectural Note: Push Notification Pattern

For every event that creates an in-app `notifications` record AND sends an email, a push notification is also sent to the same recipient(s) — **except** events during sign-on, user identification, user provisioning, and invitations (2FA codes, invitation emails, signup flows, join request submissions).

**Pattern per event**:
1. Insert a row into `notifications` to get a `notification_id` (`BIGSERIAL`).
2. Send email (existing behavior).
3. Look up all `fcm_tokens` rows for the target `user_id` where `status = 'ACTIVE'`.
4. For each active token, call `messaging.Send()` asynchronously (goroutine with a detached context and timeout):
   - Include the `notification_id` in the FCM data payload (key: `"notification_id"`) so the client can call `ReportMessageEvent`.
   - On `messaging.IsUnregistered(err)` response, set `fcm_tokens.status = 'OBSOLETE'` for that token.
5. The client app reports delivery/click events back via `ReportMessageEvent`.

## Authentication

### Validate Invite
Purpose: Users receives an invitation code from email. The UI calls this API method to validate the invitation code. If the invitation code is valid, the UI will show the signup form. If the invitation code is invalid, the UI will show the error message. 

Input: invitation_code, email
Output: valid or not, error message, User object (if user exists AND is currently logged in)
Business Logic: 
1. Verify the invitation record with (`invitation_code`, `email`) in invitations table exists and not expired.
2. Check if a user with this email exists in the users table.
3. If user exists AND has a valid authentication session (logged in), return the User object in the response.
4. If user exists but NOT logged in, do NOT return User object (validation should not substitute login process).
5. If user doesn't exist, do NOT return User object (they need to sign up).
6. Return true if validation success, otherwise return false and error message, stating the "invitation code and email pair is invalid or expired."

Note: The presence of a User object in the response indicates the user is logged in and can proceed to join the organization directly.

### Request To Join Organization
Purpose: A user who is not part of an organization wants to join. They search for the organization and submit a request.

Input: `organization_id`, `name`, `email`, `message`
Output: success/failure, message
Business Logic:
1. Verify the organization exists in the `orgs` table.
2. Search the `users` table for the user with the given `email`.
3. Create a new entry in the `join_requests` table with `status` set to 'PENDING'.
4. The `user_id` in `join_requests` may be assigned from the user found by the email, or should be null if the user is not found.
5. Find the admin users in `organization_id`. 
6. Send emails to the admin users with the new user email, name, and the message.
7. Create a notification for each admin user (insert into `notifications`).
8. Send push notification to each admin user (see Push Notification Pattern).
9. Return success/failure and message, "Your request to join the organization has been submitted."

### User Signup
Purpose: Register a new user account.

Input: `invitation_code`, `name`, `email`, `phone`, `password`
Output: success, message
Business Logic:
1. Validate the `invitation_code` and `email` pair from invitations table (exists, not expired, and `used_on` is null).
2. Retrieve the `organization_id` from the invitations record.
3. Search the user with email address from Users table.
4. If user already exists, return error "Email already registered. Please log in instead."
5. Create a new user record in the `users` table with hashed password.
6. Update the `invitations` record's `used_on` field with the current timestamp and `used_by_user_id`.
7. If the invitation has a `join_request_id`, retrieve the `join_requests` record and set its `status` to `'JOINED'`.
8. Create a record in the `users_orgs` table with `user_id`, `organization_id`, and role 'MEMBER'.
9. Initialize user's balance in the organization (balance_cents = 0).
10. Return success and message "Your account has been created. Please log in."

Note: User must go through normal login process after signup. Signup does NOT return authentication tokens.

### Login
Purpose: Authenticate existing user and initiate two-factor authentication.

Input: `email`, `password`
Output: success/failure boolean, two_fa_token, expires_at, message
Business Logic:
1. Fetch user by `email`.
2. Verify `password` against `users.password_hash`.
3. If the canonical password does not match, check `pending_credentials` for this user:
   - A pending credential is valid only when `used_at IS NULL` and `expires_at > NOW()`.
   - If a valid temporary password matches, proceed and flag the session as `temp_pwd=true`.
4. If neither password matches, return error "Either the email and/or the password is wrong".
5. Generate a 5-digit 2FA code and send it by email to the user.
6. Generate a `two_fa_token` (JWT of type `2fa_pending`) containing `user_id` and the `temp_pwd` flag.
7. Return the `two_fa_token`.

### Verify 2FA
Purpose: Complete authentication with a second factor.

Input: `two_fa_code` (2FA token required in header)
Output: bool, access token, refresh token, user profile, reset_password flag
Business Logic:
1. Extract `user_id` and `temp_pwd` from the `2fa_pending` token (injected into context by the auth interceptor).
2. Validate the `two_fa_code` against the pending code stored for this user.
3. If match, generate JWT `access_token` and `refresh_token` and return the user profile.
4. Set `reset_password = temp_pwd` in the response. When `true`, the client must redirect the user to the change-password screen before allowing normal app access.
5. Delete the pending 2FA code to prevent reuse.

### Refresh Token
Purpose: Get a new access token using a refresh token.

Input: `refresh_token`
Output: new access token, new refresh token
Business Logic:
1. Validate the `refresh_token`.
2. If valid, issue a new access token and a new refresh token.
3. if fail, output a warning security log.

### Logout
Purpose: Invalidate the current session and stop push notifications to the device.

Input: `android_device_id` (access token required in header)
Output: success flag
Business Logic:
1. Extract `user_id` from the JWT access token.
2. Invalidate/blacklist the current JWT tokens (access and refresh tokens).
3. Mark the FCM token(s) for this device as OBSOLETE:
   `UPDATE fcm_tokens SET status = 'OBSOLETE', updated_at = NOW() WHERE user_id = $user_id AND android_device_id = $android_device_id AND status = 'ACTIVE'`
   This prevents the backend from routing future push notifications to a logged-out device.

### Change Password
Purpose: Authenticated user changes their own password.

Input: `old_password`, `new_password` (access token required in header)
Output: success, message
Business Logic:
1. Extract `user_id` from the JWT access token.
2. Verify `old_password` against `users.password_hash`.
3. If the canonical password does not match, check `pending_credentials` for this user:
   - A pending credential is valid only when `used_at IS NULL` and `expires_at > NOW()`.
   - If a valid temporary password matches, proceed.
4. If neither password matches, return error "invalid email or password".
5. Hash `new_password` with bcrypt and update `users.password_hash`.
6. Stamp `pending_credentials.used_at = NOW()` for this user (if a row with `used_at IS NULL` exists), invalidating the temporary password.
7. Return success and message "Password changed successfully."

### Reset Password
Purpose: Self-service password reset — no authentication required. Generates a temporary password and emails it to the user.

Input: `user_email` (no access token required)
Output: success, message
Business Logic:
1. Look up the user by `user_email` in the `users` table.
2. If not found, return a generic success message to avoid leaking user existence: "If an account with that email exists, a temporary password has been sent."
3. Generate a cryptographically secure random temporary password (16 hex characters).
4. Hash the temporary password with bcrypt.
5. Upsert a row in `pending_credentials` for this user:
   - `temp_password_hash` = the bcrypt hash
   - `expires_at` = NOW() + 48 hours
   - `used_at` = NULL
6. Send an email to the user containing the plain-text temporary password and instructions to log in and change it immediately.
7. Return the generic success message.

## Administration

### Approve Request To Join
Purpose: Admin approves a pending request to join an organization.

Input: `organization_id`, `join_request_id`
Output: success/failure, message, invitation_code (if new user)
Business Logic:
1. Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Retrieve the `join_requests` record by `join_request_id` and verify it belongs to the given `organization_id`.
3. If the user already exists in `users` table (by email from the join request), add them to `users_orgs` with 'MEMBER' role and notify them.
4. If the user does not exist, create an invitation record in `invitations` with `join_request_id` set, and send the invitation code to the applicant (cc to the admin).
5. Update `join_requests.status` to `'INVITED'`.
6. Return the invitation code if one was created, empty string otherwise.

### Reject Pending Request To Join
Purpose: Admin rejects a pending request to join an organization.

Input: `organization_id`, `join_request_id`, `reason`
Output: success/failure, message
Business Logic:
1. Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Retrieve the `join_requests` record by `join_request_id` and verify it belongs to the given `organization_id`.
3. Update `join_requests.status` to `'REJECTED'` and populate `join_requests.reason` with the provided reason.
4. Send a rejection email to the applicant.
5. Return success.

### Send Invitation
Purpose: Admin sends an invitation to join the organization manually.

Input: `organization_id`, `email`, `name`
Output: success, message, invitation_code
Business Logic:
1. Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role.
2. Check if user is already a member. If so, return error.
3. Create a new invitation record in `invitations` table.
4. If the user does not exist in `users` table, send an invitation email with the code.
5. Return the invitation code in response.

### Admin Block User Account
Purpose: Admin blocks/unblocks a member's renting and/or lending privileges.

Input: `blocked_user_id`, `organization_id`, `block_renting`, `block_lending`, `reason`
Output: success, error_message
Business Logic:
1. Verify caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Verify the `blocked_user_id` is a member of the organization.
3. Update the `users_orgs` record:
   - Set `renting_blocked` = `block_renting`
   - Set `lending_blocked` = `block_lending`
   - If either flag is true, set `blocked_on` to current date and `blocked_reason` to the provided reason.
   - If both flags are false (unblocking), clear `blocked_on`, `blocked_reason`.
   - Update `status` to 'BLOCK' if either flag is true, otherwise 'ACTIVE'.
4. Create notification for the blocked/unblocked user (insert into `notifications`).
5. Send email to the user informing them of the block/unblock action.
6. Send push notification to the user (see Push Notification Pattern).
7. Return success or error message.

**Note**: This method can be used to:
- Block renting only: `block_renting=true`, `block_lending=false`
- Block lending only: `block_renting=false`, `block_lending=true`
- Block both: `block_renting=true`, `block_lending=true`
- Unblock user: `block_renting=false`, `block_lending=false`

### List Members
Purpose: List all members of an organization.

Input: `organization_id`
Output: list of member profiles
Business Logic:
1. Verify caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Join `users_orgs` and `users` tables to return member details.
3. For each member, populate `MemberProfile` with:
   - User basic info: `user_id`, `name`, `email`, `phone`, `avatar_url`
   - Organization-specific info: `balance_cents`, `role`, `status`, `member_since` (joined_on)
   - Blocking info: `renting_blocked`, `lending_blocked`, `blocked_on`, `blocked_reason`
   - Computed field `is_blocked` = `renting_blocked OR lending_blocked`
4. Return list of member profiles.

### Search Users
Purpose: Search for specific members within an organization.

Input: `organization_id`, `query`
Output: list of matching member profiles
Business Logic:
1. Verify caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Filter members by name or email using the `query` string in the `organization_id`.
3. Return member profiles with same fields as ListMembers, including computed `is_blocked` field.

### Get Member Profile
Purpose: Get detailed profile information for a specific member.

Input: `organization_id`, `user_id`
Output: member profile
Business Logic:
1. Verify caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Query `users_orgs` joined with `users` for the specified `user_id` and `organization_id`.
3. If member not found, return error "Member not found in this organization".
4. Populate complete `MemberProfile` with:
   - User basic info: `user_id`, `name`, `email`, `phone`, `avatar_url`
   - Organization-specific info: `balance_cents`, `role`, `status`, `member_since` (joined_on)
   - Blocking info: `renting_blocked`, `lending_blocked`, `blocked_on`, `blocked_reason`
   - Computed field `is_blocked` = `renting_blocked OR lending_blocked`
5. Return member profile.

### List Join Requests
Purpose: View pending applications to join the organization.

Input: `organization_id`
Output: list of pending join requests
Business Logic:
1. Verify caller admin rights.
2. Query `join_requests` where `org_id` matches and `status` is 'PENDING'.

## Organizations

### List My Organizations
Purpose: Get all organizations the current user belongs to.

Input: none
Output: list of organizations
Business Logic:
1. Query `users_orgs` for the current `user_id`.
2. Join with `orgs` to get details and user-specific stats (balance, active items).

### Get Organization
Purpose: Get detailed info about a specific organization.

Input: `organization_id`
Output: organization details
Business Logic:
1. Fetch organization details from `orgs`.
2. Calculate total member count from `users_orgs`.
3. Fetch user-specific info for this org (balance, active items).

### Create Organization
Purpose: Start a new community/organization.

Input: `name`, `description`, `address`, `metro`, `admin_email`, `admin_phone`
Output: created organization info
Business Logic:
1. Insert new record into `orgs`.
2. Add the creator as `SUPER_ADMIN` in `users_orgs` with `balance_cents = 0`.

### Join Organization With Invite
Purpose: Allow an existing, logged-in user to join an additional organization using an invitation code.

Input: `invitation_code` (user_id from JWT token)
Output: success, organization, user, message
Business Logic:
1. Verify the user is authenticated (extract user_id from JWT token).
2. Validate the `invitation_code` from invitations table (exists, not expired, and `used_on` is null).
3. Retrieve the `organization_id` and invited `email` from the invitations record.
4. Verify the authenticated user's email matches the invitation email.
5. Check if user is already a member of the organization (search `users_orgs` table).
6. If already a member, return error "You are already a member of this organization".
7. Update the `invitations` record's `used_on` field with the current timestamp and `used_by_user_id`.
8. Create a record in the `users_orgs` table with `user_id`, `organization_id`, and role 'MEMBER'.
9. Initialize user's balance in the organization (balance_cents = 0).
10. Create a notification to org admins about the new member.
11. Return success, organization details, user details, and message "Successfully joined [organization name]".

Note: This endpoint is for existing users who are already logged in and want to join additional organizations. New users must complete signup and login first.

### Search Organizations
Purpose: Find organizations to join.

Input: `name`, `metro`
Output: list of organizations
Business Logic:
1. Query `orgs` based on name and/or metro.

### Update Organization
Purpose: Update organization details.

Input: `organization_id`, `name`, `description`, `address`, `metro`, `admin_email`, `admin_phone`, `billsplit_settlement_threshold_cents`, `max_billsplit_rental_cost_cents`
Output: updated organization info
Business Logic:
1. Verify the caller's `user_role` is `SUPER_ADMIN` for the given `organization_id`. Return permission error if otherwise.
2. Update the `name`, `description`, `address`, `metro`, `admin_email`, and `admin_phone` fields on the `orgs` record.
3. If `billsplit_settlement_threshold_cents` is greater than zero, update `orgs.billsplit_settlement_threshold_cents`. If the value is zero, leave the existing value unchanged — zero is not a valid threshold.
4. If `max_billsplit_rental_cost_cents` is greater than zero, update `orgs.max_billsplit_rental_cost_cents`. If the value is zero, leave the existing value unchanged — zero is not a valid cap.
5. Return the updated organization record.
6. If either `billsplit_settlement_threshold_cents` or `max_billsplit_rental_cost_cents` was changed (i.e. the incoming value was greater than zero and differs from the value stored before the update), broadcast to all active members of the community:
   - Fetch all `user_id` values from `users_orgs` where `organization_id` matches and the user is not blocked.
   - For each member, create a `notifications` record with attributes:
     ```
     {
       topic:                              org_threshold_update,
       organization_id:                    organization_id,
       billsplit_settlement_threshold_cents: <new value>,
       max_billsplit_rental_cost_cents:      <new value>
     }
     ```
   - Send an email to each member with the subject **"[Community Name] Payment Threshold Update"** and a body that states the new values of both thresholds, and reminds members that rentals above the cap must be settled directly between Lender and Renter.
   - Send a push notification to all members using **FCM Multicast** (`messaging.SendEachForMulticast`) since the payload is identical for every recipient. Collect all active `fcm_tokens` for all affected `user_id` values in a single query, then batch them into multicast requests of up to 500 tokens each (FCM multicast limit). Use channel `admin_messages`. Include the new threshold values in the FCM `data` map. On `messaging.IsUnregistered(err)` for any token in the batch response, set that token's `fcm_tokens.status = 'OBSOLETE'`.

Note: Only `SUPER_ADMIN` may modify the two billsplit price fields. Regular `ADMIN` role can update the other fields but the service must reject any attempt to change the price fields from a non-SUPER_ADMIN caller.

## Bill Split

### Get Global Bill Split Summary
Purpose: Get aggregated counts of payments and receipts for the dashboard.

Input: none (uses user_id from token)
Output: BillSplitSummary (payments_to_make, receipts_to_verify, payments_in_dispute, receipts_in_dispute)
Business Logic:
1. Verify user is authenticated.
2. Aggregate counts from all organizations the user belongs to.
3. Count pending payments (where user is debtor, status is PENDING).
4. Count pending receipts (where user is creditor, status is PAID, meaning debtor paid but creditor hasn't verified).
5. Count payments in dispute (where user is debtor).
6. Count receipts in dispute (where user is creditor).
7. Return the aggregated numbers.

### Get Organization Bill Split Summary
Purpose: Get bill split summary for each organization.

Input: none
Output: List of OrganizationBillSplitSummary
Business Logic:
1. Verify user is authenticated.
2. For each organization the user is a member of, calculate the BillSplitSummary (same logic as Global but per org).
3. Return the list.

### List Payments
Purpose: List payment items for a specific organization.

Input: `organization_id`, `show_history`, `settlement_month` (optional), `pagination` (optional)
Output: List of PaymentItem, PaginationResponse
Business Logic:
1. Verify user is a member of `organization_id`.
2. Retrieve payments where user is either debtor or creditor.
3. If `show_history` is false, filter for active items (To Pay, To Ack, In Dispute).
4. If `show_history` is true, return completed items.
5. If `settlement_month` is provided, filter by exact settlement month (format: 'YYYY-MM').
6. Map internal status to PaymentCategory.
7. Populate `PaymentItem` fields from `bills` table:
    - `payment_id`, `debtor_id`, `creditor_id`, `amount_cents`
    - `settlement_month`, `status`
    - Timestamps (Epoch ms): `notice_sent_at`, `debtor_acknowledged_at`, `creditor_acknowledged_at`, `disputed_at`, `resolved_at`
    - Dispute details: `dispute_reason`, `resolution_outcome`
8. Sort by `notice_sent_at` or `created_on` descending.
9. Apply pagination:
    - If `pagination.page_size` not provided, use default (50).
    - If `pagination.page_token` provided, continue from that position.
    - Return `next_page_token` if more results exist, empty string otherwise.
    - Return `total_count` of all matching items across all pages.

### Get Payment Detail
Purpose: Get details of a specific payment/bill.

Input: `payment_id`
Output: PaymentItem, history of actions, can_acknowledge flag
Business Logic:
1. Verify user is involved in the payment (debtor, creditor) or is Admin/Super Admin of the org.
2. Retrieve `bills` record and map to `PaymentItem` (as above).
3. Retrieve `bill_actions` for this bill and map to `PaymentAction`:
    - `actor_user_id`, `actor_name`
    - `action_type`, `notes`, `created_on`
    - `action_details_json`
4. Determine `can_acknowledge`:
    - If user is Debtor and (status is PENDING or DISPUTED) and debtor_acknowledged_at is NULL -> True.
    - If user is Creditor and (status is PENDING or DISPUTED) and debtor_acknowledged_at is NOT NULL and creditor_acknowledged_at is NULL -> True.
    - Otherwise -> False.
5. Note: Acknowledging a DISPUTED bill enables graceful resolution without admin intervention.

### Acknowledge Payment
Purpose: User acknowledges sending payment (Debtor) or receiving payment (Creditor). Enables graceful resolution even after dispute.

Input: `payment_id`
Output: success, message
Business Logic:
1. Verify user is involved (debtor or creditor).
2. Get bill and verify status is PENDING or DISPUTED.
3. If Debtor:
    - Check bill status is PENDING or DISPUTED.
    - Check debtor has not already acknowledged.
    - Set `debtor_acknowledged_at` = NOW().
    - Create bill action: DEBTOR_ACKNOWLEDGED.
    - Create a notification for the creditor (insert into `notifications`).
    - Send push notification to the creditor (see Push Notification Pattern).
    - Note: Status remains PENDING or DISPUTED until creditor also acknowledges.
4. If Creditor:
    - Check bill status is PENDING or DISPUTED.
    - Check debtor has already acknowledged (debtor_acknowledged_at is not null).
    - Check creditor has not already acknowledged.
    - Set `creditor_acknowledged_at` = NOW().
    - Set `status` = PAID.
    - Set `resolved_at` = NOW().
    - Set `resolution_outcome` = GRACEFUL (both parties acknowledged).
    - Create bill action: CREDITOR_ACKNOWLEDGED.
    - Update balances in users_orgs table for both parties.
    - Create a notification for the debtor (insert into `notifications`).
    - Send push notification to the debtor (see Push Notification Pattern).
5. **Graceful Resolution after Dispute**: If bill was in DISPUTED status and both parties acknowledge, it resolves as GRACEFUL without admin intervention.

### List Disputed Payments
Purpose: Admin lists unresolved disputes requiring intervention.

Input: `organization_id`, `pagination` (optional)
Output: List of DisputedPaymentItem, PaginationResponse
Business Logic:
1. Verify user is Admin/Super Admin of `organization_id`.
2. Query `bills` where `org_id` matches and `status` is 'DISPUTED'.
3. Join with `users` to get debtor and creditor names.
4. Populate `DisputedPaymentItem`.
5. Apply pagination similar to ListPayments.

### List Resolved Disputes
Purpose: Admin lists history of resolved disputes with optional filtering.

Input: `organization_id`, `resolution_outcome` (optional), `pagination` (optional)
Output: List of DisputedPaymentItem (resolved), PaginationResponse
Business Logic:
1. Verify user is ADMIN/SUPER_ADMIN of `organization_id`.
2. Query payments with status in (PAID, ADMIN_RESOLVED, SYSTEM_DEFAULT_ACTION) and `disputed_at` IS NOT NULL.
3. If `resolution_outcome` is provided, filter by exact outcome:
    - GRACEFUL: Both parties acknowledged
    - DEBTOR_FAULT: Admin determined debtor at fault
    - CREDITOR_FAULT: Admin determined creditor at fault
    - BOTH_FAULT: Admin determined both parties at fault
4. Map to DisputedPaymentItem with resolution details.
5. Apply pagination:
    - If `pagination.page_size` not provided, use default (50).
    - If `pagination.page_token` provided, continue from that position.
    - Return `next_page_token` if more results exist.
    - Return `total_count` of all matching resolved disputes.
6. Sort by `resolved_at` descending (most recent first).

### Resolve Dispute
Purpose: Admin resolves a dispute with either forced resolution (penalties/blocking) or non-forced resolution (parties resolved offline).

Input: `payment_id`, `resolution` (DEBTOR_AT_FAULT, CREDITOR_AT_FAULT, BOTH_AT_FAULT, GRACEFUL), `notes` (optional)
Output: success, message
Business Logic:
1. Verify user is ADMIN/SUPER_ADMIN of the org of the payment.
2. Verify admin is either creditor nor debtor in the payment (cannot resolve own disputes).
3. Verify bill status is DISPUTED.
4. Set `status` = ADMIN_RESOLVED.
5. Set `resolved_at` = NOW().
6. Set `resolution_outcome` based on input.
7. Set `resolution_notes` from admin's notes field (documentation of decision).

8. Apply resolution consequences based on type:
    
    **Forced Resolutions** (with penalties/blocking):
    - **DEBTOR_AT_FAULT**: 
        - Update balances (enforce payment).
        - Block debtor from renting (`renting_blocked` = true).
        - Set `blocked_due_to_bill_id` and `blocked_reason` in users_orgs.
    - **CREDITOR_AT_FAULT**: 
        - Apply penalty to creditor balance (subtract amount).
        - Block creditor from lending (`lending_blocked` = true).
        - Set `blocked_due_to_bill_id` and `blocked_reason`.
    - **BOTH_AT_FAULT**: 
        - Apply penalties to both parties (subtract amount from each).
        - Block debtor from renting AND creditor from lending.
        - Set blocking metadata for both.
    
    **Non-Forced Resolution**:
    - **GRACEFUL**: 
        - Admin confirms parties resolved it offline (e.g., cash payment, mutual agreement).
        - Update balances (complete payment normally).
        - No penalties applied.
        - No blocking applied.
        - Resolution notes document the offline resolution.

9. Create bill action: ADMIN_RESOLUTION with notes.
10. Create a notification for both debtor and creditor (insert into `notifications`).
11. Send push notification to both debtor and creditor (see Push Notification Pattern).
12. Send email to both parties with resolution outcome and notes.

**Note**: Admins can use GRACEFUL option when parties resolved offline and admin is just confirming. For unblocking users after debts are cleared, use `AdminService.AdminBlockUserAccount` with `block_renting=false` or `block_lending=false`.

## Tools

### List My Tools
Purpose: Browse tools available in an organization.

Input: `metro`, pagination
Output: tool list
Business Logic:
1. Query `tools` belong to a user in a metro. The `user_id` is obtained from the JWT token.
2. Return active tools (`deleted_on` is null).

### Get Tool
Purpose: View specific tool details.

Input: `tool_id`
Output: tool details including images
Business Logic:
1. Query `tools` and join with `tool_images`.
2. Return the tool object.

### Add Tool
Purpose: List a new tool to rent.

Input: `name`, `description`, `categories`, prices, `duration`, `condition`, `image_url`
Output: the created tool object (includes `durationUnit` reflecting the persisted value)
Business Logic:
1. Insert into `tools` table, persisting `duration` from the request as `duration_unit` in the database.
2. The `owner_id` is the `user_id` from JWT token.
3. Insert `image_url` into `tool_images`.

Note: `duration` in the request represents the rental period unit (`day`, `week`, or `month`). It is stored as `duration_unit` in the database and returned as `durationUnit` in the `Tool` response message.

### Update Tool
Purpose: Update the content of a tool.

Input: `tool_id`, and all updated fields including `duration`, except id, owner_id, created_on, deleted_on fields.
Output: the updated tool object (includes `durationUnit` reflecting the persisted value)
Business Logic:
1. Verify the current user is the owner of the tool.
2. Update `tools` (including `duration_unit`) and `tool_images`.

Note: `duration` in the request represents the rental period unit (`day`, `week`, or `month`). It is stored as `duration_unit` in the database and returned as `durationUnit` in the `Tool` response message.

### Delete Tool
Purpose: Remove a tool listing.

Input: `tool_id`
Output: success flag
Business Logic:
1. Verify ownership.
2. Perform soft delete by setting `deleted_on` timestamp.

### Search Tools
Purpose: Advanced search for tools.

Input: `organization_id`, `query`, `categories`, `max_price`, `condition`, `metro`, `start_date`, `end_date`
Output: matching tool list
Business Logic:
1. if `organization_id` is given, verify `user_id` belongs to this organization.
2. Filter tools by search term, categories, price range, condition, metro.
3. Filter tools by `organization_id` that the user belongs to.
4. Filter tools by status="AVAILABLE" or "RENTED"
5. If the start and end date is given, filter by the rental duration for tools of status="RENTED".

### List Tool Categories
Purpose: Get all unique categories used in the system.

Input: none
Output: list of category strings
Business Logic:
1. Return `DISTINCT` categories from the `tools` table.

## Rentals

### Create Rental Request
Purpose: Renter requests to borrow a tool.

Input: `tool_id`, `start_date`, `end_date`, `organization_id`
Output: rental request details
Business Logic:
1. Verify the tool is either available or its rental schedule is free for the specified start_date and end_date.
2. Validate that `end_date` is strictly after `start_date`. If `end_date <= start_date`, return error: "end date must be after start date (minimum 1 day rental)".
3. Calculate `total_cost_cents` using the tiered pricing algorithm. Duration is computed as `end_date - start_date` (end date is **exclusive**; minimum effective duration is 1 day). See `tool-rental-pricing-algorithm.md` for details.
4. Copy the tool's current price fields into the rental record as an immutable price snapshot:
   - `duration_unit` ← tool's `duration_unit`
   - `daily_price_cents` ← tool's `price_per_day_cents`
   - `weekly_price_cents` ← tool's `price_per_week_cents`
   - `monthly_price_cents` ← tool's `price_per_month_cents`
   - `replacement_cost_cents` ← tool's `replacement_cost_cents`
   All subsequent cost calculations for this rental use these snapshot values, so future tool price changes do not affect already-created rentals.
5. Insert into `rentals` with status 'PENDING', storing the snapshot fields and computed `total_cost_cents`.
6. Create a notification to the owner with attributes set to {topic:rental_request; rental:rental_id; purpose:"request for approval"} (insert into `notifications`).
7. Send an email to the tool owner to notify the rental request, cc to the renter (user_id parsed from the JWT token).
8. Send push notification to the owner (see Push Notification Pattern).

### Approve Rental Request
Purpose: Owner approves the lending.

Input: `request_id`, `pickup_instructions`
Output: updated rental request object
Business Logic:
1. Verify `user_id` is the tool owner.
2. Update `rentals` status to 'APPROVED'.
3. Create a notification to the renter with attributes set to {topic:rental_request; rental:rental_id, purpose:"rental request approved"} (insert into `notifications`).
4. Send an email to the renter to notify the rental approval with `pickup_instruction`, cc to the owner.
5. Send push notification to the renter (see Push Notification Pattern).

### Reject Rental Request
Purpose: Owner declines the lending.

Input: `request_id`, `reason`
Output: success, updated rental request object
Business Logic:
1. Verify `user_id` is the tool owner.
2. Update `rentals` status to 'REJECTED'.
3. Create a notification to the renter with attributes set to {topic:rental_request; rental:rental_id; purpose:"rental request rejected"} (insert into `notifications`).
4. Send an email to the renter to notify the rental rejection with `reason`, cc to the owner (user_id parsed from the JWT token).
5. Send push notification to the renter (see Push Notification Pattern).

### Finalize Rental Request
Purpose: Renter confirms and pays for the rental.

Input: `request_id`
Output: updated rental request object, list of approved rental request objects, list of pending rental objects of the same type of tools
Business Logic:
1. Verify `user_id` is the renter.
2. Copy `end_date` to `last_agreed_end_date` (save the agreed-upon date for potential rollback).
3. Update `rentals` status to 'SCHEDULED'.
4. Update the tool status to 'RENTED'.
5. Create a notification to the owner with attributes set to {topic: rental_request; rental:rental_id; purpose:"rental request confirmed"} (insert into `notifications`).
6. Send an email to the owner to notify the rental confirmation, cc to the renter (user_id parsed from the JWT token).
7. Send push notification to the owner (see Push Notification Pattern).
8. Search approved rental requests of the renter for the same kind of tool.
9. Search pending rental requests of the renter for the same kind of tool.

### Cancel Rental Request
Purpose: Renter cancels the rental request.

Input: `request_id`, `reason`
Output: success, updated rental request object
Business Logic:
1. Verify `user_id` is the tool renter.
2. Update `rentals` status to 'CANCELED'.
3. Create a notification to the owner with attributes set to {topic: rental_request; rental:rental_id; purpose:"rental request canceled"} (insert into `notifications`).
4. Send an email to the owner to notify the cancelation of the rental request with `reason`, cc to the renter (user_id parsed from the JWT token).
5. Send push notification to the owner (see Push Notification Pattern).

### Complete Rental
Purpose: Mark tool as returned.

Input: `request_id`, `return_condition`, `surcharge_or_credit_cents`, `notes`, `charge_billsplit`
Output: updated rental status
Business Logic:
1. Either owner or renter can signal completion.
2. Verify the rental status is `ACTIVE`, `SCHEDULED`, or `OVERDUE`. Return error if otherwise.
3. Calculate `total_cost_cents` based on duration from `start_date` to `end_date` using the rental's price snapshot (captured at creation time). Duration is computed as `end_date - start_date` (end date is exclusive). See `tool-rental-pricing-algorithm.md` for the tiered pricing algorithm.
4. The calculation uses `duration_unit`, `daily_price_cents`, `weekly_price_cents`, and `monthly_price_cents` stored on the rental record, not the tool's current prices.
5. Let `settlement_cents = total_cost_cents + surcharge_or_credit_cents`.
6. Update `rentals`: set `status = 'COMPLETED'`, `completed_by = user_id`, `return_condition`, `surcharge_or_credit_cents`, `total_cost_cents`, `charge_billsplit`, and `notes`.
7. **Owner — balance and ledger (only if `charge_billsplit = true`)**:
   - Add `settlement_cents` to owner's `balance_cents` in `users_orgs` and set `last_balance_updated_on` to today.
   - Create a `ledger_transactions` entry of type `LENDING_CREDIT` for the owner, using `org_id` from `rentals.org_id` and `settlement_cents` as the amount.
8. Create a notification to the **owner** for the credit/balance update (insert into `notifications`) with attributes:
   ```
   {
     topic:           rental_credit_update,
     transaction:     ledger_id,          // omit if charge_billsplit=false (no ledger created)
     amount:          settlement_cents,
     rental:          rental_id,
     charge_billsplit: charge_billsplit
   }
   ```
   - If `charge_billsplit = false`, the notification body must include the following **highlighted** statement:
     > "Reminder: The rental payment of this transaction should be settled directly between you and the renter. This transaction is not included in the monthly billsplit."
9. Send email to the owner to inform the credit/balance update from the rental (include the highlighted statement above if `charge_billsplit = false`).
10. Send push notification to the owner (see Push Notification Pattern). Include `charge_billsplit` in the FCM `data` map.
11. **Renter — balance and ledger (only if `charge_billsplit = true`)**:
    - Add `settlement_cents` to renter's `balance_cents` in `users_orgs` (as a debit — subtract from balance) and set `last_balance_updated_on` to today.
    - Create a `ledger_transactions` record of type `LENDING_DEBIT` for the renter, using `org_id` from `rentals.org_id` and `settlement_cents` as the amount.
12. Create a notification to the **renter** for the debit/balance update (insert into `notifications`) with attributes:
    ```
    {
      topic:           rental_debit_update,
      transaction:     ledger_id,          // omit if charge_billsplit=false (no ledger created)
      amount:          settlement_cents,
      rental:          rental_id,
      charge_billsplit: charge_billsplit
    }
    ```
    - If `charge_billsplit = false`, the notification body must include the following **highlighted** statement:
      > "Reminder: The rental payment of this transaction should be settled directly between you and the owner. This transaction is not included in the monthly billsplit."
13. Send email to the renter to inform the debit/balance update from the rental (include the highlighted statement above if `charge_billsplit = false`).
14. Send push notification to the renter (see Push Notification Pattern). Include `charge_billsplit` in the FCM `data` map.
15. Set `tools.status` back to `AVAILABLE` if the tool has no more `ACTIVE` or `SCHEDULED` rental requests. Otherwise set to `RENTED`.
16. Create a notification to the **owner** for rental completion and tool status change (insert into `notifications`) with attributes:
    ```
    { topic: rental_completion, rental: rental_id, charge_billsplit: charge_billsplit }
    ```
17. Send email to the owner to inform completion of the rental and the tool status change.
18. Send push notification to the owner (see Push Notification Pattern).
19. Create a notification to the **renter** for rental completion (insert into `notifications`) with attributes:
    ```
    { topic: rental_completion, rental: rental_id, charge_billsplit: charge_billsplit }
    ```
20. Send email to the renter to inform completion of the rental.
21. Send push notification to the renter (see Push Notification Pattern).

**Summary — what changes based on `charge_billsplit`:**

| Action | `charge_billsplit = true` | `charge_billsplit = false` |
|---|---|---|
| Owner `users_orgs.balance_cents` updated | ✅ +`settlement_cents` | ❌ No change |
| Owner `ledger_transactions` (LENDING_CREDIT) created | ✅ | ❌ |
| Renter `users_orgs.balance_cents` updated | ✅ −`settlement_cents` | ❌ No change |
| Renter `ledger_transactions` (LENDING_DEBIT) created | ✅ | ❌ |
| Notifications sent to owner and renter | ✅ Normal | ✅ With highlighted direct-settlement reminder |
| `rentals.charge_billsplit` stored | ✅ `true` | ✅ `false` |



### Get Rental
Purpose: View rental details.

Input: `request_id`
Output: rental details
Business Logic:
1. check user_id is either the renter, the owner, or an admin in the organization of rentals.org_id.

### List My Rentals
Purpose: View history/status of tools borrowed.

Input: `organization_id`, status filter (array)
Output: list of rentals
Business Logic:
1. Filter the rental requests of the user as the renter by the status. Multiple statuses can be provided and should be applied with OR logic (match any of the given statuses). If status array is empty, return all statuses.
2. If organization_id is given, filter only the requests from that organization.

### List My Lendings
Purpose: View history/status of tools lent to others.

Input: `organization_id`, status filter (array)
Output: list of lendings
Business Logic:
1. Filter the rental requests of the user as the owner by the status. Multiple statuses can be provided and should be applied with OR logic (match any of the given statuses). If status array is empty, return all statuses.
2. If organization_id is given, filter only the requests from that organization.

### Activate Rental
Purpose: Mark a rental as picked up and in use (transition from SCHEDULED to ACTIVE).

Input: `request_id`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'SCHEDULED'.
2. Verify `user_id` is either the renter or the owner (both can initiate pickup).
3. Update `rentals` status to 'ACTIVE'.
4. Create a notification to the other party (if renter initiated, notify owner; if owner initiated, notify renter) with attributes set to {topic:rental_pickup; rental:rental_id; tool_name:tool_name; start_date:start_date; end_date:end_date; purpose:"rental has been picked up"} (insert into `notifications`).
5. Send an email to the other party to notify the tool pickup confirmation.
6. Send push notification to the other party (see Push Notification Pattern).
7. Return the updated rental request object.

### Change Rental Dates
Purpose: Allow either renter or owner to modify rental dates with appropriate approval workflow.

Input: `request_id`, `new_start_date`, `new_end_date`, `old_start_date`, `old_end_date`
Output: updated rental request object
Business Logic:
1. Verify the rental exists.
2. Extract `user_id` from JWT token.
3. Determine if user is renter or owner.
4. Check rental status and apply appropriate business rules:

**Case 1: Renter changes dates in PENDING, APPROVED, or SCHEDULED status**
   - Verify `user_id` is the renter.
   - Validate that `new_end_date` is strictly after `new_start_date` (minimum 1 day).
   - Calculate new `total_cost_cents` using the rental's price snapshot (`duration_unit`, `daily_price_cents`, `weekly_price_cents`, `monthly_price_cents`) stored on the rental record. Duration is `new_end_date - new_start_date` (end exclusive).
   - Update `rentals` with new start_date, end_date, and total_cost_cents.
   - Set status to 'PENDING' (requires owner re-approval).
   - Create a notification to the owner with attributes set to {topic:rental_date_change; rental:rental_id; tool_name:tool_name; start_date:new_start_date; end_date:new_end_date; old_start_date:old_start_date; old_end_date:old_end_date; purpose:"renter changed dates, requires re-approval"} (insert into `notifications`).
   - Send email to owner about date change requiring re-approval.
   - Send push notification to the owner (see Push Notification Pattern).

**Case 2: Owner changes dates in PENDING, APPROVED, or SCHEDULED status**
   - Verify `user_id` is the tool owner.
   - Validate that `new_end_date` is strictly after `new_start_date` (minimum 1 day).
   - Calculate new `total_cost_cents` using the rental's price snapshot. Duration is `new_end_date - new_start_date` (end exclusive).
   - Update `rentals` with new start_date, end_date, and total_cost_cents.
   - Set status to 'APPROVED' (requires renter confirmation).
   - Create a notification to the renter with attributes set to {topic:rental_date_change; rental:rental_id; tool_name:tool_name; start_date:new_start_date; end_date:new_end_date; old_start_date:old_start_date; old_end_date:old_end_date; purpose:"owner changed dates, requires confirmation"} (insert into `notifications`).
   - Send email to renter about date change requiring confirmation.
   - Send push notification to the renter (see Push Notification Pattern).

**Case 3: Renter extends return date in ACTIVE or OVERDUE status**
   - Verify `user_id` is the renter.
   - Verify only `new_end_date` is changed (start date cannot change for active rentals).
   - Validate that `new_end_date` is strictly after `start_date` (minimum 1 day).
   - Calculate new `total_cost_cents` using the rental's price snapshot. Duration is `new_end_date - start_date` (end exclusive).
   - Update `rentals` with new end_date and total_cost_cents.
   - Set status to 'RETURN_DATE_CHANGED'.
   - Create a notification to the owner with attributes set to {topic:return_date_change_request; rental:rental_id; old_date:old_end_date; new_date:new_end_date; purpose:"renter requests return date extension"} (insert into `notifications`).
   - Send email to owner about return date extension request.
   - Send push notification to the owner (see Push Notification Pattern).

5. Return the updated rental request object.

### Approve Return Date Change
Purpose: Owner approves a renter's request to extend the return date.

Input: `request_id`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'RETURN_DATE_CHANGED'.
2. Verify `user_id` is the tool owner.
3. Copy `end_date` to `last_agreed_end_date` (save the newly approved date for potential future rollback).
4. Update `rentals` status to 'ACTIVE' (or 'OVERDUE' if new end_date has passed).
5. Create a notification to the renter with attributes set to {topic:return_date_change_approved; rental:rental_id; tool_name:tool_name; purpose:"owner approved return date change."} (insert into `notifications`).
6. Send email to renter about approved return date extension.
7. Send push notification to the renter (see Push Notification Pattern).
8. Return the updated rental request object.

### Reject Return Date Change
Purpose: Owner rejects a renter's request to extend the return date and sets a new return date.

Input: `request_id`, `new_end_date`, `reason`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'RETURN_DATE_CHANGED'.
2. Verify `user_id` is the tool owner.
3. Validate `new_end_date`:
   - Must not be empty (mandatory field).
   - Must be different from the requested end_date in the RETURN_DATE_CHANGED request.
   - Must be a valid date in YYYY-MM-DD format.
   - If validation fails, return error with message "New end date is required and must be different from the requested date".
4. Update `rentals` status to 'RETURN_DATE_CHANGE_REJECTED'.
5. Store rejection `reason` in the `rejection_reason` field of the rental record.
6. Update `rentals.end_date` with the `new_end_date` set by owner.
7. Recalculate `total_cost_cents` using the rental's price snapshot (`duration_unit`, `daily_price_cents`, `weekly_price_cents`, `monthly_price_cents`) stored on the rental record. Duration is `new_end_date - start_date` (end exclusive), then update the record.
8. Create a notification to the renter with attributes set to {topic:return_date_change_rejected; rental:rental_id; rejection_reason:reason; new_end_date:new_end_date; old_end_date:old_end_date; purpose:"owner rejected return date extension and set new return date"} (insert into `notifications`).
9. Send email to renter about rejected return date extension with:
   - The rejection reason
   - The new return date set by the owner
   - The updated rental cost
10. Send push notification to the renter (see Push Notification Pattern).
11. Return the updated rental request object.

Note: The owner is required to set a counter-proposal return date when rejecting. This ensures the renter knows the actual expected return date.

### Acknowledge Return Date Rejection
Purpose: Renter acknowledges the owner's rejection of return date change and reverts to original terms.

Input: `request_id`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'RETURN_DATE_CHANGE_REJECTED'.
2. Verify `user_id` is the renter.
3. Copy `last_agreed_end_date` back to `end_date` (rollback to the last agreed date).
4. Recalculate `total_cost_cents` using the rental's price snapshot (`duration_unit`, `daily_price_cents`, `weekly_price_cents`, `monthly_price_cents`) stored on the rental record. Duration is `end_date - start_date` (end exclusive, after rollback).
5. Clear rejection_reason field.
6. Determine appropriate status based on current date vs end_date:
   - If current_date <= end_date: Set status to 'ACTIVE'
   - If current_date > end_date: Set status to 'OVERDUE'
7. Create a notification to the owner with attributes set to {topic:return_date_rejection_acknowledged; rental:rental_id; purpose:"renter acknowledged rejection"}.
8. Return the updated rental request object.

### Cancel Return Date Change
Purpose: Renter cancels their own pending return date change request.

Input: `request_id`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'RETURN_DATE_CHANGED'.
2. Verify `user_id` is the renter.
3. Copy `last_agreed_end_date` back to `end_date` (rollback to the last agreed date).
4. Recalculate `total_cost_cents` using the rental's price snapshot (`duration_unit`, `daily_price_cents`, `weekly_price_cents`, `monthly_price_cents`) stored on the rental record. Duration is `end_date - start_date` (end exclusive, after rollback).
5. Determine appropriate status based on current date vs end_date:
   - If current_date <= end_date: Set status to 'ACTIVE'
   - If current_date > end_date: Set status to 'OVERDUE'
6. Create a notification to the owner with attributes set to {topic:return_date_change_cancelled; rental:rental_id; purpose:"renter cancelled return date change request"} (insert into `notifications`).
7. Send email to owner about cancelled return date change request.
8. Send push notification to the owner (see Push Notification Pattern).
9. Return the updated rental request object.

### List Tool Rentals
Purpose: View complete rental history for a specific tool (for tool owners).

Input: `tool_id`, `organization_id`, status filter (array), pagination
Output: list of rental requests for the tool
Business Logic:
1. Extract `user_id` from JWT token.
2. Verify the tool exists and `user_id` is the owner of the tool.
3. Query `rentals` where `tool_id` matches the input.
4. Apply `organization_id` filter if provided.
5. Apply `status` filter if provided. Multiple statuses can be provided and should be applied with OR logic (match any of the given statuses). If status array is empty, return all statuses.
6. Order results by `created_on` DESC (most recent first).
7. Apply pagination (page, page_size).
8. Return list of rental requests with complete details:
   - Rental ID, status, dates, cost
   - Renter information (name, email)
   - Request and completion timestamps
9. Include summary metadata:
   - Total rental count for this tool
   - Count by status (completed, active, pending, etc.)

## Ledger

### Get Balance
Purpose: Check current credits in an organization.

Input: `organization_id`
Output: balance and last updated date
Business Logic:
1. Query `users_orgs` by the `user_id` and `organization_id`.
2. return users_orgs.balance_cents and user_orgs.last_balance_updated_on. 

### Get Transactions
Purpose: View credit history.

Input: `organization_id`, pagination
Output: transaction list
Business Logic:
1. Query `ledger_transactions` by the `user_id` and `organization_id`.

### Get Ledger Summary
Purpose: High-level overview of user's financial state in an org.

Input: `organization_id`, `number_of_months`
Output: balance, recent transactions, and activity counts
Business Logic:
1. Fetch balance from `users_orgs`, if `organization_id` is not given, rollup all the balances of the user in user_orgs table.
2. Retrieve rentals records of the user in the organization for the last `number_of_months`. If `organization_id` is not given, retrieve rental records from all the user's organizations.
3. Count each rental status from the retrieved rentals.

## Notifications

### Get Notifications
Purpose: View user alerts.

Input: pagination
Output: notification list
Business Logic:
1. Query `notifications` for the `user_id`.

### Mark Notification Read
Purpose: Clear an alert.

Input: `notification_id`
Output: success flag
Business Logic:
1. Validate the notification belongs to `user_id`.
2. Update `read_at = NOW()` only if `read_at` is currently NULL (first-write-wins): `UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id = $notification_id AND user_id = $user_id`.

### Sync Device Token
Purpose: Register or refresh an FCM push notification token for the user's device. Called by the mobile app on startup and whenever the FCM token is refreshed by the Android system.

Input: `fcm_token`, `android_device_id`, `device_name`
Output: empty
Business Logic:
1. Extract `user_id` from the JWT token.
2. Upsert into `fcm_tokens` matching on `fcm_token`:
   - If the token already exists: update `user_id`, `device_info` (from `device_name`), `status = 'ACTIVE'`, `updated_at = NOW()`. Reassigning `user_id` handles the case where a user re-logs in on the same device.
   - If the token does not exist: insert a new record with `status = 'ACTIVE'`.
3. Return empty response.

### Report Message Event
Purpose: Client reports a push notification lifecycle event. Called from the mobile app's FCM handlers: `onMessageReceived()` for delivery, `NotificationOpened` callback for click.

Input: `notification_id`, `event_type` ("DELIVERED" or "CLICKED"), `event_time`
Output: empty
Business Logic:
1. Extract `user_id` from the JWT token.
2. Validate `event_type` is one of: `DELIVERED`, `CLICKED`.
3. Verify `notification_id` exists in `notifications` and belongs to `user_id`.
4. Update the corresponding timestamp using first-write-wins (no override if already set):
   - `DELIVERED`: `UPDATE notifications SET delivered_at = COALESCE(delivered_at, $event_time) WHERE id = $notification_id AND user_id = $user_id`
   - `CLICKED`: `UPDATE notifications SET clicked_at = COALESCE(clicked_at, $event_time) WHERE id = $notification_id AND user_id = $user_id`
5. Return empty response.

## Image Storage

### Get Upload URL (Presigned URL Pattern)
Purpose: Generate a presigned URL for uploading an image to cloud storage (S3/GCS).

Input: `filename`, `content_type`, `tool_id`, `is_primary`
Output: `upload_url`, `image_id`, `download_url`, `expires_at`
Business Logic:
1. Extract `user_id` from JWT token in authorization header.
2. Generate a unique `image_id` (UUID).
3. Determine storage path: `images/{organization_id}/{tool_id}/{image_id}/{filename}` (or pending path if tool_id=0).
4. Generate presigned PUT URL for cloud storage (S3 or GCS) with 15-minute expiration.
5. Create a pending image record in `tool_images` table:
   - `id` = generated image_id
   - `tool_id` = tool_id (0 for new tools)
   - `user_id` = from JWT
   - `file_name` = filename
   - `file_path` = storage path
   - `mime_type` = content_type
   - `is_primary` = is_primary
   - `status` = 'PENDING'
   - `uploaded_on` = null
   - `expires_at` = current_time + 15 minutes
6. Generate presigned GET URL or CDN URL for downloading (optional, can be generated later).
7. Return:
   - `upload_url`: Presigned PUT URL for client to upload
   - `image_id`: UUID to reference this image
   - `download_url`: Public or presigned GET URL for downloading
   - `expires_at`: Unix timestamp when URLs expire (15 minutes)

### Confirm Image Upload
Purpose: Confirm that the client successfully uploaded the image to the presigned URL.

Input: `image_id`, `tool_id`, `file_size`
Output: `success`, `tool_image`, `message`
Business Logic:
1. Extract `user_id` from JWT token.
2. Find the pending image record by `image_id` and `user_id`.
3. Verify the image wasn't already confirmed (status != 'CONFIRMED').
4. Verify the pending record hasn't expired (`expires_at` > current_time).
5. Verify the file exists in cloud storage (HEAD request to S3/GCS).
6. If file doesn't exist, return error: "Image not found in storage. Please upload again."
7. Update the `tool_images` record:
   - `status` = 'CONFIRMED'
   - `file_size` = file_size
   - `uploaded_on` = current timestamp
   - `tool_id` = tool_id (if tool was created after getting upload URL)
8. If `tool_id` > 0:
   - Verify the tool exists and belongs to the user.
   - Link image to tool.
   - If `is_primary` is true, unset other images' `is_primary` flag for this tool.
9. Schedule async job to generate thumbnail (300x300).
10. Return success with complete `ToolImage` object including:
    - `id`, `tool_id`, `file_name`, `file_path`, `thumbnail_path`
    - `file_size`, `is_primary`, `display_order`, `uploaded_on`

### Get Download URL
Purpose: Get a presigned download URL for an image (for secure access) or return CDN URL.

Input: `image_id`, `tool_id`, `is_thumbnail`
Output: `download_url`, `expires_at`
Business Logic:
1. Extract `user_id` from JWT token.
2. Find the image record by `image_id` and `tool_id`.
3. Verify the tool is visible to the user:
   - If tool belongs to same organization as user → Allow
   - If tool is public and status='AVAILABLE' → Allow
   - Otherwise → Return PERMISSION_DENIED error
4. Determine file path:
   - If `is_thumbnail` = true: use `thumbnail_path`
   - Otherwise: use `file_path`
5. Generate presigned GET URL (1-hour expiration) or return public CDN URL.
6. Return:
   - `download_url`: Presigned GET URL or CDN URL
   - `expires_at`: Unix timestamp when URL expires (optional for CDN)

### Get Tool Images
Purpose: List all images for a specific tool.

Input: `tool_id`
Output: list of `ToolImage` objects
Business Logic:
1. Extract `user_id` from JWT token.
2. Verify the tool exists and user has access (same logic as Get Download URL).
3. Query `tool_images` where:
   - `tool_id` = tool_id
   - `status` = 'CONFIRMED'
   - Order by: `is_primary` DESC, `display_order` ASC, `uploaded_on` ASC
4. Return list of `ToolImage` objects with all metadata.
5. If no images found, return empty list (not an error).

### Delete Image (Updated)
Purpose: Delete an image and its associated files from storage.

Input: `image_id`, `tool_id`
Output: success flag
Business Logic:
1. Extract `user_id` from JWT token.
2. Search `tool_images` by `tool_id`, `user_id`, and `image_id`.
3. Verify the user owns the tool (tool.owner_id = user_id).
4. Delete the image and thumbnail files from cloud storage (S3/GCS).
5. Delete the record from `tool_images` table (or mark as deleted with `deleted_on` timestamp).
6. If this was the primary image and other images exist:
   - Set the oldest remaining image as primary.
7. Return success.

### Set Primary Image (Updated)
Purpose: Set a specific image as the primary image for a tool.

Input: `image_id`, `tool_id`
Output: success flag, message
Business Logic:
1. Extract `user_id` from JWT token.
2. Verify the user owns the tool (tool.owner_id = user_id).
3. Find the current primary image: Query `tool_images` by `tool_id`, `is_primary`=true, and `user_id`. Call it image1.
4. Find the target image: Query `tool_images` by `tool_id`, `user_id`, and `image_id`. Call it image2.
5. Verify image2 exists and belongs to this tool. If not, return error.
6. If image1 = image2, return success with message "Image is already the primary image."
7. Set image1.`is_primary` = false
8. Set image2.`is_primary` = true
9. Return success with message "Primary image updated successfully."

---

## Legacy Image Storage Methods (Deprecated)

### Upload Image (Streaming - Deprecated)
**Status:** Deprecated - Use presigned URL pattern instead (GetUploadUrl → Upload → ConfirmImageUpload)

Purpose: Upload image via gRPC streaming.

Input: stream of `UploadImageRequest` (first message contains metadata, subsequent messages contain chunks)
Output: `UploadImageResponse` with `ToolImage`
Business Logic:
1. Receive first message with metadata (tool_id, filename, mime_type, is_primary).
2. Stream subsequent chunks and write to temporary file.
3. Save to storage and create `tool_images` record.
4. Generate thumbnail.
5. Return ToolImage metadata.

**Note:** This method is kept for backward compatibility but presigned URL pattern is recommended for new implementations.

### Download Image (Streaming - Deprecated)
**Status:** Deprecated - Use GetDownloadUrl instead

Purpose: Download image via gRPC streaming.

Input: `image_id`, `tool_id`, `is_thumbnail`
Output: stream of `DownloadImageResponse` (first message contains metadata, subsequent messages contain chunks)
Business Logic:
1. Find image record and verify access.
2. Read file from storage.
3. Stream chunks to client.

**Note:** This method is kept for backward compatibility but presigned URL pattern (GetDownloadUrl) is recommended.

---

## Users

### Get User
Purpose: Get own profile details.

Input: none
Output: user profile including organizations
Business Logic:
1. Query `users` by `user_id` in JWT.
2. Query `users_orgs` and join with `orgs` to list memberships.

### Update Profile
Purpose: Change name, avatar, etc.

Input: `name`, `email`, `phone`, `avatar_url`
Output: updated user profile
Business Logic:
1. Update `users` table for the `user_id` in JWT.

