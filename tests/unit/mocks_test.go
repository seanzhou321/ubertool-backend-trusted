package unit

import (
	"context"

	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/mock"
)

// MockUserRepo
type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id int32) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepo) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepo) AddUserToOrg(ctx context.Context, userOrg *domain.UserOrg) error {
	args := m.Called(ctx, userOrg)
	return args.Error(0)
}
func (m *MockUserRepo) GetUserOrg(ctx context.Context, userID, orgID int32) (*domain.UserOrg, error) {
	args := m.Called(ctx, userID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserOrg), args.Error(1)
}
func (m *MockUserRepo) ListUserOrgs(ctx context.Context, userID int32) ([]domain.UserOrg, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.UserOrg), args.Error(1)
}
func (m *MockUserRepo) UpdateUserOrg(ctx context.Context, userOrg *domain.UserOrg) error {
	args := m.Called(ctx, userOrg)
	return args.Error(0)
}
func (m *MockUserRepo) ListMembersByOrg(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]domain.User), args.Get(1).([]domain.UserOrg), args.Error(2)
}
func (m *MockUserRepo) SearchMembersByOrg(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error) {
	args := m.Called(ctx, orgID, query)
	return args.Get(0).([]domain.User), args.Get(1).([]domain.UserOrg), args.Error(2)
}

// MockOrganizationRepo
type MockOrganizationRepo struct {
	mock.Mock
}

func (m *MockOrganizationRepo) Create(ctx context.Context, org *domain.Organization) error {
	args := m.Called(ctx, org)
	return args.Error(0)
}
func (m *MockOrganizationRepo) GetByID(ctx context.Context, id int32) (*domain.Organization, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Organization), args.Error(1)
}
func (m *MockOrganizationRepo) List(ctx context.Context) ([]domain.Organization, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Organization), args.Error(1)
}
func (m *MockOrganizationRepo) Search(ctx context.Context, name, metro string) ([]domain.Organization, error) {
	args := m.Called(ctx, name, metro)
	return args.Get(0).([]domain.Organization), args.Error(1)
}
func (m *MockOrganizationRepo) Update(ctx context.Context, org *domain.Organization) error {
	args := m.Called(ctx, org)
	return args.Error(0)
}

// MockInviteRepo
type MockInviteRepo struct {
	mock.Mock
}

func (m *MockInviteRepo) Create(ctx context.Context, invite *domain.Invitation) error {
	args := m.Called(ctx, invite)
	return args.Error(0)
}
func (m *MockInviteRepo) GetByInvitationCode(ctx context.Context, invitationCode string) (*domain.Invitation, error) {
	args := m.Called(ctx, invitationCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invitation), args.Error(1)
}

func (m *MockInviteRepo) GetByInvitationCodeAndEmail(ctx context.Context, invitationCode, email string) (*domain.Invitation, error) {
	args := m.Called(ctx, invitationCode, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invitation), args.Error(1)
}

func (m *MockInviteRepo) Update(ctx context.Context, invite *domain.Invitation) error {
	args := m.Called(ctx, invite)
	return args.Error(0)
}

// Type alias for compatibility
type MockInvitationRepo = MockInviteRepo

// MockJoinRequestRepo
type MockJoinRequestRepo struct {
	mock.Mock
}

func (m *MockJoinRequestRepo) Create(ctx context.Context, req *domain.JoinRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}
func (m *MockJoinRequestRepo) GetByID(ctx context.Context, id int32) (*domain.JoinRequest, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.JoinRequest), args.Error(1)
}
func (m *MockJoinRequestRepo) Update(ctx context.Context, req *domain.JoinRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}
func (m *MockJoinRequestRepo) ListByOrg(ctx context.Context, orgID int32) ([]domain.JoinRequest, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]domain.JoinRequest), args.Error(1)
}

// MockToolRepo
type MockToolRepo struct {
	mock.Mock
}

func (m *MockToolRepo) Create(ctx context.Context, tool *domain.Tool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}
func (m *MockToolRepo) ListByOwner(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, userID, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolRepo) GetByID(ctx context.Context, id int32) (*domain.Tool, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tool), args.Error(1)
}
func (m *MockToolRepo) Update(ctx context.Context, tool *domain.Tool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}
func (m *MockToolRepo) Delete(ctx context.Context, id int32) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockToolRepo) ListByOrg(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, orgID, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolRepo) Search(ctx context.Context, userID int32, metro, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, userID, metro, query, categories, maxPrice, condition, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolRepo) CreateImage(ctx context.Context, image *domain.ToolImage) error {
	args := m.Called(ctx, image)
	return args.Error(0)
}
func (m *MockToolRepo) GetImageByID(ctx context.Context, imageID int32) (*domain.ToolImage, error) {
	args := m.Called(ctx, imageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ToolImage), args.Error(1)
}
func (m *MockToolRepo) GetImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	args := m.Called(ctx, toolID)
	return args.Get(0).([]domain.ToolImage), args.Error(1)
}
func (m *MockToolRepo) GetPendingImagesByUser(ctx context.Context, userID int32) ([]domain.ToolImage, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.ToolImage), args.Error(1)
}
func (m *MockToolRepo) UpdateImage(ctx context.Context, image *domain.ToolImage) error {
	args := m.Called(ctx, image)
	return args.Error(0)
}
func (m *MockToolRepo) ConfirmImage(ctx context.Context, imageID int32, toolID int32) error {
	args := m.Called(ctx, imageID, toolID)
	return args.Error(0)
}
func (m *MockToolRepo) DeleteImage(ctx context.Context, imageID int32) error {
	args := m.Called(ctx, imageID)
	return args.Error(0)
}
func (m *MockToolRepo) SetPrimaryImage(ctx context.Context, toolID, imageID int32) error {
	args := m.Called(ctx, toolID, imageID)
	return args.Error(0)
}
func (m *MockToolRepo) DeleteExpiredPendingImages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockRentalRepo
type MockRentalRepo struct {
	mock.Mock
}

func (m *MockRentalRepo) Create(ctx context.Context, rental *domain.Rental) error {
	args := m.Called(ctx, rental)
	return args.Error(0)
}
func (m *MockRentalRepo) GetByID(ctx context.Context, id int32) (*domain.Rental, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalRepo) Update(ctx context.Context, rental *domain.Rental) error {
	args := m.Called(ctx, rental)
	return args.Error(0)
}
func (m *MockRentalRepo) ListByRenter(ctx context.Context, renterID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, renterID, orgID, status, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalRepo) ListByOwner(ctx context.Context, ownerID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, ownerID, orgID, status, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalRepo) ListByTool(ctx context.Context, toolID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, toolID, orgID, status, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}

// MockLedgerRepo
type MockLedgerRepo struct {
	mock.Mock
}

func (m *MockLedgerRepo) CreateTransaction(ctx context.Context, tx *domain.LedgerTransaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}
func (m *MockLedgerRepo) GetBalance(ctx context.Context, userID, orgID int32) (int32, error) {
	args := m.Called(ctx, userID, orgID)
	return args.Get(0).(int32), args.Error(1)
}
func (m *MockLedgerRepo) ListTransactions(ctx context.Context, userID, orgID int32, page, pageSize int32) ([]domain.LedgerTransaction, int32, error) {
	args := m.Called(ctx, userID, orgID, page, pageSize)
	return args.Get(0).([]domain.LedgerTransaction), args.Get(1).(int32), args.Error(2)
}
func (m *MockLedgerRepo) GetSummary(ctx context.Context, userID, orgID int32) (*domain.LedgerSummary, error) {
	args := m.Called(ctx, userID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LedgerSummary), args.Error(1)
}

// MockEmailService
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendInvitation(ctx context.Context, email, name, token string, orgName string, ccEmail string) error {
	args := m.Called(ctx, email, name, token, orgName, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendAccountStatusNotification(ctx context.Context, email, name, orgName, status, reason string) error {
	args := m.Called(ctx, email, name, orgName, status, reason)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalRequestNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error {
	args := m.Called(ctx, ownerEmail, renterName, toolName, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalApprovalNotification(ctx context.Context, renterEmail, toolName, ownerName, pickupNote string, ccEmail string) error {
	args := m.Called(ctx, renterEmail, toolName, ownerName, pickupNote, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalRejectionNotification(ctx context.Context, renterEmail, toolName, ownerName string, ccEmail string) error {
	args := m.Called(ctx, renterEmail, toolName, ownerName, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalConfirmationNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error {
	args := m.Called(ctx, ownerEmail, renterName, toolName, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalCancellationNotification(ctx context.Context, ownerEmail, renterName, toolName, reason string, ccEmail string) error {
	args := m.Called(ctx, ownerEmail, renterName, toolName, reason, ccEmail)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalCompletionNotification(ctx context.Context, email, role, toolName string, amount int32) error {
	args := m.Called(ctx, email, role, toolName, amount)
	return args.Error(0)
}

func (m *MockEmailService) SendRentalPickupNotification(ctx context.Context, email, name, toolName, startDate, endDate string) error {
	args := m.Called(ctx, email, name, toolName, startDate, endDate)
	return args.Error(0)
}

func (m *MockEmailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	args := m.Called(ctx, adminEmail, subject, message)
	return args.Error(0)
}

// MockNotificationRepo
type MockNotificationRepo struct {
	mock.Mock
}

func (m *MockNotificationRepo) Create(ctx context.Context, note *domain.Notification) error {
	args := m.Called(ctx, note)
	return args.Error(0)
}

func (m *MockNotificationRepo) List(ctx context.Context, userID, limit, offset int32) ([]domain.Notification, int32, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]domain.Notification), args.Get(1).(int32), args.Error(2)
}

func (m *MockNotificationRepo) MarkAsRead(ctx context.Context, id, userID int32) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}
