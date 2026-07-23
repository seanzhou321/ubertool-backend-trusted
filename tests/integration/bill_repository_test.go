package integration

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBillRepository_ListDisputedByOrg_ExcludesAdminParty covers FR-012 (specs/008-bill-split):
// ListDisputedPayments must exclude any disputed bill the calling admin is a party to (debtor or
// creditor). Every prior test (unit, integration, e2e) used an admin who was a party-free third
// user, so the exclusion clause itself — as opposed to the admin-authorization gate around it —
// was never actually exercised against a real adversarial fixture where the admin IS a party.
func TestBillRepository_ListDisputedByOrg_ExcludesAdminParty(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	repo := postgres.NewBillRepository(db)
	ctx := context.Background()

	orgID := createTestOrgForLedger(t, db)
	admin := createTestUserForLedger(t, db, "dispute-admin")
	otherDebtor := createTestUserForLedger(t, db, "dispute-other-debtor")
	otherCreditor := createTestUserForLedger(t, db, "dispute-other-creditor")

	defer func() {
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2, $3)", admin, otherDebtor, otherCreditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	// Bill where the admin IS the debtor — must be excluded.
	adminAsDebtorBill := createTestBill(t, db, orgID, admin, otherCreditor, "2026-01", string(domain.BillStatusDisputed))
	// Bill where the admin IS the creditor — must be excluded.
	adminAsCreditorBill := createTestBill(t, db, orgID, otherDebtor, admin, "2026-01", string(domain.BillStatusDisputed))
	// Bill where the admin is not a party at all — must be included.
	uninvolvedBill := createTestBill(t, db, orgID, otherDebtor, otherCreditor, "2026-01", string(domain.BillStatusDisputed))

	bills, err := repo.ListDisputedByOrg(ctx, orgID, &admin)
	require.NoError(t, err)

	var ids []int32
	for _, b := range bills {
		ids = append(ids, b.ID)
	}
	assert.Contains(t, ids, uninvolvedBill, "a disputed bill the admin is not a party to must be included")
	assert.NotContains(t, ids, adminAsDebtorBill, "a disputed bill where the admin is the debtor must be excluded")
	assert.NotContains(t, ids, adminAsCreditorBill, "a disputed bill where the admin is the creditor must be excluded")
}
