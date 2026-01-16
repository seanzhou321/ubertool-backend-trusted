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
func (m *MockInviteRepo) GetByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invitation), args.Error(1)
}
func (m *MockInviteRepo) Update(ctx context.Context, invite *domain.Invitation) error {
	args := m.Called(ctx, invite)
	return args.Error(0)
}

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
func (m *MockToolRepo) Search(ctx context.Context, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, orgID, query, categories, maxPrice, condition, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolRepo) AddImage(ctx context.Context, image *domain.ToolImage) error {
	args := m.Called(ctx, image)
	return args.Error(0)
}
func (m *MockToolRepo) GetImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	args := m.Called(ctx, toolID)
	return args.Get(0).([]domain.ToolImage), args.Error(1)
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
