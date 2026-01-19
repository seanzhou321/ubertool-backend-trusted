# Business Logic of Ubertool Trusted Backend

The business logic should be implemented in the service layer. 

## Authentication

### Validate Invite
Purpose: Users receives an invitation code from email. The UI calls this API method to validate the invitation code. If the invitation code is valid, the UI will show the signup form. If the invitation code is invalid, the UI will show the error message. 

Input: invitation code, email
Output: valid or not, error message 
Business Logic: 
1. Verify the invitation record with (`invitation_code`, `email`) exists and not expired.
2. Return true if validation success, otherwise return false and error message, stating the "invitation code and email pair is invalid or expired."

### Request To Join Organization
Purpose: A user who is not part of an organization wants to join. They search for the organization and submit a request.

Input: `organization_id`, `name`, `email`, `message`
Output: success/failure, message
Business Logic:
1. Verify the organization exists in the `orgs` table.
2. Search the `users` table for the user with the given `email`.
3. Create a new entry in the `join_requests` table with `status` set to 'PENDING'.
4. The `user_id` in `join_requests` may be assigned from the user found by the email, or should be null if the user is not found.
5. Return success/failure and message, "Your request to join the organization has been submitted."

### User Signup
Purpose: Register a new user and join an organization simultaneously after the invitation code is validated.

Input: `invitation_code`, `name`, `email`, `phone`, `password`
Output: access token, refresh token, user profile
Business Logic:
1. Validate the `invitation_code` and `email` pair (exists, not expired, and `used_on` is null).
2. Create a new user record in the `users` table with hashed password.
3. Update the `invitations` record's `used_on` field with the current timestamp.
4. Record the user's membership in the `users_orgs` table for the organization specified in the invitation.
5. Generate and return access token, refresh token, user profile.

### Login
Purpose: Authenticate existing user.

Input: `email`, `password`
Output: session ID, requires 2FA flag
Business Logic:
1. Fetch user by `email`.
2. Verify `password` against hashed password in database.
3. If valid, generate JWT tokens and require 2FA. 

### Verify 2FA
Purpose: Complete authentication with a second factor.

Input: `session_id`, `code`
Output: access token, refresh token, user profile
Business Logic:
1. Validate the `session_id` and the 2FA `code`.
2. Generate and return JWT tokens.

### Refresh Token
Purpose: Get a new access token using a refresh token.

Input: `refresh_token`
Output: new access token, new refresh token
Business Logic:
1. Validate the `refresh_token`.
2. Issue a new access token and a new refresh token.

### Logout
Purpose: Invalidate the current session.

Input: none
Output: success flag
Business Logic:
1. Invalidate/blacklist the current tokens or session.

## Administration

### Approve Request To Join
Purpose: Admin approves a pending request to join an organization.

Input: `organization_id`, `applicant_email`, `applicant_name`
Output: success/failure, message
Business Logic:
1. Verify the caller has 'ADMIN' or 'SUPER_ADMIN' role in the given `organization_id`.
2. Find the pending request in `join_requests`.
3. If the user already exists in `users` table (by email), add them to `users_orgs` with 'MEMBER' role.
4. If the user does not exist, creates an invitation record in `invitations` and notify them.
5. Update `join_requests` status to 'APPROVED'.

### Admin Block User Account
Purpose: Admin suspends or blocks a member's access.

Input: `blocked_user_id`, `organization_id`, `is_block`, `reason`
Output: success/failure
Business Logic:
1. Verify caller admin rights.
2. Update the `status` field in `users_orgs` to 'SUSPEND' or 'BLOCK' if `is_block` is true.
3. Set `blocked_date` and `block_reason`.

### List Members
Purpose: List all members of an organization.

Input: `organization_id`
Output: list of member profiles
Business Logic:
1. Verify caller belongs to the organization.
2. Join `users_orgs` and `users` to return member details and their current balance in that org.

### Search Users
Purpose: Search for specific members within an organization.

Input: `organization_id`, `query`
Output: list of matching member profiles
Business Logic:
1. Filter members by name or email using the `query` string.

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
2. Add the creator as 'SUPER_ADMIN' in `users_orgs` with `balance_cents = 0`.

### Search Organizations
Purpose: Find organizations to join.

Input: `name`, `metro`
Output: list of organizations
Business Logic:
1. Query `orgs` based on name or metro.

### Update Organization
Purpose: Modify organization settings.

Input: `organization_id`, `name`, `description`, etc.
Output: updated organization info
Business Logic:
1. Verify caller is an 'ADMIN' or 'SUPER_ADMIN'.
2. Update the `orgs` record.

## Tools

### List Tools
Purpose: Browse tools available in an organization.

Input: `organization_id`, pagination
Output: tool list
Business Logic:
1. Query `tools` associated with the organization's metro or specific filters.
2. Note: Schema doesn't directly link `tools` to `orgs`, so tools are usually per-metro or per-owner. Need to clarify if tools are "in" an organization or just shared by members.
3. Return active tools (`deleted_on` is null).

### Get Tool
Purpose: View specific tool details.

Input: `tool_id`
Output: tool details including images
Business Logic:
1. Query `tools` and join with `tool_images`.
2. Return owner information.

### Add Tool
Purpose: List a new tool for rent.

Input: `name`, `description`, `categories`, prices, `condition`, `image_url`
Output: created tool info
Business Logic:
1. Insert into `tools` table with `owner_id` from header.
2. Insert `image_url` into `tool_images`.

### Update Tool
Purpose: Edit tool listing.

Input: `tool_id`, and updated fields
Output: updated tool info
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

Input: `organization_id`, `query`, `categories`, `max_price`, `condition`
Output: matching tool list
Business Logic:
1. Filter tools by search term, categories, price range, and condition.

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
1. Verify the tool is 'AVAILABLE'.
2. Calculate `total_cost_cents` based on duration and tool price.
3. Verify renter has enough `balance_cents` in the specified organization.
4. Insert into `rentals` with status 'PENDING'.

### Approve Rental Request
Purpose: Owner approves the lending.

Input: `request_id`, `pickup_instructions`
Output: updated rental status
Business Logic:
1. Verify caller is the tool owner.
2. Update `rentals` status to 'APPROVED' and set `pickup_note`.

### Reject Rental Request
Purpose: Owner declines the lending.

Input: `request_id`, `reason`
Output: success, updated rental status
Business Logic:
1. Verify caller is the tool owner.
2. Update `rentals` status to 'CANCELLED'.

### Finalize Rental Request
Purpose: Renter confirms and pays for the rental.

Input: `request_id`
Output: updated rental status
Business Logic:
1. Verify caller is the renter.
2. Deduct `total_cost_cents` from renter's balance in `users_orgs`.
3. Create a `ledger_transactions` entry of type 'RENTAL_DEBIT'.
4. Update `rentals` status to 'SCHEDULED' or 'ACTIVE' (depending on start date).

### Complete Rental
Purpose: Mark tool as returned.

Input: `request_id`
Output: updated rental status
Business Logic:
1. Either owner or renter can signal completion.
2. Update `rentals` status to 'COMPLETED' and set `end_date`.
3. Add `total_cost_cents` (minus platform fees if any) to owner's balance in `users_orgs`.
4. Create a `ledger_transactions` entry of type 'LENDING_CREDIT'.
5. Set `tools.status` back to 'AVAILABLE'.

### Get Rental
Purpose: View rental details.

Input: `request_id`
Output: rental details

### List My Rentals
Purpose: View history/status of tools borrowed.

Input: `organization_id`, status filter
Output: list of rentals

### List My Lendings
Purpose: View history/status of tools lent to others.

Input: `organization_id`, status filter
Output: list of lendings

## Ledger

### Get Balance
Purpose: Check current credits in an organization.

Input: `organization_id`
Output: balance and last updated date
Business Logic:
1. Query `users_orgs` for the current user and organization.

### Get Transactions
Purpose: View credit history.

Input: `organization_id`, pagination
Output: transaction list
Business Logic:
1. Query `ledger_transactions` for the user and organization.

### Get Ledger Summary
Purpose: High-level overview of user's financial state in an org.

Input: `organization_id`
Output: balance, recent transactions, and activity counts
Business Logic:
1. Fetch balance from `users_orgs`.
2. Query last 5 `ledger_transactions`.
3. Count active rentals and lendings from `rentals` table.

## Notifications

### Get Notifications
Purpose: View user alerts.

Input: pagination
Output: notification list
Business Logic:
1. Query `notifications` for the current user.

### Mark Notification Read
Purpose: Clear an alert.

Input: `notification_id`
Output: success flag
Business Logic:
1. Update `is_read = TRUE` for the given notification ID.

## Image Storage

### Get Upload URL
Purpose: Securely upload images.

Input: `filename`, `content_type`
Output: presigned upload URL, download URL
Business Logic:
1. Generate a presigned URL using S3 or GCS API.

## Users

### Get User
Purpose: Get own profile details.

Input: none
Output: user profile including organizations
Business Logic:
1. Query `users` for basic info.
2. Query `users_orgs` and join with `orgs` to list memberships.

### Update Profile
Purpose: Change name, avatar, etc.

Input: `name`, `email`, `phone`, `avatar_url`
Output: updated user profile
Business Logic:
1. Update `users` table for the current user.
