package handlers

import (
	"context"

	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/mock"
)

// MockToolService
type MockToolService struct {
	mock.Mock
}

func (m *MockToolService) AddTool(ctx context.Context, tool *domain.Tool, images []string) error {
	args := m.Called(ctx, tool, images)
	return args.Error(0)
}
func (m *MockToolService) GetTool(ctx context.Context, id, requestingUserID int32) (*domain.Tool, []domain.ToolImage, error) {
	args := m.Called(ctx, id, requestingUserID)
	return args.Get(0).(*domain.Tool), args.Get(1).([]domain.ToolImage), args.Error(2)
}
func (m *MockToolService) UpdateTool(ctx context.Context, tool *domain.Tool) error {
	args := m.Called(ctx, tool)
	return args.Error(0)
}
func (m *MockToolService) DeleteTool(ctx context.Context, id int32) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockToolService) ListTools(ctx context.Context, orgID, requestingUserID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, orgID, requestingUserID, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolService) SearchTools(ctx context.Context, userID, orgID int32, metro, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, userID, orgID, metro, query, categories, maxPrice, condition, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolService) ListMyTools(ctx context.Context, userID, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, userID, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolService) ListCategories(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

// MockRentalService
type MockRentalService struct {
	mock.Mock
}

func (m *MockRentalService) CreateRentalRequest(ctx context.Context, renterID, toolID, orgID int32, startDate, endDate string) (*domain.Rental, error) {
	args := m.Called(ctx, renterID, toolID, orgID, startDate, endDate)
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID, pickupNote)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) CancelRental(ctx context.Context, renterID, rentalID int32, reason string) (*domain.Rental, error) {
	args := m.Called(ctx, renterID, rentalID, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) (*domain.Rental, []domain.Rental, []domain.Rental, error) {
	args := m.Called(ctx, renterID, rentalID)
	if args.Get(0) == nil {
		return nil, nil, nil, args.Error(3)
	}
	return args.Get(0).(*domain.Rental), args.Get(1).([]domain.Rental), args.Get(2).([]domain.Rental), args.Error(3)
}
func (m *MockRentalService) CompleteRental(ctx context.Context, ownerID, rentalID int32, returnCondition string, surchargeOrCreditCents int32) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID, returnCondition, surchargeOrCreditCents)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) GetRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, userID, rentalID)
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) ListRentals(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, userID, orgID, statuses, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalService) ListLendings(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, userID, orgID, statuses, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalService) ActivateRental(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) ChangeRentalDates(ctx context.Context, userID, rentalID int32, newStart, newEnd, oldStart, oldEnd string) (*domain.Rental, error) {
	args := m.Called(ctx, userID, rentalID, newStart, newEnd, oldStart, oldEnd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) ApproveReturnDateChange(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) RejectReturnDateChange(ctx context.Context, ownerID, rentalID int32, reason, newEndDate string) (*domain.Rental, error) {
	args := m.Called(ctx, ownerID, rentalID, reason, newEndDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) AcknowledgeReturnDateRejection(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, renterID, rentalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) CancelReturnDateChange(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error) {
	args := m.Called(ctx, renterID, rentalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Rental), args.Error(1)
}
func (m *MockRentalService) ListToolRentals(ctx context.Context, ownerID, toolID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, ownerID, toolID, orgID, statuses, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalService) Update(ctx context.Context, rental *domain.Rental) error {
	args := m.Called(ctx, rental)
	return args.Error(0)
}

// MockUserService
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) GetUserProfile(ctx context.Context, userID int32) (*domain.User, []domain.Organization, []domain.UserOrg, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, nil, nil, args.Error(3)
	}
	return args.Get(0).(*domain.User), args.Get(1).([]domain.Organization), args.Get(2).([]domain.UserOrg), args.Error(3)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, userID int32, name, email, phone, avatarURL string) error {
	args := m.Called(ctx, userID, name, email, phone, avatarURL)
	return args.Error(0)
}
