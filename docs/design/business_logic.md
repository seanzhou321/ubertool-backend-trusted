# Business Logic of Ubertool Trusted Backend

The business logic should be implemented in the service layer. 

## Authentication

### Validate Invite
Purpose: Users receives an invitation code from email. The UI calls this API method to validate the invitation code. If the invitation code is valid, the UI will show the signup form. If the invitation code is invalid, the UI will show the error message. 

Input: invitation code, email
Output: valid or not, error message 
Business Logic: 
1. Verify the invitation record with (`invitation_code`, `email`) in invitations table exists and not expired.
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
5. Find the admin users in `organization_id`. 
6. Send emails to the admin users with the new user email, name, and the message.
7. Create a list of notifications to each admin users that the email was sent.
8. Return success/failure and message, "Your request to join the organization has been submitted."

### User Signup
Purpose: Register a new user and join an organization simultaneously after the invitation code is validated.

Input: `invitation_code`, `name`, `email`, `phone`, `password`
Output: success, message
Business Logic:
1. Validate the `invitation_code` and `email` pair from invitations table (exists, not expired, and `used_on` is null).
2. Retrieves the `organization_id` from the invitations record. 
3. Search the user with email address from Users table.
4. If no user is found by the email, create a new user record in the `users` table with hashed password.
5. Update the `invitations` record's `used_on` field with the current timestamp.
6. Record the user's membership in the `users_orgs` table for the `organization_id`.
7. Return success/failure and message, "Your user account has been created (for new user) and successfully joined the `orgs.name`.

### Login
Purpose: Authenticate existing user.

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
3. Set `blocked_date` and `block_reason`.

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

### Search Organizations
Purpose: Find organizations to join.

Input: `name`, `metro`
Output: list of organizations
Business Logic:
1. Query `orgs` based on name and/or metro.

### Update Organization
Purpose: Modify organization settings.

Input: `organization_id`, `description`, `admin_email`, `admin_phone_number`.
Output: updated organization info
Business Logic:
1. Verify caller is an 'ADMIN' or 'SUPER_ADMIN'.
2. Update the `orgs` record.

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
2. Calculate `total_cost_cents` based on duration and tool price.
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
2. Deduct `total_cost_cents` from renter's balance in `users_orgs`.
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

Input: `request_id`, `start_date`, `end_date`
Output: updated rental status
Business Logic:
1. Either owner or renter can signal completion.
2. Verify the rental status is 'ACTIVE', 'SCHEDULED', or 'OVERDUE'. Report error if otherwise.
3. Calculate `total_cost_cents` according to the price and duration of the rental. 
4. The calculation of the cost is based on the none zero price fields and taking the lowest cost option. For example, if the tool has only weekly price and zero on daily price, if the rental duation is 3 days, it will be charged by the weekly price. However, if the tool has both weekly price and daily price and the weekly price is equivalent of 3 times of the daily price, if the duration is 4 days, the cost will be the weekly price because it is cheaper than 4 daily prices. By the same principle, if the duration is 8 days, the price will be one weekly price plus one daily price.
5. The duration calculation should include both the start and the end dates. For example, if start_date="Jan 2, 2026" and end_date="Jan 4, 2026", the duration is 3 to count both start and end dates.
6. Update `rentals` status to 'COMPLETED' and set `start_date`, `end_date`, `completed_by` to `user_id`, and `total_cost_cents`.
7. Add `total_cost_cents` to owner's balance in `users_orgs`.
8. Create a `ledger_transactions` entry of type 'LENDING_CREDIT' to the owner using the org_id from the rentals.org_id and `total_cost_cents` for the amount.
9. Update owner's user_org record by adding `total_cost_cents` to the balance_cents field and set the last_balance_updated_on to today.
10. Create a notification to the owner with attributes set to {topic:rental_credit_update; transaction:ledger_id; amount:total_cost_cents; rental:rental_id}
11. Send email to owner to inform the credit update from the rental.
12. Create a `ledger_transactions` record of type 'LENDING_DEBIT' to the renter using the org_id from the rentals.org_id
13. Update renter's user_org record by adding `total_cost_cents` to the balance_cents field and set the last_balance_updated_on to today.
14. Create a notification to the renter with attributes set to {topic:rental_debit_update; transaction:ledger_id; amount:total_cost_cents; rental:rental_id}
15. Send email to the renter to inform the debit update from the rental.
16. Set `tools.status` back to 'AVAILABLE' if the tool has no more 'ACTIVE' or 'SCHEDULED' rental requests. Otherwise, set to 'RENTED'
17. Create a notification to owner to inform the completion of the rental and the tool status change with attributes set to {topic:rental_completion; rental:rental_id}.
18. Send email to owner to inform the completion of the rental and the tool status change.
19. Create a notification to the renter to inform the completion of the rental with attributes set to {topic:rental_completion; rental:rental_id}.
20. Send email to renter to inform the completion of the rental and the tool status change.

### Get Rental
Purpose: View rental details.

Input: `request_id`
Output: rental details
Business Logic:
1. check user_id is either the renter, the owner, or an admin in the organization of rentals.org_id.

### List My Rentals
Purpose: View history/status of tools borrowed.

Input: `organization_id`, status filter
Output: list of rentals
Business Logic:
1. Filter the rental requests of the user as the renter by the status. 
2. If organization_is is given, filter only the requests from that organization.

### List My Lendings
Purpose: View history/status of tools lent to others.

Input: `organization_id`, status filter
Output: list of lendings
Business Logic:
1. Filter the rental requests of the user as the owner by the status. 
2. If organization_is is given, filter only the requests from that organization.

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

### Upload an image
Purpose: Securely upload a tool image.

Input: `tool_id`, `file_name`, `mime_type`, `is_primary`, `chunk`
Output: the new tool image record
Business Logic:
1. get configured file_path and thumbnail_path.
2. Create a new tool_images record
3. Streaming upload of the image.
4. calculate a thumbnail image
5. Save the image and its thumbnail into the file by their path and the file_name.
6. Return created the tool_image record.

### Get images of a tool
Purpose: Get all image of a tool.

Input: `tool_id`
Output: a list of tool image records
Business Logic:
1. Search tool_images by `tool_id` and `user_id`.

### Download an image
Purpose: Download an image.

Input: `image_id`, `tool_id`, `is_thumbnail`
Output: the tool_image object, streaming the image
Business Logic:
1. Search tool_images by `tool_id`, `user_id`, and `image_id`.
2. Return the tool_image object.
3. Streaming download of the image if is_thumbnail=false
4. Streaming download of the thumbnail of the image if is_thumbnail=true

### Delete an image
Purpose: Delete an image.

Input: `image_id`, `tool_id`
Output: success flag
Business Logic:
1. Search tool_images by `tool_id`, `user_id`, and `image_id`.
2. Delete the image and thumbnail files.

### Set primary image of a tool
Purpose: Delete an image.

Input: `image_id`, `tool_id`
Output: success flag, message
Business Logic:
1. Search tool_images by `tool_id`, `is_primary`=true, and `user_id`. Say it is image1
2. Search tool_images by `tool_id`, `user_id`, and `image_id`. Say it is image2
3. If image1=image2, return success with message indicating the image was already the primary.
4. Set image1.is_primary=false and image2.is_primary=true 


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
