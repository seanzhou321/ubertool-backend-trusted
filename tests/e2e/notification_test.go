package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "localhost:50051")
	defer client.Close()

	notificationClient := pb.NewNotificationServiceClient(client.Conn())

	t.Run("GetNotifications", func(t *testing.T) {
		// Setup: Create user, org, and notifications
		userID := db.CreateTestUser("e2e-test-notif-user@test.com", "Notif User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)

		// Create test notifications
		_, err := db.Exec(`
			INSERT INTO notifications (user_id, org_id, title, message, is_read, attributes)
			VALUES 
				($1, $2, 'Test Notification 1', 'This is a test notification', false, '{"type": "rental_request"}'::jsonb),
				($1, $2, 'Test Notification 2', 'Another test notification', false, '{"type": "admin_alert"}'::jsonb),
				($1, $2, 'Old Notification', 'This one is read', true, '{}'::jsonb)
		`, userID, orgID)
		require.NoError(t, err)

		// Test: Get notifications
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetNotificationsRequest{
			Limit:  10,
			Offset: 0,
		}

		resp, err := notificationClient.GetNotifications(ctx, req)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(resp.Notifications), 3)
		assert.GreaterOrEqual(t, resp.TotalCount, int32(3))

		// Verify: Notifications contain expected data
		unreadCount := 0
		for _, notif := range resp.Notifications {
			if !notif.Read {
				unreadCount++
			}
			assert.NotEmpty(t, notif.Title)
			assert.NotEmpty(t, notif.Message)
		}
		assert.GreaterOrEqual(t, unreadCount, 2, "Should have at least 2 unread notifications")
	})

	t.Run("MarkNotificationRead", func(t *testing.T) {
		// Setup: Create user, org, and notification
		userID := db.CreateTestUser("e2e-test-notif-mark@test.com", "Mark User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)

		var notifID int32
		err := db.QueryRow(`
			INSERT INTO notifications (user_id, org_id, title, message, is_read)
			VALUES ($1, $2, 'Unread Notification', 'Please read this', false)
			RETURNING id
		`, userID, orgID).Scan(&notifID)
		require.NoError(t, err)

		// Test: Mark notification as read
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.MarkNotificationReadRequest{
			NotificationId: notifID,
		}

		resp, err := notificationClient.MarkNotificationRead(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: Notification is marked as read in database
		var isRead bool
		err = db.QueryRow("SELECT is_read FROM notifications WHERE id = $1", notifID).Scan(&isRead)
		assert.NoError(t, err)
		assert.True(t, isRead)
	})

	t.Run("GetNotifications with Pagination", func(t *testing.T) {
		// Setup: Create user and many notifications
		userID := db.CreateTestUser("e2e-test-notif-page@test.com", "Page User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)

		// Create 15 notifications
		for i := 0; i < 15; i++ {
			_, err := db.Exec(`
				INSERT INTO notifications (user_id, org_id, title, message, is_read)
				VALUES ($1, $2, $3, $4, false)
			`, userID, orgID, "Notification "+string(rune(i)), "Message "+string(rune(i)))
			require.NoError(t, err)
		}

		// Test: Get first page (10 items)
		ctx1, cancel1 := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel1()

		req1 := &pb.GetNotificationsRequest{
			Limit:  10,
			Offset: 0,
		}

		resp1, err := notificationClient.GetNotifications(ctx1, req1)
		require.NoError(t, err)
		assert.Equal(t, 10, len(resp1.Notifications))
		assert.GreaterOrEqual(t, resp1.TotalCount, int32(15))

		// Test: Get second page (5 items)
		ctx2, cancel2 := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel2()

		req2 := &pb.GetNotificationsRequest{
			Limit:  10,
			Offset: 10,
		}

		resp2, err := notificationClient.GetNotifications(ctx2, req2)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp2.Notifications), 5)
	})

	t.Run("MarkNotificationRead - Wrong User", func(t *testing.T) {
		// Setup: Create two users and a notification for user1
		user1ID := db.CreateTestUser("e2e-test-notif-user1@test.com", "User 1")
		user2ID := db.CreateTestUser("e2e-test-notif-user2@test.com", "User 2")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(user1ID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(user2ID, orgID, "MEMBER", "ACTIVE", 0)

		var notifID int32
		err := db.QueryRow(`
			INSERT INTO notifications (user_id, org_id, title, message, is_read)
			VALUES ($1, $2, 'User 1 Notification', 'For user 1 only', false)
			RETURNING id
		`, user1ID, orgID).Scan(&notifID)
		require.NoError(t, err)

		// Test: User 2 tries to mark User 1's notification as read (should fail)
		ctx, cancel := ContextWithUserIDAndTimeout(user2ID, 5*time.Second)
		defer cancel()

		req := &pb.MarkNotificationReadRequest{
			NotificationId: notifID,
		}

		_, err = notificationClient.MarkNotificationRead(ctx, req)
		// Should fail - user 2 cannot mark user 1's notification
		assert.Error(t, err)
	})

	t.Run("Notifications Created by Rental Workflow", func(t *testing.T) {
		// This test verifies that notifications are created through the rental workflow
		// Setup
		orgID := db.CreateTestOrg("")
		ownerID := db.CreateTestUser("e2e-test-notif-owner@test.com", "Owner")
		renterID := db.CreateTestUser("e2e-test-notif-renter@test.com", "Renter")

		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(renterID, orgID, "MEMBER", "ACTIVE", 5000)

		toolID := db.CreateTestTool(ownerID, "Notif Test Tool", 1000)

		// Create rental request (should create notification for owner)
		rentalClient := pb.NewRentalServiceClient(client.Conn())

		ctx1, cancel1 := ContextWithUserIDAndTimeout(renterID, 5*time.Second)
		defer cancel1()

		startDate := time.Now().Add(24 * time.Hour)
		endDate := startDate.Add(24 * time.Hour)

		createReq := &pb.CreateRentalRequestRequest{
			ToolId:         toolID,
			StartDate:      startDate.Format("2006-01-02"),
			EndDate:        endDate.Format("2006-01-02"),
			OrganizationId: orgID,
		}

		_, err := rentalClient.CreateRentalRequest(ctx1, createReq)
		require.NoError(t, err)

		// Verify: Owner received notification
		ctx2, cancel2 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
		defer cancel2()

		notifReq := &pb.GetNotificationsRequest{
			Limit:  10,
			Offset: 0,
		}

		notifResp, err := notificationClient.GetNotifications(ctx2, notifReq)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(notifResp.Notifications), 1, "Owner should have received rental request notification")

		// Verify: Notification contains rental information
		foundRentalNotif := false
		for _, notif := range notifResp.Notifications {
			if notif.Attributes != nil {
				// Check if this is a rental-related notification
				foundRentalNotif = true
				break
			}
		}
		assert.True(t, foundRentalNotif, "Should have rental-related notification")
	})
}
