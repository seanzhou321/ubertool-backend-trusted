# Go gRPC Backend: Push Notification Tracking Design

## 1. Database Schema (PostgreSQL)
To track message delivery and user devices, we maintain two core tables.

### `user_devices`
Tracks active FCM registration tokens per user.
- `fcm_token` (TEXT, PK): The unique registration token from Firebase.
- `user_id` (UUID, Indexed): The owner of the device.
- `device_info` (JSONB): Metadata (e.g., "Pixel 8", "Android 14").
- `last_seen_at` (TIMESTAMP): Updated whenever the token is used or refreshed.

### `notification_logs`
Tracks the lifecycle of every sent message.
- `internal_msg_id` (UUID, PK): Our internal tracking ID.
- `fcm_message_id` (TEXT): The ID returned by Firebase after a successful `Send()`.
- `user_id` (UUID): Target user.
- `status` (ENUM): 'SENT', 'DELIVERED', 'CLICKED', 'APP_OPENED'.
- `updated_at` (TIMESTAMP): Time of last status transition.

## 2. gRPC Service Definition
```proto
service PushTrackingService {
  // Client calls this on app start and token refresh
  rpc SyncDeviceToken(SyncTokenRequest) returns (google.protobuf.Empty);

  // Client calls this to report message lifecycle events
  rpc ReportMessageEvent(ReportEventRequest) returns (google.protobuf.Empty);
}

message SyncTokenRequest {
  string fcm_token = 1;
  string device_name = 2;
}

message ReportEventRequest {
  string internal_msg_id = 1;
  string event_type = 2; // "DELIVERED", "CLICKED"
}
```

## 3. Core Backend Logic

### Sending Notifications

1. Generate ID: Create a new `notification_id` (bigserial from the `notifications` table).
2. Payload: Construct an FCM **notification+data** message using the Firebase Admin Go SDK.
   - Set the `notification` field with `title` and `body` so the Android OS can render the
     notification automatically when the app is in the background or killed.
   - Set the `data` map with `notification_id` (as a string) and `channel_id` so the client
     routes to the correct notification channel and can report lifecycle events.
   - Set `AndroidConfig.Priority` based on the channel: `"high"` for channels 1–3 (wakes the
     device in Doze), `"normal"` for channels 4–5 (delivered when the device is active).
   - Go SDK example:
     ```go
     // Determine FCM priority from channel.
     fcmPriority := "normal"
     if channel == "rental_request_messages" ||
         channel == "billsplitting_messages" ||
         channel == "dispute_messages" {
         fcmPriority = "high"
     }

     message := &messaging.Message{
         Notification: &messaging.Notification{
             Title: title,
             Body:  body,
         },
         Data: map[string]string{
             "notification_id": strconv.FormatInt(notificationID, 10),
             "channel_id":      channel, // routes to the correct Android channel
         },
         Token: fcmToken,
         Android: &messaging.AndroidConfig{
             Priority: fcmPriority,
         },
     }
     ```

### Notification Channels

The backend must supply `channel_id` in the `data` map. The Android client registers these
channels at startup; users may adjust them in Settings → App notifications.

| `channel_id` | Label | Priority | Behavior |
|---|---|---|---|
| `rental_request_messages` | Rental Request Messages | HIGH | Heads-up; wakes device from sleep (Doze bypass) |
| `billsplitting_messages` | Billsplitting events notifications | HIGH | Heads-up; wakes device from sleep |
| `dispute_messages` | Rental dispute notifications | HIGH | Heads-up; wakes device from sleep |
| `admin_messages` | Administrative event messages | DEFAULT | Appears in notification shade; no heads-up or wake |
| `app_messages` | General App events | DEFAULT | Appears in notification shade; no heads-up or wake |

If `channel_id` is omitted or unrecognised, the client defaults to `app_messages`.

#### Messages per channel

| `channel_id` | Notification Title | Recipient | Trigger |
|---|---|---|---|
| `rental_request_messages` | New Rental Request | Tool owner | Renter submits a rental request |
| `rental_request_messages` | Rental Approved | Renter | Owner approves a rental request |
| `rental_request_messages` | Rental Rejected | Renter | Owner rejects a rental request |
| `rental_request_messages` | Rental Cancelled | Tool owner | Renter cancels a rental request |
| `rental_request_messages` | Rental Confirmed | Tool owner | Renter finalises / confirms the rental |
| `rental_request_messages` | Rental Picked Up | Other party | Either party confirms tool pick-up (rental activated) |
| `rental_request_messages` | Rental Dates Changed | Tool owner | Renter changes dates on a pre-active rental (requires re-approval) |
| `rental_request_messages` | Rental Dates Updated | Renter | Owner updates dates on a pre-active rental (requires renter confirmation) |
| `rental_request_messages` | Return Date Extension Request | Tool owner | Renter requests to extend the return date on an active rental |
| `rental_request_messages` | Extension Request Updated | Tool owner | Renter updates their pending extension request |
| `rental_request_messages` | Extension Approved | Renter | Owner approves an extension request |
| `rental_request_messages` | Extension Rejected - Counter-Proposal | Renter | Owner rejects the extension and proposes a new return date |
| `rental_request_messages` | Rejection Acknowledged | Tool owner | Renter acknowledges the extension rejection / rolls back to last agreed date |
| `rental_request_messages` | Extension Request Cancelled | Tool owner | Renter cancels their pending extension request |
| `billsplitting_messages` | Payment Acknowledged | Creditor | Debtor marks a bill as paid |
| `billsplitting_messages` | Payment Receipt Confirmed | Debtor | Creditor confirms receipt of payment (bill marked PAID) |
| `dispute_messages` | Dispute Resolved | Debtor | Admin resolves a disputed bill |
| `dispute_messages` | Dispute Resolved | Creditor | Admin resolves a disputed bill |
| `admin_messages` | New Join Request | Org admin | User submits a request to join an organisation |
3. DB Entry: Insert a record into `notifications` with status `SENT`.
4. Execute: Call `messaging.Send(ctx, message)`.
5. Use a goroutine or a worker queue to send the message without blocking other processes.
6. Embed the `notifications.id` value in the `data` map as `notification_id`. When the app
   reports back, this becomes `ReportEventRequest.notification_id` in `notification_service.proto`.

### Android Client Lifecycle Behavior

The use of notification+data messages means the delivery path differs depending on app state:

| App state | Who renders the notification | DELIVERED / CLICKED reported by |
|---|---|---|
| **Foreground** | `onMessageReceived()` calls `showNotification()` → `NotificationOpenedReceiver` handles tap | `UberToolFirebaseMessagingService` (DELIVERED) + `NotificationOpenedReceiver` (CLICKED) |
| **Background / killed** | Android OS renders automatically from the `notification` field; `onMessageReceived()` is **not** called | `MainActivity.handleFcmNotificationTapIfPresent()` reports both DELIVERED and CLICKED on tap |

When the OS renders a background notification and the user taps it, FCM delivers the `data` map
keys as String extras on the `MainActivity` launch intent. `handleFcmNotificationTapIfPresent()`
reads `notification_id` from those extras and enqueues DELIVERED + CLICKED via WorkManager.
`onNewIntent()` covers the case where the activity is already running and brought to front.

### Token Maintenance

- If messaging.Send() returns an Unregistered error, immediately DELETE that token from user_devices.

### Manage fcm_token lifecycle

- When receive messaging.IsUnregistered(err) ie Http 404 from FCM, stamp the fcm_tokens.status='OBSOLETE'. 
- notifications.delivered_at, clicked_at, app_opened_at, read_at in postgresql database should only take the first value entered, ie. they should record the first timestamp of the event; no overrides. 