package handlers

import (
	"context"
	"testing"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

func TestToolHandler_AddTool(t *testing.T) {
	svc := new(MockToolService)
	handler := grpc.NewToolHandler(svc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &pb.AddToolRequest{
			// UserId: 1, // Removed
			Name:             "Hammer",
			PricePerDayCents: 100,
			Condition:        pb.ToolCondition_TOOL_CONDITION_EXCELLENT,
		}

		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)

		svc.On("AddTool", ctx, mock.AnythingOfType("*domain.Tool"), mock.Anything).Return(nil)

		res, err := handler.AddTool(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		svc.AssertCalled(t, "AddTool", ctx, mock.MatchedBy(func(tool *domain.Tool) bool {
			return tool.Name == "Hammer" && tool.PricePerDayCents == 100
		}), mock.Anything)
	})
}

func TestToolHandler_SearchTools(t *testing.T) {
	svc := new(MockToolService)
	handler := grpc.NewToolHandler(svc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		md := metadata.Pairs("user-id", "1")
		ctx = metadata.NewIncomingContext(ctx, md)

		req := &pb.SearchToolsRequest{
			OrganizationId: 1,
			Query:          "Drill",
			Page:           1,
			PageSize:       10,
		}

		tools := []domain.Tool{{ID: 1, Name: "Drill", PricePerDayCents: 500, Status: domain.ToolStatusAvailable}}
		// Note: userID is now passed, assuming context has userID 1 (default in helper/mock)
		svc.On("SearchTools", ctx, int32(1), int32(1), "", "Drill", mock.Anything, int32(0), "", int32(1), int32(10)).
			Return(tools, int32(1), nil)

		res, err := handler.SearchTools(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 1, len(res.Tools))
		assert.Equal(t, "Drill", res.Tools[0].Name)
	})
}
