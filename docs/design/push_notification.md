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
- `notification_id` (bigserial, PK): Stable numeric ID embedded in the FCM `data` map.
- `fcm_message_id` (TEXT): The ID returned by Firebase after a successful `Send()`.
- `user_id` (UUID): Target user.
- `status` (ENUM): `SENT`, `DELIVERED`, `CLICKED`.
- `delivered_at`, `clicked_at` (TIMESTAMPTZ): First-write-only — never overwritten once set.
- `updated_at` (TIMESTAMPTZ): Time of last status transition.

## 2. gRPC Service Definition
```proto
service PushTrackingService {
  // Client calls this on app start and token refresh
  rpc SyncDeviceToken(SyncTokenRequest) returns (google.protobuf.Empty);

  // Client calls this to report message lifecycle events
  rpc ReportMessageEvent(ReportEventRequest) returns (google.protobuf.Empty);
}

message SyncTokenRequest {
  string fcm_token    = 1;
  string device_id    = 2; // FirebaseInstallations.getId() — stable per app install
  string device_name  = 3; // e.g. "Google Pixel 8"
}

message ReportEventRequest {
  int64  notification_id = 1; // bigserial from the notifications table
  string event_type      = 2; // "DELIVERED" or "CLICKED"
}
```

## 3. Core Backend Logic

### Sending Notifications

> ⚠️ **Data-only messages are mandatory.** The `Notification` field in the FCM message
> **MUST be omitted**. See [Why data-only?](#why-data-only) below.

1. Generate ID: Create a new `notification_id` (bigserial from the `notifications` table).
2. Payload: Construct a **data-only** FCM message using the Firebase Admin Go SDK.
   - Omit the `Notification` field entirely — `title` and `body` go in the `data` map.
   - Set the `data` map with `notification_id`, `channel_id`, `title`, and `body`.
   - Set `AndroidConfig.Priority` based on the channel: `"high"` for `rental_request_messages`
     (wakes the device in Doze), `"normal"` for all other channels (delivered when the device is active).
   - Go SDK example:
     ```go
     // Determine FCM priority from channel.
     fcmPriority := "normal"
     if channel == "rental_request_messages" {
         fcmPriority = "high"
     }

     message := &messaging.Message{
         // DO NOT set Notification field — data-only so onMessageReceived is
         // always called regardless of app state (foreground / background / killed).
         Data: map[string]string{
             "notification_id": strconv.FormatInt(notificationID, 10),
             "channel_id":      channel, // routes to the correct Android channel
             "title":           title,
             "body":            body,
         },
         Token: fcmToken,
         Android: &messaging.AndroidConfig{
             Priority: fcmPriority,
         },
     }
     ```

#### Why data-only?

FCM has three message types:

| Type | `Notification` field | `onMessageReceived` called when app is killed? | Sound/UI control |
|---|---|---|---|
| Notification message | Present | ❌ OS renders directly | OS-controlled |
| **Data-only message** | **Absent** | ✅ **Always called** | **App-controlled** |
| Notification + data | Both present | ❌ OS renders directly | OS-controlled |

When a `Notification` field is included and the app is **killed (swiped away)**:
- `onMessageReceived` is never called
- The OS renders the notification silently (no sound, no heads-up banner, dot only)
- The app cannot track `notification_id`, report `DELIVERED`, or trigger a data reload

When data-only is used:
- `onMessageReceived` fires for **all app states** — the app builds the notification itself
- Sound, vibration, heads-up banner, `BigTextStyle`, and lifecycle tracking all work correctly

### Notification Channels

The backend must supply `channel_id` in the `data` map. The Android client registers these
channels at startup; users may adjust them in Settings → App notifications.

| `channel_id` | Label | Priority | Behavior |
|---|---|---|---|
| `rental_request_messages` | Rental Request Messages | HIGH | Heads-up; wakes device from sleep (Doze bypass) |
| `billsplitting_messages` | Billsplitting events notifications | DEFAULT | Appears in notification shade; no heads-up or wake |
| `dispute_messages` | Rental dispute notifications | DEFAULT | Appears in notification shade; no heads-up or wake |
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

### Android Client Lifecycle Behavior

With data-only messages, `onMessageReceived` is **always called** regardless of app state.
The split-path behaviour of notification+data messages no longer applies.

| App state | Who renders the notification | DELIVERED / CLICKED reported by |
|---|---|---|
| **Foreground** | `onMessageReceived()` → `showNotification()` | `UberToolFirebaseMessagingService` (DELIVERED) + `MainActivity.handleFcmNotificationTapIfPresent()` (CLICKED) |
| **Background** | `onMessageReceived()` → `showNotification()` | Same as foreground |
| **Killed (swiped away)** | `onMessageReceived()` → `showNotification()` | Same as foreground — CLICKED reported by `MainActivity.handleFcmNotificationTapIfPresent()` on tap |

When the user taps the notification, FCM delivers the `data` map keys as Long/String extras
on the `MainActivity` launch intent. `handleFcmNotificationTapIfPresent()` reads
`notification_id` from those extras and enqueues DELIVERED + CLICKED via WorkManager.
`onNewIntent()` covers the case where the activity is already running and brought to front.
In all tap cases `AppRefreshState.triggerRefresh()` is also called so the UI reloads.

### Token Maintenance

- If messaging.Send() returns an Unregistered error, immediately DELETE that token from user_devices.

### Manage fcm_token lifecycle

- When receive messaging.IsUnregistered(err) ie Http 404 from FCM, stamp the fcm_tokens.status='OBSOLETE'. 
- notifications.delivered_at, clicked_at, app_opened_at, read_at in postgresql database should only take the first value entered, ie. they should record the first timestamp of the event; no overrides. 