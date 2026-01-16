package handlers

import (
	"context"
	"testing"

	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"
	pb "ubertool-backend-trusted/api/gen/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestToolHandler_AddTool(t *testing.T) {
	svc := new(MockToolService)
	handler := grpc.NewToolHandler(svc)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &pb.AddToolRequest{
			UserId: 1,
			Name: "Hammer",
			PricePerDayCents: 100,
			Condition: pb.ToolCondition_TOOL_CONDITION_EXCELLENT,
		}

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
		req := &pb.SearchToolsRequest{
			OrganizationId: 1,
			Query: "Drill",
			Page: 1,
			PageSize: 10,
		}

		tools := []domain.Tool{{ID: 1, Name: "Drill", PricePerDayCents: 500, Status: domain.ToolStatusAvailable}}
		svc.On("SearchTools", ctx, int32(1), "Drill", mock.Anything, int32(0), "TOOL_CONDITION_UNSPECIFIED", int32(1), int32(10)).
			Return(tools, int32(1), nil)

		res, err := handler.SearchTools(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 1, len(res.Tools))
		assert.Equal(t, "Drill", res.Tools[0].Name)
	})
}
