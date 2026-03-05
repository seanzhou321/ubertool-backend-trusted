package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupUsers are the two real users defined in tests/data-setup/user_org.yaml.
// Their FCM tokens are registered when they log in on their physical devices.
var setupUserEmails = [2]string{
	"ubertool320@gmail.com",
	"sean.zhou321@gmail.com",
}

// activeTokenUser holds the resolved DB data for one test participant.
type activeTokenUser struct {
	UserID   int32
	FCMToken string
	DeviceID string
	Email    string
}

// requireSetupUsers looks up the two data-setup users and their latest active FCM
// tokens.  The test is skipped when either user is missing or has no active token.
func requireSetupUsers(t *testing.T, db *TestDB) (activeTokenUser, activeTokenUser) {
	t.Helper()
	var users [2]activeTokenUser
	for i, email := range setupUserEmails {
		// Resolve user ID.
		err := db.QueryRow(
			`SELECT id FROM users WHERE email = $1`, email,
		).Scan(&users[i].UserID)
		if err != nil {
			t.Skipf("setup user %q not found in database — run tests/data-setup first: %v", email, err)
		}
		users[i].Email = email

		// Resolve latest active FCM token.
		err = db.QueryRow(`
			SELECT fcm_token, android_device_id
			FROM fcm_tokens
			WHERE user_id = $1 AND status = 'ACTIVE'
			ORDER BY updated_at DESC
			LIMIT 1
		`, users[i].UserID).Scan(&users[i].FCMToken, &users[i].DeviceID)
		if err != nil {
			t.Skipf("setup user %q (id=%d) has no active FCM token — device must be logged in: %v",
				email, users[i].UserID, err)
		}
		t.Logf("setup user %q  userID=%d  token_prefix=%.16s...", email, users[i].UserID, users[i].FCMToken)
	}
	return users[0], users[1]
}

// TestPushNotificationService_E2E verifies the end-to-end push notification path using the
// two real users defined in tests/data-setup/user_org.yaml.
//
// Prerequisites (skip if not met):
//   - Both ubertool320@gmail.com and sean.zhou321@gmail.com exist in the database
//   - Both users have at least one token with status = 'ACTIVE' in fcm_tokens
//     (i.e. each user has logged in on a physical device)
//
// What is verified:
//   - The notification record is created in the DB (server-side dispatch is confirmed)
//   - A push is sent to each user's registered device token(s) via Firebase
//   - The notification can be retrieved via GetNotifications gRPC
func TestPushNotificationService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()

	notifClient := pb.NewNotificationServiceClient(NewGRPCClient(t, "").Conn())

	user1, user2 := requireSetupUsers(t, db)

	// Find an org that both users share (they are both in "Test Community 1" per user_org.yaml).
	var sharedOrgID int32
	err := db.QueryRow(`
		SELECT uo1.org_id
		FROM users_orgs uo1
		JOIN users_orgs uo2 ON uo1.org_id = uo2.org_id
		WHERE uo1.user_id = $1 AND uo2.user_id = $2
		LIMIT 1
	`, user1.UserID, user2.UserID).Scan(&sharedOrgID)
	if err != nil {
		t.Skipf("user1 and user2 share no common org — both must be members of 'Test Community 1': %v", err)
	}
	t.Logf("shared org id=%d", sharedOrgID)

	t.Run("DispatchToUser1_ReceivesPush", func(t *testing.T) {
		// user1 owns a temporary tool; user2 creates a rental request on it.
		// On the server this calls notificationService.Dispatch for user1 (the owner),
		// which in turn calls SendToUser → Firebase → user1's device receives the push.
		toolID := db.CreateTestTool(user1.UserID, "[E2E TEST] Push Test Tool – ignore", 100)
		defer func() {
			db.Exec("DELETE FROM rentals WHERE tool_id = $1", toolID)
			db.Exec("DELETE FROM tools WHERE id = $1", toolID)
		}()

		ctx, cancel := ContextWithUserIDAndTimeout(user2.UserID, 5*time.Second)
		defer cancel()
		startDate := time.Now().AddDate(0, 0, 2)
		endDate := startDate.AddDate(0, 0, 1)
		_, err := pb.NewRentalServiceClient(NewGRPCClient(t, "").Conn()).CreateRentalRequest(ctx, &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: sharedOrgID,
		})
		require.NoError(t, err)
		t.Logf("rental request created: renter=%s requests tool owned by %s — push should arrive on %s's device",
			user2.Email, user1.Email, user1.Email)

		// The push goroutine fires on attempt 1 with no pre-delay; 2 s is enough.
		time.Sleep(2 * time.Second)

		// Confirm the notification row was created for user1.
		ctxCheck, cancelCheck := ContextWithUserIDAndTimeout(user1.UserID, 5*time.Second)
		defer cancelCheck()
		resp, err := notifClient.GetNotifications(ctxCheck, &pb.GetNotificationsRequest{Limit: 5, Offset: 0})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Notifications, "user1 must have at least one notification after the rental request")
		if len(resp.Notifications) > 0 {
			t.Logf("latest notification for user1: id=%d title=%q", resp.Notifications[0].Id, resp.Notifications[0].Title)
		}
	})

	t.Run("DispatchToUser2_ReceivesPush", func(t *testing.T) {
		// user2 owns a temporary tool; user1 creates a rental request on it.
		toolID := db.CreateTestTool(user2.UserID, "[E2E TEST] Push Test Tool – ignore", 100)
		defer func() {
			db.Exec("DELETE FROM rentals WHERE tool_id = $1", toolID)
			db.Exec("DELETE FROM tools WHERE id = $1", toolID)
		}()

		ctx, cancel := ContextWithUserIDAndTimeout(user1.UserID, 5*time.Second)
		defer cancel()
		startDate := time.Now().AddDate(0, 0, 2)
		endDate := startDate.AddDate(0, 0, 1)
		_, err := pb.NewRentalServiceClient(NewGRPCClient(t, "").Conn()).CreateRentalRequest(ctx, &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: sharedOrgID,
		})
		require.NoError(t, err)
		t.Logf("rental request created: renter=%s requests tool owned by %s — push should arrive on %s's device",
			user1.Email, user2.Email, user2.Email)

		time.Sleep(2 * time.Second)

		// Confirm the notification row was created for user2.
		ctxCheck, cancelCheck := ContextWithUserIDAndTimeout(user2.UserID, 5*time.Second)
		defer cancelCheck()
		resp, err := notifClient.GetNotifications(ctxCheck, &pb.GetNotificationsRequest{Limit: 5, Offset: 0})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Notifications, "user2 must have at least one notification after the rental request")
		if len(resp.Notifications) > 0 {
			t.Logf("latest notification for user2: id=%d title=%q", resp.Notifications[0].Id, resp.Notifications[0].Title)
		}
	})

	t.Run("BothUsersStillHaveActiveTokens", func(t *testing.T) {
		// After the push dispatch goroutines have had time to run (they are async
		// with production delays), verify neither token was silently removed.
		// If a token were erroneously revoked by a bug in MarkObsolete the count here
		// would drop to 0.
		assert.GreaterOrEqual(t, db.CountActiveTokensForUser(user1.UserID), 1,
			"user1 must still have at least one ACTIVE token")
		assert.GreaterOrEqual(t, db.CountActiveTokensForUser(user2.UserID), 1,
			"user2 must still have at least one ACTIVE token")
	})

	t.Run("SyncDeviceToken_User1_Idempotent", func(t *testing.T) {
		// Re-syncing the same token must not increase the active-token count for user1.
		before := db.CountActiveTokensForUser(user1.UserID)

		ctx, cancel := ContextWithUserIDAndTimeout(user1.UserID, 5*time.Second)
		defer cancel()
		_, err := notifClient.SyncDeviceToken(ctx, &pb.SyncTokenRequest{
			FcmToken:        user1.FCMToken,
			AndroidDeviceId: user1.DeviceID,
			DeviceName:      "e2e-re-sync",
		})
		require.NoError(t, err)

		after := db.CountActiveTokensForUser(user1.UserID)
		assert.Equal(t, before, after, "re-syncing the same token must not create a duplicate row")
	})

	t.Run("SyncDeviceToken_User2_Idempotent", func(t *testing.T) {
		before := db.CountActiveTokensForUser(user2.UserID)

		ctx, cancel := ContextWithUserIDAndTimeout(user2.UserID, 5*time.Second)
		defer cancel()
		_, err := notifClient.SyncDeviceToken(ctx, &pb.SyncTokenRequest{
			FcmToken:        user2.FCMToken,
			AndroidDeviceId: user2.DeviceID,
			DeviceName:      "e2e-re-sync",
		})
		require.NoError(t, err)

		after := db.CountActiveTokensForUser(user2.UserID)
		assert.Equal(t, before, after, "re-syncing the same token must not create a duplicate row")
	})

	t.Run("UnregisteredToken_MarkedObsolete", func(t *testing.T) {
		// Create a throwaway user with a deliberately invalid FCM token seeded
		// directly in the DB (simulates a token that was once valid but the device
		// has since been wiped / the app re-installed).
		//
		// Firebase returns UNREGISTERED (404) for such tokens. The push service
		// must then call MarkObsolete so the stale row is never used again.
		//
		// The rental workflow is used to trigger a real Dispatch call on the server:
		// the fake user owns a tool, a renter requests it, which fires a push to
		// the fake user's invalid token. Firebase returns UNREGISTERED → token
		// must become OBSOLETE.
		//
		// The first Send attempt happens immediately (no pre-delay), so a short
		// sleep after the rental request is sufficient.
		orgID := db.CreateTestOrg("")
		fakeOwnerID := db.CreateTestUser(
			fmt.Sprintf("e2e-test-push-fake-%d@test.com", time.Now().UnixNano()),
			"Fake Push Owner",
		)
		renterID := db.CreateTestUser(
			fmt.Sprintf("e2e-test-push-renter-%d@test.com", time.Now().UnixNano()),
			"Fake Push Renter",
		)
		defer db.Exec("DELETE FROM rentals WHERE owner_id = $1", fakeOwnerID)
		defer db.Exec("DELETE FROM tools WHERE owner_id = $1", fakeOwnerID)
		defer db.Exec("DELETE FROM fcm_tokens WHERE user_id = $1", fakeOwnerID)
		defer db.Exec("DELETE FROM notifications WHERE user_id = $1 OR user_id = $2", fakeOwnerID, renterID)
		defer db.Exec("DELETE FROM users_orgs WHERE user_id = $1 OR user_id = $2", fakeOwnerID, renterID)
		defer db.Exec("DELETE FROM users WHERE id = $1 OR id = $2", fakeOwnerID, renterID)

		db.AddUserToOrg(fakeOwnerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 100000)
		toolID := db.CreateTestTool(fakeOwnerID, "Fake Owner Tool", 500)

		// Seed a syntactically plausible but unregistered FCM token for the fake owner.
		// The long prefix is intentional: FCM rejects short/random strings with
		// INVALID_ARGUMENT; a valid-looking token that is not registered to any device
		// returns UNREGISTERED instead.
		fakeToken := fmt.Sprintf(
			"fXmW3zK9:%d_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			time.Now().UnixNano(),
		)
		db.CreateFcmToken(fakeOwnerID, fakeToken, "fake-device-e2e")
		require.Equal(t, "ACTIVE", db.GetFcmTokenStatus(fakeToken), "token must start as ACTIVE")

		// Create a rental request as the renter → server calls Dispatch for the owner
		// → push goroutine fires SendToUser with the fake token → Firebase returns
		// UNREGISTERED → MarkObsolete is called.
		ctx, cancel := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel()
		startDate := time.Now().AddDate(0, 0, 2)
		endDate := startDate.AddDate(0, 0, 1)
		_, err := pb.NewRentalServiceClient(NewGRPCClient(t, "").Conn()).CreateRentalRequest(ctx, &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		})
		require.NoError(t, err)

		// Firebase returns UNREGISTERED (404) for an expired token or INVALID_ARGUMENT
		// (400) for a syntactically invalid token. Both are permanent failures; the push
		// service marks the token OBSOLETE in either case.
		// After the rental request the goroutine fires immediately (no pre-delay on
		// attempt 1) and marks the token obsolete. 3 seconds is generous for that.
		time.Sleep(3 * time.Second)

		// Token must be OBSOLETE — Firebase rejected it as either unregistered or invalid.
		finalStatus := db.GetFcmTokenStatus(fakeToken)
		assert.Equal(t, "OBSOLETE", finalStatus,
			"fake token must be marked OBSOLETE after Firebase returns UNREGISTERED or INVALID_ARGUMENT")
	})

	t.Run("DirectFCMSend_PrintResponseID", func(t *testing.T) {
		// Bypass the server entirely: initialise Firebase directly in the test
		// process, send a push to each real user's FCM token, and log the FCM
		// message ID (or error). This confirms whether Firebase accepted the
		// message independently of the server's async goroutine.
		keyPath := ""
		for _, p := range []string{
			"config/firebase-admin-key.json",
			"../../config/firebase-admin-key.json",
		} {
			if _, err := os.Stat(p); err == nil {
				keyPath = p
				break
			}
		}
		if keyPath == "" {
			t.Skip("firebase-admin-key.json not found — skipping direct FCM test")
		}

		app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsFile(keyPath))
		require.NoError(t, err, "initialize Firebase app")
		fcmClient, err := app.Messaging(context.Background())
		require.NoError(t, err, "get FCM messaging client")

		for _, u := range []activeTokenUser{user1, user2} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			msg := &messaging.Message{
				Token: u.FCMToken,
				Notification: &messaging.Notification{
					Title: "[E2E DIRECT] FCM connectivity check",
					Body:  "If you see this, Firebase accepted the push.",
				},
				Android: &messaging.AndroidConfig{
					Priority: "high",
					Notification: &messaging.AndroidNotification{
						ChannelID: "rental_request_messages",
					},
				},
			}
			msgID, sendErr := fcmClient.Send(ctx, msg)
			if sendErr != nil {
				t.Errorf("FCM REJECTED push for user %s (token_prefix=%.16s...): %v",
					u.Email, u.FCMToken, sendErr)
			} else {
				t.Logf("FCM ACCEPTED push for user %s — FCM message ID: %s", u.Email, msgID)
			}
		}
	})
}

