package e2e

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"ubertool-backend-trusted/internal/config"
	"ubertool-backend-trusted/internal/security"
)

var configPath string
var testConfig *config.Config
var tokenManager security.TokenManager

func init() {
	flag.StringVar(&configPath, "config", "../../config/config.test.yaml", "path to config file")
}

func loadConfig(t *testing.T) *config.Config {
	if testConfig != nil {
		return testConfig
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	// Logic to handle running from root vs package dir
	finalPath := configPath
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		// If running from tests/e2e, try going up
		altPath := filepath.Join("..", "..", configPath)
		if _, err := os.Stat(altPath); err == nil {
			finalPath = altPath
		}
	}

	var err error
	testConfig, err = config.Load(finalPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", finalPath, err)
	}
	tokenManager = security.NewTokenManager(testConfig.JWT.Secret)
	return testConfig
}

// TestDB wraps database connection with helper methods
type TestDB struct {
	*sql.DB
	t *testing.T
}

// PrepareDB creates a database connection for E2E tests
func PrepareDB(t *testing.T) *TestDB {
	cfg := loadConfig(t)
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	var db *sql.DB
	var err error

	// Retry connection as DB might still be starting up
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				return &TestDB{DB: db, t: t}
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("failed to connect to database: %v", err)
	return nil
}

// Cleanup performs test cleanup
func (db *TestDB) Cleanup() {
	db.Exec("DELETE FROM notifications WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	db.Exec("DELETE FROM ledger_transactions WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	// Delete bill_actions first (foreign key to bills)
	db.Exec("DELETE FROM bill_actions WHERE bill_id IN (SELECT id FROM bills WHERE debtor_user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%') OR creditor_user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%'))")
	// Delete bills (foreign key to users and orgs)
	db.Exec("DELETE FROM bills WHERE debtor_user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%') OR creditor_user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	db.Exec("DELETE FROM rentals WHERE renter_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	db.Exec("DELETE FROM tool_images WHERE tool_id IN (SELECT id FROM tools WHERE owner_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%'))")
	db.Exec("DELETE FROM tools WHERE owner_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	db.Exec("DELETE FROM invitations WHERE email LIKE 'e2e-test-%'")
	db.Exec("DELETE FROM join_requests WHERE email LIKE 'e2e-test-%'")
	db.Exec("DELETE FROM users_orgs WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'e2e-test-%')")
	db.Exec("DELETE FROM users WHERE email LIKE 'e2e-test-%'")
	db.Exec("DELETE FROM orgs WHERE name LIKE 'E2E-Test-%'")
}

// CreateTestOrg creates a test organization
func (db *TestDB) CreateTestOrg(name string) int32 {
	if name == "" {
		name = fmt.Sprintf("E2E-Test-Org-%d", time.Now().UnixNano())
	}
	var orgID int32
	err := db.QueryRow(`
		INSERT INTO orgs (name, metro, admin_email, admin_phone_number)
		VALUES ($1, 'San Jose', 'admin@test.com', '555-0000')
		RETURNING id
	`, name).Scan(&orgID)
	if err != nil {
		db.t.Fatalf("failed to create test org: %v", err)
	}
	return orgID
}

// CreateTestUser creates a test user
func (db *TestDB) CreateTestUser(email, name string) int32 {
	if email == "" {
		email = fmt.Sprintf("e2e-test-%d@test.com", time.Now().UnixNano())
	}
	if name == "" {
		name = "Test User"
	}
	var userID int32
	err := db.QueryRow(`
		INSERT INTO users (email, phone_number, password_hash, name)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, email, fmt.Sprintf("555-%d", time.Now().UnixNano()), "hashed_password", name).Scan(&userID)
	if err != nil {
		db.t.Fatalf("failed to create test user: %v", err)
	}
	return userID
}

// AddUserToOrg adds a user to an organization
func (db *TestDB) AddUserToOrg(userID, orgID int32, role, status string, balanceCents int32) {
	_, err := db.Exec(`
		INSERT INTO users_orgs (user_id, org_id, role, status, balance_cents)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, orgID, role, status, balanceCents)
	if err != nil {
		db.t.Fatalf("failed to add user to org: %v", err)
	}
}

// IsUserInOrg checks if a user is already a member of an organization
func (db *TestDB) IsUserInOrg(userID, orgID int32) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, orgID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// CreateTestTool creates a test tool
func (db *TestDB) CreateTestTool(ownerID int32, name string, pricePerDay int32) int32 {
	if name == "" {
		name = fmt.Sprintf("Test Tool %d", time.Now().UnixNano())
	}
	var toolID int32
	err := db.QueryRow(`
		INSERT INTO tools (owner_id, name, description, price_per_day_cents, price_per_week_cents, price_per_month_cents, condition, metro, status)
		VALUES ($1, $2, 'Test tool description', $3, $4, $5, 'EXCELLENT', 'San Jose', 'AVAILABLE')
		RETURNING id
	`, ownerID, name, pricePerDay, pricePerDay*6, pricePerDay*20).Scan(&toolID)
	if err != nil {
		db.t.Fatalf("failed to create test tool: %v", err)
	}
	return toolID
}

// CreateTestToolWithMetro creates a test tool with a specific metro
func (db *TestDB) CreateTestToolWithMetro(ownerID int32, name, metro string, pricePerDay int32) int32 {
	if name == "" {
		name = fmt.Sprintf("Test Tool %d", time.Now().UnixNano())
	}
	var toolID int32
	err := db.QueryRow(`
		INSERT INTO tools (owner_id, name, description, price_per_day_cents, price_per_week_cents, price_per_month_cents, condition, metro, status)
		VALUES ($1, $2, 'Test tool description', $3, $4, $5, 'EXCELLENT', $6, 'AVAILABLE')
		RETURNING id
	`, ownerID, name, pricePerDay, pricePerDay*6, pricePerDay*20, metro).Scan(&toolID)
	if err != nil {
		db.t.Fatalf("failed to create test tool: %v", err)
	}
	return toolID
}

// GetOrgByID retrieves an organization by ID, returns nil if not found
func (db *TestDB) GetOrgByID(orgID int32) *int32 {
	var id int32
	err := db.QueryRow("SELECT id FROM orgs WHERE id = $1", orgID).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// GetUserByID retrieves a user by ID, returns nil if not found
func (db *TestDB) GetUserByID(userID int32) *int32 {
	var id int32
	err := db.QueryRow("SELECT id FROM users WHERE id = $1", userID).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// GetToolByID retrieves a tool by ID, returns nil if not found
func (db *TestDB) GetToolByID(toolID int32) *int32 {
	var id int32
	err := db.QueryRow("SELECT id FROM tools WHERE id = $1 AND deleted_on IS NULL", toolID).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// UpdateToolMetro updates a tool's metro
func (db *TestDB) UpdateToolMetro(toolID int32, metro string) {
	_, err := db.Exec("UPDATE tools SET metro = $1 WHERE id = $2", metro, toolID)
	if err != nil {
		db.t.Fatalf("failed to update tool metro: %v", err)
	}
}

// CreateTestInvitation creates a test invitation and returns the invitation code
func (db *TestDB) CreateTestInvitation(orgID int32, email string, createdBy int32) string {
	var invitationCode string
	err := db.QueryRow(`
		INSERT INTO invitations (invitation_code, org_id, email, created_by, expires_on, created_on)
		VALUES ($1, $2, $3, $4, CURRENT_DATE + INTERVAL '7 days', CURRENT_DATE)
		RETURNING invitation_code
	`, generateTestInvitationCode(), orgID, email, createdBy).Scan(&invitationCode)
	if err != nil {
		db.t.Fatalf("failed to create test invitation: %v", err)
	}
	return invitationCode
}

// generateTestInvitationCode generates a simple test invitation code
// Format: XXX-XXX-XXX (9 uppercase alphanumeric characters with dashes)
func generateTestInvitationCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 9
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[i%len(charset)]
	}
	// Format as XXX-XXX-XXX
	return string(code[0:3]) + "-" + string(code[3:6]) + "-" + string(code[6:9])
}

// GRPCClient wraps gRPC connection with helper methods
type GRPCClient struct {
	conn *grpc.ClientConn
	t    *testing.T
}

// NewGRPCClient creates a new gRPC client connection
func NewGRPCClient(t *testing.T, serverAddr string) *GRPCClient {
	if serverAddr == "" {
		cfg := loadConfig(t)
		serverAddr = cfg.GetServerAddress()
	}

	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to gRPC server: %v", err)
	}

	return &GRPCClient{conn: conn, t: t}
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() {
	c.conn.Close()
}

// Conn returns the underlying gRPC connection
func (c *GRPCClient) Conn() *grpc.ClientConn {
	return c.conn
}

// ContextWithUserID creates a context with user ID and JWT token in metadata
func ContextWithUserID(userID int32) context.Context {
	md := metadata.Pairs("user-id", fmt.Sprintf("%d", userID))
	if tokenManager != nil {
		// Generate a valid token for the test user
		// Note: We use a generic "user" role here; tests needing higher privileges should adjust
		token, _ := tokenManager.GenerateAccessToken(userID, fmt.Sprintf("test%d@example.com", userID), []string{"user"})
		md.Set("authorization", "Bearer "+token)
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

// ContextWithTimeout creates a context with timeout
func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// ContextWithUserIDAndTimeout creates a context with user ID, JWT token and timeout
func ContextWithUserIDAndTimeout(userID int32, timeout time.Duration) (context.Context, context.CancelFunc) {
	md := metadata.Pairs("user-id", fmt.Sprintf("%d", userID))
	if tokenManager != nil {
		token, _ := tokenManager.GenerateAccessToken(userID, fmt.Sprintf("test%d@example.com", userID), []string{"user"})
		md.Set("authorization", "Bearer "+token)
	}
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return context.WithTimeout(ctx, timeout)
}

// CreateTestBill creates a test bill
func (db *TestDB) CreateTestBill(debtorID, creditorID, orgID int32, amountCents int32, settlementMonth, status string) int32 {
	var billID int32
	err := db.QueryRow(`
		INSERT INTO bills (org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id
	`, orgID, debtorID, creditorID, amountCents, settlementMonth, status).Scan(&billID)
	if err != nil {
		db.t.Fatalf("failed to create test bill: %v", err)
	}
	return billID
}

// GetBillByID retrieves a bill by ID
func (db *TestDB) GetBillByID(billID int32) map[string]interface{} {
	var id, debtorID, creditorID, orgID, amountCents int32
	var settlementMonth, status, disputeReason, resolutionOutcome, resolutionNotes string
	var noticeSentAt, debtorAck, creditorAck, disputedAt, resolvedAt sql.NullTime

	err := db.QueryRow(`
		SELECT id, org_id, debtor_user_id, creditor_user_id, amount_cents, settlement_month, status,
		       notice_sent_at, debtor_acknowledged_at, creditor_acknowledged_at, disputed_at, resolved_at,
		       dispute_reason, resolution_outcome, resolution_notes
		FROM bills WHERE id = $1
	`, billID).Scan(&id, &orgID, &debtorID, &creditorID, &amountCents, &settlementMonth, &status,
		&noticeSentAt, &debtorAck, &creditorAck, &disputedAt, &resolvedAt,
		&disputeReason, &resolutionOutcome, &resolutionNotes)

	if err != nil {
		db.t.Fatalf("failed to get bill: %v", err)
	}

	return map[string]interface{}{
		"id":                       id,
		"org_id":                   orgID,
		"debtor_user_id":           debtorID,
		"creditor_user_id":         creditorID,
		"amount_cents":             amountCents,
		"settlement_month":         settlementMonth,
		"status":                   status,
		"notice_sent_at":           noticeSentAt,
		"debtor_acknowledged_at":   debtorAck,
		"creditor_acknowledged_at": creditorAck,
		"disputed_at":              disputedAt,
		"resolved_at":              resolvedAt,
		"dispute_reason":           disputeReason,
		"resolution_outcome":       resolutionOutcome,
		"resolution_notes":         resolutionNotes,
	}
}

// GetUserBalance retrieves a user's balance in an organization
func (db *TestDB) GetUserBalance(userID, orgID int32) int32 {
	var balance int32
	err := db.QueryRow("SELECT balance_cents FROM users_orgs WHERE user_id = $1 AND org_id = $2", userID, orgID).Scan(&balance)
	if err != nil {
		db.t.Fatalf("failed to get user balance: %v", err)
	}
	return balance
}

// SetUserBalance sets a user's balance in an organization
func (db *TestDB) SetUserBalance(userID, orgID int32, balanceCents int32) {
	_, err := db.Exec("UPDATE users_orgs SET balance_cents = $1 WHERE user_id = $2 AND org_id = $3", balanceCents, userID, orgID)
	if err != nil {
		db.t.Fatalf("failed to set user balance: %v", err)
	}
}

// CountBillActions counts the number of actions for a bill
func (db *TestDB) CountBillActions(billID int32) int {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM bill_actions WHERE bill_id = $1", billID).Scan(&count)
	if err != nil {
		db.t.Fatalf("failed to count bill actions: %v", err)
	}
	return count
}

// GetLatestBillAction retrieves the latest action for a bill
func (db *TestDB) GetLatestBillAction(billID int32) map[string]interface{} {
	var id, actorUserID sql.NullInt32
	var actionType, actionDetails, notes string
	var createdAt time.Time

	err := db.QueryRow(`
		SELECT id, actor_user_id, action_type, action_details, notes, created_at
		FROM bill_actions
		WHERE bill_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, billID).Scan(&id, &actorUserID, &actionType, &actionDetails, &notes, &createdAt)

	if err != nil {
		db.t.Fatalf("failed to get latest bill action: %v", err)
	}

	result := map[string]interface{}{
		"action_type":    actionType,
		"action_details": actionDetails,
		"notes":          notes,
		"created_at":     createdAt,
	}

	if actorUserID.Valid {
		result["actor_user_id"] = actorUserID.Int32
	}

	return result
}
