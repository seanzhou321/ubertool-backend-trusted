package integration

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/jobs"
	"ubertool-backend-trusted/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminNotificationCall records one SendAdminNotification invocation.
type adminNotificationCall struct {
	Email, Subject, Message string
}

// capturingEmailService wraps the package's plain no-op MockEmailService (already satisfying
// service.EmailService for every other method) and records SendAdminNotification calls, which
// is the one method both notification jobs actually use.
type capturingEmailService struct {
	MockEmailService
	mu    sync.Mutex
	calls []adminNotificationCall
}

func (m *capturingEmailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, adminNotificationCall{Email: adminEmail, Subject: subject, Message: message})
	return nil
}

func (m *capturingEmailService) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *capturingEmailService) emailedTo(email string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c.Email == email {
			return true
		}
	}
	return false
}

// TestSendBillSplittingNotices covers FR-002 (specs/008-bill-split): the system must email both
// debtor and creditor when a bill is created, and must record notice_sent_at only once the
// debtor's email succeeds. Prior to this test, `grep -r SendBillSplittingNotices tests/`
// returned nothing.
func TestSendBillSplittingNotices(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	emailSvc := &capturingEmailService{}
	jr := jobs.NewJobRunner(db, postgres.NewStore(db), &jobs.Services{Email: emailSvc}, &config.Config{})

	orgID := createTestOrgForLedger(t, db)
	debtor := createTestUserForLedger(t, db, "notice-debtor")
	creditor := createTestUserForLedger(t, db, "notice-creditor")
	defer func() {
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", debtor, creditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	billID := createTestBill(t, db, orgID, debtor, creditor, "2026-03", "PENDING")

	jr.SendBillSplittingNotices()

	debtorEmail := getUserEmail(t, db, debtor)
	creditorEmail := getUserEmail(t, db, creditor)
	assert.True(t, emailSvc.emailedTo(debtorEmail), "debtor must be emailed")
	assert.True(t, emailSvc.emailedTo(creditorEmail), "creditor must be emailed")

	var noticeSentAt *time.Time
	require.NoError(t, db.QueryRow("SELECT notice_sent_at FROM bills WHERE id = $1", billID).Scan(&noticeSentAt))
	assert.NotNil(t, noticeSentAt, "notice_sent_at must be recorded once the notice is sent")
}

// TestSendBillReminders covers FR-003 (specs/008-bill-split): the system must send a reminder
// email to both parties of a PENDING bill once notice_sent_at is more than 72 hours in the past
// — and must NOT remind a bill whose notice was sent recently. Prior to this test,
// `grep -r SendBillReminders tests/` returned nothing.
func TestSendBillReminders(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	emailSvc := &capturingEmailService{}
	jr := jobs.NewJobRunner(db, postgres.NewStore(db), &jobs.Services{Email: emailSvc}, &config.Config{})

	orgID := createTestOrgForLedger(t, db)
	oldDebtor := createTestUserForLedger(t, db, "reminder-old-debtor")
	oldCreditor := createTestUserForLedger(t, db, "reminder-old-creditor")
	freshDebtor := createTestUserForLedger(t, db, "reminder-fresh-debtor")
	freshCreditor := createTestUserForLedger(t, db, "reminder-fresh-creditor")
	defer func() {
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2, $3, $4)", oldDebtor, oldCreditor, freshDebtor, freshCreditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	oldBillID := createTestBill(t, db, orgID, oldDebtor, oldCreditor, "2026-03", "PENDING")
	_, err := db.Exec("UPDATE bills SET notice_sent_at = NOW() - INTERVAL '4 days' WHERE id = $1", oldBillID)
	require.NoError(t, err)

	freshBillID := createTestBill(t, db, orgID, freshDebtor, freshCreditor, "2026-04", "PENDING")
	_, err = db.Exec("UPDATE bills SET notice_sent_at = NOW() - INTERVAL '1 hour' WHERE id = $1", freshBillID)
	require.NoError(t, err)

	jr.SendBillReminders()

	oldDebtorEmail := getUserEmail(t, db, oldDebtor)
	oldCreditorEmail := getUserEmail(t, db, oldCreditor)
	freshDebtorEmail := getUserEmail(t, db, freshDebtor)
	freshCreditorEmail := getUserEmail(t, db, freshCreditor)

	assert.True(t, emailSvc.emailedTo(oldDebtorEmail), "a bill overdue by more than 72 hours must remind the debtor")
	assert.True(t, emailSvc.emailedTo(oldCreditorEmail), "a bill overdue by more than 72 hours must remind the creditor")
	assert.False(t, emailSvc.emailedTo(freshDebtorEmail), "a bill whose notice was sent recently must not be reminded")
	assert.False(t, emailSvc.emailedTo(freshCreditorEmail), "a bill whose notice was sent recently must not be reminded")
}

// conditionalFailingEmailService wraps MockEmailService and returns a configurable error
// only for SendAdminNotification calls targeting a specific recipient email. For all other
// methods (and for emails not in the fail set) it behaves as a no-op.
//
// This is used by the FR-002 asymmetric-failure tests to exercise the code paths where the
// debtor's or creditor's notice email bounces.
type conditionalFailingEmailService struct {
	MockEmailService
	mu           sync.Mutex
	failForEmail map[string]error // recipient email → error to return (nil = no-op for clarity)
	calls        []adminNotificationCall
}

func newConditionalFailingEmailService() *conditionalFailingEmailService {
	return &conditionalFailingEmailService{
		failForEmail: make(map[string]error),
	}
}

func (m *conditionalFailingEmailService) setFailFor(email string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failForEmail[email] = err
}

func (m *conditionalFailingEmailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err, ok := m.failForEmail[adminEmail]
	if ok {
		return err
	}
	m.calls = append(m.calls, adminNotificationCall{Email: adminEmail, Subject: subject, Message: message})
	return nil
}

func (m *conditionalFailingEmailService) emailedTo(email string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c.Email == email {
			return true
		}
	}
	return false
}

// getNoticedBillNoticeSentAt queries the notice_sent_at for a bill, returning nil if NULL.
func getNoticedBillNoticeSentAt(t *testing.T, db *sql.DB, billID int32) *time.Time {
	t.Helper()
	var noticeSentAt *time.Time
	require.NoError(t, db.QueryRow("SELECT notice_sent_at FROM bills WHERE id = $1", billID).Scan(&noticeSentAt))
	return noticeSentAt
}

// SBR-Trace: FR-002 — debtor email failure leaves notice_sent_at NULL (the bill is skipped
// via `continue` and remains eligible for retry on the next job run);
// does not assert creditor email behavior in this scenario.
func TestSendBillSplittingNotices_DebtorEmailFailure_NoticeSentAtStaysNull(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	emailSvc := newConditionalFailingEmailService()
	jr := jobs.NewJobRunner(db, postgres.NewStore(db), &jobs.Services{Email: emailSvc}, &config.Config{})

	orgID := createTestOrgForLedger(t, db)
	debtor := createTestUserForLedger(t, db, "fail-debtor")
	creditor := createTestUserForLedger(t, db, "fail-creditor")
	defer func() {
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", debtor, creditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	billID := createTestBill(t, db, orgID, debtor, creditor, "2026-05", "PENDING")

	debtorEmail := getUserEmail(t, db, debtor)
	creditorEmail := getUserEmail(t, db, creditor)

	// Force the debtor's email to fail; creditor email succeeds normally.
	emailSvc.setFailFor(debtorEmail, fmt.Errorf("smtp bounce: mail for %s rejected", debtorEmail))

	jr.SendBillSplittingNotices()

	// notice_sent_at must remain NULL — the job skips the bill via `continue` when
	// the debtor's send fails.
	noticeSentAt := getNoticedBillNoticeSentAt(t, db, billID)
	assert.Nil(t, noticeSentAt, "notice_sent_at must stay NULL when the debtor's email send fails")

	// The creditor must never have been emailed (the debtor send short-circuits before it).
	assert.False(t, emailSvc.emailedTo(creditorEmail), "creditor must not be emailed when debtor send fails")
}

// SBR-Trace: FR-002 — debtor email succeeds but creditor email fails: notice_sent_at is still
// stamped because the spec's rule is "only once the debtor's send succeeds";
// does not assert debtor email content.
func TestSendBillSplittingNotices_CreditorFailsDebtorSucceeds(t *testing.T) {
	db := prepareDB(t)
	defer db.Close()

	emailSvc := newConditionalFailingEmailService()
	jr := jobs.NewJobRunner(db, postgres.NewStore(db), &jobs.Services{Email: emailSvc}, &config.Config{})

	orgID := createTestOrgForLedger(t, db)
	debtor := createTestUserForLedger(t, db, "cred-fail-debtor")
	creditor := createTestUserForLedger(t, db, "cred-fail-creditor")
	defer func() {
		db.Exec("DELETE FROM bills WHERE org_id = $1", orgID)
		db.Exec("DELETE FROM users WHERE id IN ($1, $2)", debtor, creditor)
		db.Exec("DELETE FROM orgs WHERE id = $1", orgID)
	}()

	billID := createTestBill(t, db, orgID, debtor, creditor, "2026-06", "PENDING")

	debtorEmail := getUserEmail(t, db, debtor)
	creditorEmail := getUserEmail(t, db, creditor)

	// Debtor email succeeds; force the creditor's email to fail.
	emailSvc.setFailFor(creditorEmail, fmt.Errorf("smtp bounce: mail for %s rejected", creditorEmail))

	jr.SendBillSplittingNotices()

	// notice_sent_at must be set — the spec says it is stamped only once the debtor's
	// send succeeds; a failed creditor email does not block it.
	noticeSentAt := getNoticedBillNoticeSentAt(t, db, billID)
	assert.NotNil(t, noticeSentAt, "notice_sent_at must be set when debtor email succeeds, even if creditor email fails")

	// The debtor must have been emailed successfully.
	assert.True(t, emailSvc.emailedTo(debtorEmail), "debtor must be emailed")
}

func getUserEmail(t *testing.T, db *sql.DB, userID int32) string {
	t.Helper()
	var email string
	require.NoError(t, db.QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&email))
	return email
}
