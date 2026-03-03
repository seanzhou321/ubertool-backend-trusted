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

1. Generate ID: Create a new UUID internal_msg_id.
2. Payload: Construct an FCM message using the Firebase Admin Go SDK. Include internal_msg_id in the Data Payload.
3. DB Entry: Insert a record into notification_logs with status SENT.
4. Execute: Call messaging.Send(ctx, message).
5. Use a goroutine or a worker queue to send the message without blocking the other processes.
6. embed each message the notifications.id value of the notification record to track the message. When the message comes back from the phone app, it is the ReportEventRequest.notification_id field in the notification_service.proto.

### Token Maintenance

- If messaging.Send() returns an Unregistered error, immediately DELETE that token from user_devices.

### Manage fcm_token lifecycle

- When receive messaging.IsUnregistered(err) ie Http 404 from FCM, stamp the fcm_tokens.status='OBSOLETE'. 
- notifications.delivered_at, clicked_at, app_opened_at, read_at in postgresql database should only take the first value entered, ie. they should record the first timestamp of the event; no overrides. 