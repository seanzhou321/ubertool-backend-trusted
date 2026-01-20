package handlers

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"
	pb "ubertool-backend-trusted/api/gen/v1"
	"google.golang.org/grpc/metadata"
	"github.com/stretchr/testify/assert"
)

func TestRentalHandler_CreateRentalRequest(t *testing.T) {
	svc := new(MockRentalService)
	handler := grpc.NewRentalHandler(svc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)
		req := &pb.CreateRentalRequestRequest{
			ToolId: 2,
			OrganizationId: 3,
			StartDate: "2026-01-20",
			EndDate: "2026-01-21",
		}

		rental := &domain.Rental{ID: 1, ToolID: 2, RenterID: 1, OrgID: 3}
		svc.On("CreateRentalRequest", ctx, int32(1), int32(2), int32(3), "2026-01-20", "2026-01-21").
			Return(rental, nil)

		res, err := handler.CreateRentalRequest(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, int32(1), res.RentalRequest.Id)
	})
}

func TestRentalHandler_ApproveRentalRequest(t *testing.T) {
	svc := new(MockRentalService)
	handler := grpc.NewRentalHandler(svc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)
		req := &pb.ApproveRentalRequestRequest{
			RequestId: 1,
			PickupInstructions: "Note",
		}

		rental := &domain.Rental{ID: 1, Status: domain.RentalStatusApproved}
		svc.On("ApproveRentalRequest", ctx, int32(1), int32(1), "Note").Return(rental, nil)

		res, err := handler.ApproveRentalRequest(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})
}
