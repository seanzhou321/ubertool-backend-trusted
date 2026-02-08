package integration

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/repository/postgres"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationService_MemberCount(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	// Initialize repositories
	orgRepo := postgres.NewOrganizationRepository(db)
	userRepo := postgres.NewUserRepository(db)
	inviteRepo := postgres.NewInvitationRepository(db)
	notifRepo := postgres.NewNotificationRepository(db)

	// Create organization service
	orgSvc := service.NewOrganizationService(orgRepo, userRepo, inviteRepo, notifRepo)

	ctx := context.Background()

	t.Run("GetOrganization returns correct member_count", func(t *testing.T) {
		// Use existing org_id=1 from the database
		org, err := orgSvc.GetOrganization(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, org)

		t.Logf("Organization: id=%d, name=%s, member_count=%d", org.ID, org.Name, org.MemberCount)
		
		// Verify member_count is populated (should be > 0 if org has members)
		assert.GreaterOrEqual(t, org.MemberCount, int32(0), "Member count should be >= 0")
		
		// Verify against actual count in database
		var actualCount int32
		err = db.QueryRow("SELECT COUNT(*) FROM users_orgs WHERE org_id = 1 AND status != 'BLOCK'").Scan(&actualCount)
		require.NoError(t, err)
		
		assert.Equal(t, actualCount, org.MemberCount, "Member count should match database count")
		t.Logf("✓ Member count verified: %d (excluding blocked users)", org.MemberCount)
	})
}
