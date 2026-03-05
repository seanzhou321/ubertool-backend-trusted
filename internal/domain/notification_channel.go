package domain

// NotificationChannel identifies the Android notification channel that should
// render a push notification on the client. The backend embeds this value as
// the "channel_id" key in the FCM data payload.
//
// Channel priority (FCM Android priority):
//   - HIGH  – rental_request_messages, billsplitting_messages, dispute_messages
//   - NORMAL – admin_messages, app_messages
type NotificationChannel string

const (
	// ChannelRentalRequest covers all rental lifecycle events (request, approve,
	// reject, cancel, confirm, pickup, date changes, extensions).
	ChannelRentalRequest NotificationChannel = "rental_request_messages"

	// ChannelBillSplitting covers bill-split acknowledgement events.
	ChannelBillSplitting NotificationChannel = "billsplitting_messages"

	// ChannelDispute covers admin dispute-resolution events.
	ChannelDispute NotificationChannel = "dispute_messages"

	// ChannelAdmin covers administrative events such as join requests.
	ChannelAdmin NotificationChannel = "admin_messages"

	// ChannelApp covers general app events (fallback / default).
	ChannelApp NotificationChannel = "app_messages"
)
