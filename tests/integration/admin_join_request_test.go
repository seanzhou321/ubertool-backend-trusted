package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminService_ListJoinRequests_UsedOnField(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	// Initialize repositories
	joinReqRepo := postgres.NewJoinRequestRepository(db)
	userRepo := postgres.NewUserRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	inviteRepo := postgres.NewInvitationRepository(db)

	// Create admin service (emailSvc can be nil for this test)
	adminSvc := service.NewAdminService(joinReqRepo, userRepo, ledgerRepo, orgRepo, inviteRepo, nil)

	ctx := context.Background()

	t.Run("Verify UsedOn Field is Populated Correctly", func(t *testing.T) {
		// Clean up test data
		db.Exec("DELETE FROM join_requests WHERE email LIKE 'test-join-req-%'")
		db.Exec("DELETE FROM invitations WHERE email LIKE 'test-join-req-%'")
		db.Exec("DELETE FROM users_orgs WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'test-join-req-%')")
		db.Exec("DELETE FROM users WHERE email LIKE 'test-join-req-%'")
		db.Exec("DELETE FROM orgs WHERE name LIKE 'Test-Join-Req-Org-%'")

		// Create test organization
		var orgID int32
		err := db.QueryRow(`INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address) 
			VALUES ($1, 'San Jose', 'admin@test.com', '555-0000', '123 Test St') RETURNING id`,
			fmt.Sprintf("Test-Join-Req-Org-%d", time.Now().UnixNano())).Scan(&orgID)
		require.NoError(t, err)
		t.Logf("Created test org_id=%d", orgID)

		// Create test users
		email1 := fmt.Sprintf("test-join-req-user1-%d@test.com", time.Now().UnixNano())
		email2 := fmt.Sprintf("test-join-req-user2-%d@test.com", time.Now().UnixNano())

		var userID1, userID2 int32
		err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name) 
			VALUES ($1, '555-1111', 'hash', 'Test User 1') RETURNING id`, email1).Scan(&userID1)
		require.NoError(t, err)

		err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name) 
			VALUES ($1, '555-2222', 'hash', 'Test User 2') RETURNING id`, email2).Scan(&userID2)
		require.NoError(t, err)
		t.Logf("Created test users: user_id1=%d, user_id2=%d", userID1, userID2)

		// Create join requests
		var joinReqID1, joinReqID2 int32
		err = db.QueryRow(`INSERT INTO join_requests (org_id, user_id, name, email, note, status) 
			VALUES ($1, $2, 'Test User 1', $3, 'Please add me', 'PENDING') RETURNING id`,
			orgID, userID1, email1).Scan(&joinReqID1)
		require.NoError(t, err)

		err = db.QueryRow(`INSERT INTO join_requests (org_id, user_id, name, email, note, status) 
			VALUES ($1, $2, 'Test User 2', $3, 'Add me too', 'PENDING') RETURNING id`,
			orgID, userID2, email2).Scan(&joinReqID2)
		require.NoError(t, err)
		t.Logf("Created join_requests: id1=%d, id2=%d", joinReqID1, joinReqID2)

		// Create invitations with used_on dates
		usedDate1 := time.Now().AddDate(0, 0, -5) // 5 days ago
		usedDate2 := time.Now().AddDate(0, 0, -1) // 1 day ago (today for recent case)

		var invID1, invID2 int32
		err = db.QueryRow(`INSERT INTO invitations (invitation_code, org_id, email, created_by, expires_on, used_on, used_by_user_id) 
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			fmt.Sprintf("INV-%d", time.Now().UnixNano()), orgID, email1, userID1,
			time.Now().AddDate(0, 0, 30), usedDate1, userID1).Scan(&invID1)
		require.NoError(t, err)

		err = db.QueryRow(`INSERT INTO invitations (invitation_code, org_id, email, created_by, expires_on, used_on, used_by_user_id) 
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			fmt.Sprintf("INV-%d", time.Now().UnixNano()+1), orgID, email2, userID1,
			time.Now().AddDate(0, 0, 30), usedDate2, userID2).Scan(&invID2)
		require.NoError(t, err)
		t.Logf("Created invitations: id1=%d (used %s), id2=%d (used %s)",
			invID1, usedDate1.Format("2006-01-02"), invID2, usedDate2.Format("2006-01-02"))

		// Create an admin caller — ListJoinRequests now requires ADMIN/SUPER_ADMIN
		// membership in orgID (see internal/service/admin.go verifyAdminRights).
		adminEmail := fmt.Sprintf("test-join-req-admin-%d@test.com", time.Now().UnixNano())
		var adminUserID int32
		err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name)
			VALUES ($1, '555-3333', 'hash', 'Test Admin') RETURNING id`, adminEmail).Scan(&adminUserID)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO users_orgs (user_id, org_id, role, status) VALUES ($1, $2, 'ADMIN', 'ACTIVE')`,
			adminUserID, orgID)
		require.NoError(t, err)

		// Now test the service method
		reqs, err := adminSvc.ListJoinRequests(ctx, adminUserID, orgID)
		require.NoError(t, err)
		require.Len(t, reqs, 2, "Should return 2 join requests")

		// Verify both have used_on populated
		foundReq1 := false
		foundReq2 := false

		for _, req := range reqs {
			if req.ID == joinReqID1 {
				foundReq1 = true
				require.NotNil(t, req.UsedOn, "Join request 1 should have used_on populated")
				assert.Equal(t, usedDate1.Format("2006-01-02"), *req.UsedOn,
					"Join request 1 used_on should match invitation used_on")
				t.Logf("✓ Join request 1: used_on=%s (correct)", *req.UsedOn)
			}
			if req.ID == joinReqID2 {
				foundReq2 = true
				require.NotNil(t, req.UsedOn, "Join request 2 should have used_on populated")
				assert.Equal(t, usedDate2.Format("2006-01-02"), *req.UsedOn,
					"Join request 2 used_on should match invitation used_on")
				t.Logf("✓ Join request 2: used_on=%s (correct)", *req.UsedOn)
			}
		}

		assert.True(t, foundReq1, "Should find join request 1")
		assert.True(t, foundReq2, "Should find join request 2")
	})
}

// TestAdminService_ListJoinRequests_FiltersToPendingOnly closes the gap flagged in
// sbr/rtm/003-organizations-administration.rtm.md FR-008: the repository query
// (internal/repository/postgres/join_request.go ListByOrg) filters to `status = 'PENDING'`,
// but no prior test ever seeded a non-PENDING row to prove that filter actually excludes it —
// both this file's other test and the e2e test only ever seeded PENDING rows.
func TestAdminService_ListJoinRequests_FiltersToPendingOnly(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	joinReqRepo := postgres.NewJoinRequestRepository(db)
	userRepo := postgres.NewUserRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	inviteRepo := postgres.NewInvitationRepository(db)
	adminSvc := service.NewAdminService(joinReqRepo, userRepo, ledgerRepo, orgRepo, inviteRepo, nil)

	ctx := context.Background()

	var orgID int32
	err := db.QueryRow(`INSERT INTO orgs (name, metro, admin_email, admin_phone_number, address)
		VALUES ($1, 'San Jose', 'admin@test.com', '555-0000', '123 Test St') RETURNING id`,
		fmt.Sprintf("Test-Join-Req-Status-Org-%d", time.Now().UnixNano())).Scan(&orgID)
	require.NoError(t, err)

	adminEmail := fmt.Sprintf("test-join-req-status-admin-%d@test.com", time.Now().UnixNano())
	var adminUserID int32
	err = db.QueryRow(`INSERT INTO users (email, phone_number, password_hash, name)
		VALUES ($1, '555-3333', 'hash', 'Test Admin') RETURNING id`, adminEmail).Scan(&adminUserID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users_orgs (user_id, org_id, role, status) VALUES ($1, $2, 'ADMIN', 'ACTIVE')`,
		adminUserID, orgID)
	require.NoError(t, err)

	var pendingID, approvedID, rejectedID int32
	err = db.QueryRow(`INSERT INTO join_requests (org_id, user_id, name, email, note, status)
		VALUES ($1, NULL, 'Pending Applicant', $2, 'note', 'PENDING') RETURNING id`,
		orgID, fmt.Sprintf("test-join-req-status-pending-%d@test.com", time.Now().UnixNano())).Scan(&pendingID)
	require.NoError(t, err)

	err = db.QueryRow(`INSERT INTO join_requests (org_id, user_id, name, email, note, status)
		VALUES ($1, NULL, 'Invited Applicant', $2, 'note', 'INVITED') RETURNING id`,
		orgID, fmt.Sprintf("test-join-req-status-invited-%d@test.com", time.Now().UnixNano())).Scan(&approvedID)
	require.NoError(t, err)

	err = db.QueryRow(`INSERT INTO join_requests (org_id, user_id, name, email, note, status, reason)
		VALUES ($1, NULL, 'Rejected Applicant', $2, 'note', 'REJECTED', 'not a fit') RETURNING id`,
		orgID, fmt.Sprintf("test-join-req-status-rejected-%d@test.com", time.Now().UnixNano())).Scan(&rejectedID)
	require.NoError(t, err)

	reqs, err := adminSvc.ListJoinRequests(ctx, adminUserID, orgID)
	require.NoError(t, err)

	var gotIDs []int32
	for _, r := range reqs {
		gotIDs = append(gotIDs, r.ID)
	}
	assert.Contains(t, gotIDs, pendingID, "the PENDING join request must be returned")
	assert.NotContains(t, gotIDs, approvedID, "the INVITED join request must be filtered out")
	assert.NotContains(t, gotIDs, rejectedID, "the REJECTED join request must be filtered out")
}
