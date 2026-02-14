package integration

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/repository/postgres"
)

func TestPerformBillSplittingForOrg(t *testing.T) {
	// Mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Setup Config
	cfg := &config.Config{
		Billing: config.BillingConfig{
			SettlementThresholdCents: 500, // $5.00
		},
	}

	// Setup JobRunner
	// We only need DB and Config for this function
	jr := jobs.NewJobRunner(db, &postgres.Store{}, nil, cfg)

	// Test Data
	orgID := int32(101)
	orgName := "Test Org"
	settlementMonth := "2026-02"

	// Mock SELECT query for user balances
	// Scenario: A owes B $10.00 (1000 cents). Threshold is $5.00.
	// Users: A (-1000), B (+1000)
	rows := sqlmock.NewRows([]string{"user_id", "balance_cents"}).
		AddRow(1, -1000).
		AddRow(2, 1000)

	mock.ExpectQuery(`SELECT user_id, balance_cents FROM users_orgs WHERE org_id = \$1 AND status = 'ACTIVE' AND balance_cents != 0`).
		WithArgs(orgID).
		WillReturnRows(rows)

	// Mock INSERT statements for bills
	// Algorithm: A pays B 1000.
	// Expect 1 bill.
	mock.ExpectExec(`INSERT INTO bills`).
		WithArgs(
			orgID,
			1,    // Debtor (User 1)
			2,    // Creditor (User 2)
			1000, // Amount
			settlementMonth,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call function
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := jr.PerformBillSplittingForOrg(ctx, orgID, orgName, settlementMonth)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Ensure all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPerformBillSplittingForOrg_BelowThreshold(t *testing.T) {
	// Mock DB
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	cfg := &config.Config{
		Billing: config.BillingConfig{
			SettlementThresholdCents: 500, // $5.00
		},
	}
	jr := jobs.NewJobRunner(db, &postgres.Store{}, nil, cfg)

	orgID := int32(102)
	orgName := "Small Org"
	settlementMonth := "2026-02"

	// Users: A (-400), B (+400). Both < 500.
	rows := sqlmock.NewRows([]string{"user_id", "balance_cents"}).
		AddRow(1, -400).
		AddRow(2, 400)

	mock.ExpectQuery(`SELECT user_id, balance_cents FROM users_orgs WHERE org_id = \$1`).
		WithArgs(orgID).
		WillReturnRows(rows)

	// Expect NO insert statements because balances are below threshold
	
	count, err := jr.PerformBillSplittingForOrg(context.Background(), orgID, orgName, settlementMonth)

	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
