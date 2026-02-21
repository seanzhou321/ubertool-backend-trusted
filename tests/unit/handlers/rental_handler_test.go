package handlers

import (
	"context"
	"testing"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestRentalHandler_CreateRentalRequest(t *testing.T) {
	rentalSvc := new(MockRentalService)
	userSvc := new(MockUserService)
	toolSvc := new(MockToolService)
	orgSvc := new(MockOrganizationService)
	handler := grpc.NewRentalHandler(rentalSvc, userSvc, toolSvc, orgSvc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)
		req := &pb.CreateRentalRequestRequest{
			ToolId:         2,
			OrganizationId: 3,
			StartDate:      "2026-01-20",
			EndDate:        "2026-01-21",
		}

		rental := &domain.Rental{ID: 1, ToolID: 2, RenterID: 1, OrgID: 3}
		rentalSvc.On("CreateRentalRequest", ctx, int32(1), int32(2), int32(3), "2026-01-20", "2026-01-21").
			Return(rental, nil)

		// Mock the name fetching calls (return empty objects instead of nil to avoid nil pointer panics)
		userSvc.On("GetUserProfile", ctx, int32(1)).Return(&domain.User{ID: 1, Name: "Renter"}, []domain.Organization{}, []domain.UserOrg{}, nil)
		userSvc.On("GetUserProfile", ctx, int32(0)).Return(&domain.User{ID: 0, Name: "Owner"}, []domain.Organization{}, []domain.UserOrg{}, nil)
		toolSvc.On("GetTool", ctx, int32(2), int32(1)).Return(&domain.Tool{ID: 2, Name: "TestTool"}, []domain.ToolImage{}, nil)
		orgSvc.On("GetOrganization", ctx, int32(3)).Return(&domain.Organization{ID: 3, Name: "TestOrg"}, nil)

		res, err := handler.CreateRentalRequest(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, int32(1), res.RentalRequest.Id)
	})
}

func TestRentalHandler_ApproveRentalRequest(t *testing.T) {
	rentalSvc := new(MockRentalService)
	userSvc := new(MockUserService)
	toolSvc := new(MockToolService)
	orgSvc := new(MockOrganizationService)
	handler := grpc.NewRentalHandler(rentalSvc, userSvc, toolSvc, orgSvc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)
		req := &pb.ApproveRentalRequestRequest{
			RequestId:          1,
			PickupInstructions: "Note",
		}

		rental := &domain.Rental{ID: 1, Status: domain.RentalStatusApproved, RenterID: 2, OwnerID: 3, ToolID: 4}
		rentalSvc.On("ApproveRentalRequest", ctx, int32(1), int32(1), "Note").Return(rental, nil)

		// Mock the name fetching calls
		userSvc.On("GetUserProfile", ctx, int32(2)).Return(&domain.User{ID: 2, Name: "Renter"}, []domain.Organization{}, []domain.UserOrg{}, nil)
		userSvc.On("GetUserProfile", ctx, int32(3)).Return(&domain.User{ID: 3, Name: "Owner"}, []domain.Organization{}, []domain.UserOrg{}, nil)
		toolSvc.On("GetTool", ctx, int32(4), int32(2)).Return(&domain.Tool{ID: 4, Name: "TestTool"}, []domain.ToolImage{}, nil)
		orgSvc.On("GetOrganization", ctx, int32(0)).Return(&domain.Organization{ID: 0, Name: ""}, nil)

		res, err := handler.ApproveRentalRequest(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})
}
