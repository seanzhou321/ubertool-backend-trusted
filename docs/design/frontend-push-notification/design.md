# FCM Push Notification Design

## ⚠️ Critical Backend Requirement: Data-Only Messages

The backend **MUST** send **data-only** FCM messages — the `notification` field **MUST be omitted**.

### Why this matters

FCM has two message types:

| Type | `notification` field | `onMessageReceived` called when app is killed? | Sound/UI control |
|---|---|---|---|
| **Notification message** | Present | ❌ No — OS renders directly | OS-controlled, ignores app code |
| **Data-only message** | Absent | ✅ Yes — always | App-controlled |
| **Notification + data** | Both present | ❌ No — OS renders directly | OS-controlled |

When the backend includes a `notification` field and the app is **swiped away (killed)**:
- `onMessageReceived` is **never called**
- The Android OS renders the notification directly from the `notification` payload
- The app has **no control** over sound, appearance, or `notification_id` tracking
- Result: silent dot on launcher icon, no heads-up banner, no sound

When the backend sends **data-only** (no `notification` field):
- `onMessageReceived` is **always called** regardless of app state (foreground / background / killed)
- The app builds and posts the notification itself with full control over sound, style, and tracking
- Sound, vibration, heads-up banner, and `BigTextStyle` all work correctly

---

## Backend FCM Payload Format (REQUIRED)

```json
{
  "message": {
    "token": "<device_fcm_token>",
    "android": {
      "priority": "high"
    },
    "data": {
      "notification_id": "12345",
      "channel_id":      "rental_request_messages",
      "title":           "Rental Request Approved",
      "body":            "Your request for the DeWalt drill has been approved."
    }
  }
}
```

**Do NOT include a `notification` block.** All display fields go in `data`.

### Data field reference

| Field | Type | Required | Description |
|---|---|---|---|
| `notification_id` | string (int64) | ✅ | Postgres `bigserial` from the `notifications` table. Used to report DELIVERED/CLICKED lifecycle events via gRPC. |
| `channel_id` | string | ✅ | One of the channel IDs below. Determines sound/priority. Defaults to `app_messages` if omitted or unrecognised. |
| `title` | string | ✅ | Notification title shown to the user. |
| `body` | string | ✅ | Notification body text shown to the user. |

### Notification channel IDs

| `channel_id` value | Priority | Vibration | Use case |
|---|---|---|---|
| `rental_request_messages` | HIGH | ✅ | Rental request created / changed / approved / rejected |
| `billsplitting_messages` | DEFAULT | ❌ | End-of-month bill-split events |
| `dispute_messages` | DEFAULT | ❌ | Rental dispute notifications |
| `admin_messages` | DEFAULT | ❌ | Administrative events |
| `app_messages` | DEFAULT | ❌ | General app events (default) |

---

## 1. Token Lifecycle Management

The app ensures the backend always has the current FCM device token.

### `onNewToken(token: String)`
Called by the FCM SDK when the token is issued or rotated.
- Enqueues `SyncDeviceTokenWorker` (WorkManager, requires network) to call gRPC `SyncDeviceToken`.
- Stores the new token in DataStore for comparison on next launch.

### App Launch Sync (`MainActivity.onCreate`)
- Retrieves the current FCM token via `FirebaseMessaging.getInstance().token`.
- Calls `SyncDeviceToken` if the stored token differs from the current one.

### Device Identifier
`SyncDeviceToken` uses `FirebaseInstallations.getId()` as the `device_id` — stable per app
installation, resets on reinstall. This is also used at logout to mark the token `OBSOLETE`
so the backend stops delivering to a logged-out device.

---

## 2. Message Lifecycle Tracking

### Step 1 — Received (`DELIVERED`)
`onMessageReceived` is called for every data-only message regardless of app state.
- Extracts `notification_id` from `remoteMessage.data`.
- Enqueues `ReportMessageEventWorker` (WorkManager) → gRPC `ReportMessageEvent(id, "DELIVERED")`.
- If the app is in the **foreground**, also calls `AppRefreshState.triggerRefresh()` so the UI
  reloads immediately.
- Calls `showNotification()` to post the visible heads-up notification.

### Step 2 — Clicked (`CLICKED`)
Tapping the notification launches `MainActivity` with the `notification_id` as a Long intent extra.
- `handleFcmNotificationTapIfPresent()` in `onCreate` / `onNewIntent` reads the extra.
- Enqueues `ReportMessageEventWorker` → gRPC `ReportMessageEvent(id, "CLICKED")`.
- Calls `AppRefreshState.triggerRefresh()` so the UI reloads to reflect the notified state.

> **Note:** The `PendingIntent` uses `getActivity` pointing directly at `MainActivity`,
> not a `BroadcastReceiver`. `BroadcastReceiver` intents are only appropriate for
> notification *action buttons* (`addAction`), not the primary tap target.

### Step 3 — Missed messages (`onDeletedMessages`)
Called when FCM drops messages because the device was offline too long.
- **Foreground:** calls `AppRefreshState.triggerRefresh()` silently.
- **Background / killed:** posts a "You have new messages" notification. Tapping it
  launches `MainActivity` with `EXTRA_REFRESH_ON_START = true`, triggering a full reload.

---

## 3. Reliability with WorkManager

All gRPC lifecycle reporting (`DELIVERED`, `CLICKED`) is wrapped in a WorkManager
`OneTimeWorkRequest` with a `NetworkType.CONNECTED` constraint and exponential back-off.
This ensures events are delivered even if the device is offline when the notification
arrives or is tapped.

---

## 4. Notification Channels

Channels are registered in `UberToolApplication.onCreate` via
`UberToolFirebaseMessagingService.createNotificationChannel(context)`.

Channel properties (sound, vibration, importance) are **immutable after first creation**.
The registration code detects stale channels (missing sound or wrong importance) and
deletes them before re-creating, so updates take effect on the next app launch.

All channels use the default notification ringtone (`RingtoneManager.TYPE_NOTIFICATION`)
and have notification lights enabled.
