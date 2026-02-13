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
7. Create a list of notifications to each admin users that the email was sent.
8. Return success/failure and message, "Your request to join the organization has been submitted."

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
7. Create a record in the `users_orgs` table with `user_id`, `organization_id`, and role 'MEMBER'.
8. Initialize user's balance in the organization (balance_cents = 0).
9. Return success and message "Your account has been created. Please log in."

Note: User must go through normal login process after signup. Signup does NOT return authentication tokens.

### Login
Purpose: Authenticate existing user and initiate two-factor authentication.

Input: `email`, `password`
Output: success/failure boolean, temporary_token, expires_at, message
Business Logic:
1. Fetch user by `email`.
2. Verify `password` against hashed password in database.
3. If valid, generate a two_fa_code, send the 2FA code by email to the user.
4. If valid, generate a temporary_token and return the temporary_token and milisecond time stamp of expires at.
5. If not valid, return false and a message, "Either the email and/or the password is wrong".

### Verify 2FA
Purpose: Complete authentication with a second factor.

Input: `two_fa_code`
Output: bool, access token, refresh token, user profile
Business Logic:
1. Validate the `two_fa_code` with previously generated one in the session.
2. If match, Generate and return JWT tokens (access_token, refresh_token) and the user profile.

### Refresh Token
Purpose: Get a new access token using a refresh token.

Input: `refresh_token`
Output: new access token, new refresh token
Business Logic:
1. Validate the `refresh_token`.
2. If valid, issue a new access token and a new refresh token.
3. if fail, output a warning security log.

### Logout
Purpose: Invalidate the current session.

Input: none
Output: success flag
Business Logic:
1. Invalidate/blacklist the current JWT tokens (access and refresh tokens).

## Administration

### Approve Request To Join
Purpose: Admin approves a pending request to join an organization.

Input: `organization_id`, `applicant_email`, `applicant_name`
Output: success/failure, message
Business Logic:
1. Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Find the pending request in `join_requests`.
3. If the user already exists in `users` table (by email), add them to `users_orgs` with 'MEMBER' role.
4. If the user does not exist, creates an invitation record in `invitations` and send the invitation code to the new user, cc to the admin user (`user_id` parsed from this JWT token).
5. Update `join_requests` status to 'APPROVED'.

### Admin Block User Account
Purpose: Admin suspends or blocks a member's access.

Input: `blocked_user_id`, `organization_id`, `reason`
Output: success, error_message
Business Logic:
1. Verify caller admin rights.
2. Update the `status` field in `users_orgs` to `BLOCK`.
3. Set `blocked_on` and `blocked_reason`.

### List Members
Purpose: List all members of an organization.

Input: `organization_id`
Output: list of member profiles
Business Logic:
1. Verify user is either `SUPER_ADMIN` or `ADMIN`
2. Join `users_orgs` and `users` to return member details and their current balance in that org.

### Search Users
Purpose: Search for specific members within an organization.

Input: `organization_id`, `query`
Output: list of matching member profiles
Business Logic:
1. Filter members by name or email using the `query` string in the `organization_id`.

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

Input: `organization_id`, `name`, `description`, `address`, `metro`, `admin_email`, `admin_phone`
Output: updated organization info
Business Logic:
1. Verify user is `SUPER_ADMIN` of the organization.
2. Update the organization record in `orgs` table.
3. Return updated organization details.

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
    - Notify Creditor.
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
    - Notify Debtor.
5. **Graceful Resolution after Dispute**: If bill was in DISPUTED status and both parties acknowledge, it resolves as GRACEFUL without admin intervention.

### List Disputed Payments
Purpose: Admin lists unresolved disputes requiring intervention.

Input: `organization_id`, `pagination` (optional)
Output: List of DisputedPaymentItem, PaginationResponse
Business Logic:
1. Verify user is ADMIN/SUPER_ADMIN of `organization_id`.
2. Query payments with status = DISPUTED (unresolved only).
3. Filter out disputes involving the current admin (Admins cannot resolve their own disputes).
4. Map to DisputedPaymentItem with dispute details in bills table.
5. Apply pagination:
    - If `pagination.page_size` not provided, use default (50).
    - If `pagination.page_token` provided, continue from that position.
    - Return `next_page_token` if more results exist.
    - Return `total_count` of all matching disputes.

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
10. Notify both debtor and creditor of resolution.
11. Send email to both parties with resolution outcome and notes.

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

Input: `name`, `description`, `categories`, prices, `condition`, `image_url`
Output: the created tool object
Business Logic:
1. Insert into `tools` table.
2. The `owner_id` is the `user_id` from JWT token.
2. Insert `image_url` into `tool_images`.

### Update Tool
Purpose: Update the content of a tool.

Input: `tool_id`, and all updated fields, except id, owner_id, created_on, deleted_on fields.
Output: the updated tool object
Business Logic:
1. Verify the current user is the owner of the tool.
2. Update `tools` and `tool_images`.

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
2. Calculate `total_cost_cents` based on duration from start_date to end_date and tool price.
3. Insert into `rentals` with status 'PENDING'.
4. Create a notification to the owner with attributes set to {topic:rental_request; rental:rental_id; purpose:"request for approval"}
5. Send an email to the tool owner to notify the rental request, cc to the renter (user_id parsed from the JWT token).

### Approve Rental Request
Purpose: Owner approves the lending.

Input: `request_id`, `pickup_instructions`
Output: updated rental request object
Business Logic:
1. Verify `user_id` is the tool owner.
2. Update `rentals` status to 'APPROVED'.
3. Create a notification to the renter with attributes set to {topic:rental_request; rental:rental_id, purpose:"rental request approved"}
4. Send an email to the renter to notify the rental approval with `pickup_instruction`, cc to the owner.

### Reject Rental Request
Purpose: Owner declines the lending.

Input: `request_id`, `reason`
Output: success, updated rental request object
Business Logic:
1. Verify `user_id` is the tool owner.
2. Update `rentals` status to 'REJECTED'.
3. Create a notification to the renter with attributes set to {topic:rental_request; rental:rental_id; purpose:"rental request rejected"}
4. Send an email to the renter to notify the rental rejection with `reason`, cc to the owner (user_id parsed from the JWT token). 

### Finalize Rental Request
Purpose: Renter confirms and pays for the rental.

Input: `request_id`
Output: updated rental request object, list of approved rental request objects, list of pending rental objects of the same type of tools
Business Logic:
1. Verify `user_id` is the renter.
2. Copy `end_date` to `last_agreed_end_date` (save the agreed-upon date for potential rollback).
3. Update `rentals` status to 'SCHEDULED'.
4. Update the tool status to 'RENTED'.
5. Create a notification to the owner with attributes set to {topic: rental_request; rental:rental_id; purpose:"rental request confirmed"}
5. Send an email to the owner to notify the rental confirmation, cc to the renter (user_id parsed from the JWT token).
6. Search approved rental requests of the renter for the same kind of tool.
7. Search pending rental requests of the renter for the same kind of tool. 

### Cancel Rental Request
Purpose: Renter cancels the rental request.

Input: `request_id`, `reason`
Output: success, updated rental request object
Business Logic:
1. Verify `user_id` is the tool renter.
2. Update `rentals` status to 'CANCELED'.
3. Create a notification to the owner with attributes set to {topic: rental_request; rental:rental_id; purpose:"rental request canceled"}
4. Send an email to the owner to notify the cancelation of the rental request with `reason`, cc to the renter (user_id parsed from the JWT token). 

### Complete Rental
Purpose: Mark tool as returned.

Input: `request_id`, `return_condition`, `surcharge_or_credit_cents`
Output: updated rental status
Business Logic:
1. Either owner or renter can signal completion.
2. Verify the rental status is 'ACTIVE', 'SCHEDULED', or 'OVERDUE'. Report error if otherwise.
3. Calculate `total_cost_cents` based on duration from start_date to end_date and tool price. 
4. The calculation of the cost is based on a tiered pricing structure. The owner can set the duration unit to monthly, weekly, or daily. Please refer tool-rental-pricing-instruction.md and tool-rental-pricing-algorith.md files for detail. 
5. Update `rentals` status to 'COMPLETED' and set `completed_by` to `user_id`, `return_condition`, `surcharge_or_credit_cents`, and `total_cost_cents`.
6. Add `total_cost_cents`+`surcharge_or_credit_cents` to owner's balance in `users_orgs`.
7. Create a `ledger_transactions` entry of type 'LENDING_CREDIT' to the owner using the org_id from the rentals.org_id and `total_cost_cents`+`surcharge_or_credit_cents` for the amount.
8. Update owner's user_org record by adding `total_cost_cents` to the balance_cents field and set the last_balance_updated_on to today.
9. Create a notification to the owner with attributes set to {topic:rental_credit_update; transaction:ledger_id; amount:total_cost_cents; rental:rental_id}
10. Send email to owner to inform the credit update from the rental.
11. Create a `ledger_transactions` record of type 'LENDING_DEBIT' to the renter using the org_id from the rentals.org_id
12. Update renter's user_org record by adding `total_cost_cents`+`surcharge_or_credit_cents` to the balance_cents field and set the last_balance_updated_on to today.
13. Create a notification to the renter with attributes set to {topic:rental_debit_update; transaction:ledger_id; amount:total_cost_cents; rental:rental_id}
14. Send email to renter to inform the debit update from the rental.
15. Set `tools.status` back to 'AVAILABLE' if the tool has no more 'ACTIVE' or 'SCHEDULED' rental requests. Otherwise, set to 'RENTED'
16. Create a notification to owner to inform the completion of the rental and the tool status change with attributes set to {topic:rental_completion; rental:rental_id}.
17. Send email to owner to inform the completion of the rental and the tool status change.
18. Create a notification to the renter to inform the completion of the rental with attributes set to {topic:rental_completion; rental:rental_id}.
19. Send email to renter to inform the completion of the rental and the tool status change.

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
4. Create a notification to the other party (if renter initiated, notify owner; if owner initiated, notify renter) with attributes set to {topic:rental_pickup; rental:rental_id; tool_name:tool_name; start_date:start_date; end_date:end_date; purpose:"rental has been picked up"}.
5. Send an email to the other party to notify the tool pickup confirmation.
6. Return the updated rental request object.

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
   - Calculate new `total_cost_cents` based on duration from new start_date to new end_date and tool pricing.
   - Update `rentals` with new start_date, end_date, and total_cost_cents.
   - Set status to 'PENDING' (requires owner re-approval).
   - Create a notification to the owner with attributes set to {topic:rental_date_change; rental:rental_id; tool_name:tool_name; start_date:new_start_date; end_date:new_end_date; old_start_date:old_start_date; old_end_date:old_end_date; purpose:"renter changed dates, requires re-approval"}.
   - Send email to owner about date change requiring re-approval.

**Case 2: Owner changes dates in PENDING, APPROVED, or SCHEDULED status**
   - Verify `user_id` is the tool owner.
   - Calculate new `total_cost_cents` based on duration from new start_date to new end_date and tool pricing.
   - Update `rentals` with new start_date, end_date, and total_cost_cents.
   - Set status to 'APPROVED' (requires renter confirmation).
   - Create a notification to the renter with attributes set to {topic:rental_date_change; rental:rental_id; tool_name:tool_name; start_date:new_start_date; end_date:new_end_date; old_start_date:old_start_date; old_end_date:old_end_date; purpose:"owner changed dates, requires confirmation"}.
   - Send email to renter about date change requiring confirmation.

**Case 3: Renter extends return date in ACTIVE or OVERDUE status**
   - Verify `user_id` is the renter.
   - Verify only `new_end_date` is changed (start date cannot change for active rentals).
   - Calculate new `total_cost_cents` based on duration from start_date to new end_date and tool pricing.
   - Update `rentals` with new end_date and total_cost_cents.
   - Set status to 'RETURN_DATE_CHANGED'.
   - Create a notification to the owner with attributes set to {topic:return_date_change_request; rental:rental_id; old_date:old_end_date; new_date:new_end_date; purpose:"renter requests return date extension"}.
   - Send email to owner about return date extension request.

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
5. Create a notification to the renter with attributes set to {topic:return_date_change_approved; rental:rental_id; tool_name:tool_name; purpose:"owner approved return date change."}.
6. Send email to renter about approved return date extension.
7. Return the updated rental request object.

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
7. Recalculate `total_cost_cents` based on duration from start_date to new_end_date and tool pricing, then update the record.
8. Create a notification to the renter with attributes set to {topic:return_date_change_rejected; rental:rental_id; rejection_reason:reason; new_end_date:new_end_date; old_end_date:old_end_date; purpose:"owner rejected return date extension and set new return date"}.
9. Send email to renter about rejected return date extension with:
   - The rejection reason
   - The new return date set by the owner
   - The updated rental cost
10. Return the updated rental request object.

Note: The owner is required to set a counter-proposal return date when rejecting. This ensures the renter knows the actual expected return date.

### Acknowledge Return Date Rejection
Purpose: Renter acknowledges the owner's rejection of return date change and reverts to original terms.

Input: `request_id`
Output: updated rental request object
Business Logic:
1. Verify the rental exists and status is 'RETURN_DATE_CHANGE_REJECTED'.
2. Verify `user_id` is the renter.
3. Copy `last_agreed_end_date` back to `end_date` (rollback to the last agreed date).
4. Recalculate `total_cost_cents` based on duration from start_date to end_date (now restored) and tool pricing.
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
4. Recalculate `total_cost_cents` based on duration from start_date to end_date (now restored) and tool pricing.
5. Determine appropriate status based on current date vs end_date:
   - If current_date <= end_date: Set status to 'ACTIVE'
   - If current_date > end_date: Set status to 'OVERDUE'
6. Create a notification to the owner with attributes set to {topic:return_date_change_cancelled; rental:rental_id; purpose:"renter cancelled return date change request"}.
7. Send email to owner about cancelled return date change request.
8. Return the updated rental request object.

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
1. Validate the notification is for `user_id`.
2. Update `is_read = TRUE` for the given notification ID.

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

