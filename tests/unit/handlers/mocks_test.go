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
func (m *MockToolService) GetTool(ctx context.Context, id int32) (*domain.Tool, []domain.ToolImage, error) {
	args := m.Called(ctx, id)
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
func (m *MockToolService) ListTools(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, orgID, page, pageSize)
	return args.Get(0).([]domain.Tool), args.Get(1).(int32), args.Error(2)
}
func (m *MockToolService) SearchTools(ctx context.Context, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	args := m.Called(ctx, orgID, query, categories, maxPrice, condition, page, pageSize)
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
func (m *MockRentalService) ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) error {
	args := m.Called(ctx, ownerID, rentalID, pickupNote)
	return args.Error(0)
}
func (m *MockRentalService) RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) error {
	args := m.Called(ctx, ownerID, rentalID)
	return args.Error(0)
}
func (m *MockRentalService) FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) error {
	args := m.Called(ctx, renterID, rentalID)
	return args.Error(0)
}
func (m *MockRentalService) CompleteRental(ctx context.Context, ownerID, rentalID int32) error {
	args := m.Called(ctx, ownerID, rentalID)
	return args.Error(0)
}
func (m *MockRentalService) ListRentals(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, userID, orgID, status, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
func (m *MockRentalService) ListLendings(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error) {
	args := m.Called(ctx, userID, orgID, status, page, pageSize)
	return args.Get(0).([]domain.Rental), args.Get(1).(int32), args.Error(2)
}
