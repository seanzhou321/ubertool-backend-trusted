package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
)

type ToolHandler struct {
	pb.UnimplementedToolServiceServer
	toolSvc service.ToolService
}

func NewToolHandler(toolSvc service.ToolService) *ToolHandler {
	return &ToolHandler{toolSvc: toolSvc}
}

func (h *ToolHandler) AddTool(ctx context.Context, req *pb.AddToolRequest) (*pb.AddToolResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	tool := &domain.Tool{
		OwnerID:              userID,
		Name:                 req.Name,
		Description:          req.Description,
		Categories:           req.Categories,
		PricePerDayCents:    req.PricePerDayCents,
		PricePerWeekCents:   req.PricePerWeekCents,
		PricePerMonthCents:  req.PricePerMonthCents,
		ReplacementCostCents: req.ReplacementCostCents,
		Condition:            domain.ToolCondition(req.Condition),
		Status:               domain.ToolStatusAvailable,
	}
	err = h.toolSvc.AddTool(ctx, tool, []string{req.ImageUrl})
	if err != nil {
		return nil, err
	}
	return &pb.AddToolResponse{Tool: MapDomainToolToProto(tool)}, nil
}

func (h *ToolHandler) GetTool(ctx context.Context, req *pb.GetToolRequest) (*pb.GetToolResponse, error) {
	tool, _, err := h.toolSvc.GetTool(ctx, req.ToolId)
	if err != nil {
		return nil, err
	}
	return &pb.GetToolResponse{
		Tool: MapDomainToolToProto(tool),
	}, nil
}

func (h *ToolHandler) UpdateTool(ctx context.Context, req *pb.UpdateToolRequest) (*pb.UpdateToolResponse, error) {
	tool := &domain.Tool{
		ID:                   req.ToolId,
		Name:                 req.Name,
		Description:          req.Description,
		Categories:           req.Categories,
		PricePerDayCents:    req.PricePerDayCents,
		PricePerWeekCents:   req.PricePerWeekCents,
		PricePerMonthCents:  req.PricePerMonthCents,
		ReplacementCostCents: req.ReplacementCostCents,
		Condition:            domain.ToolCondition(req.Condition),
	}
	err := h.toolSvc.UpdateTool(ctx, tool)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateToolResponse{Tool: MapDomainToolToProto(tool)}, nil
}

func (h *ToolHandler) DeleteTool(ctx context.Context, req *pb.DeleteToolRequest) (*pb.DeleteToolResponse, error) {
	err := h.toolSvc.DeleteTool(ctx, req.ToolId)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteToolResponse{Success: true}, nil
}

func (h *ToolHandler) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	tools, count, err := h.toolSvc.ListTools(ctx, req.OrganizationId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	protoTools := make([]*pb.Tool, len(tools))
	for i, t := range tools {
		protoTools[i] = MapDomainToolToProto(&t)
	}
	return &pb.ListToolsResponse{
		Tools:      protoTools,
		TotalCount: count,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

func (h *ToolHandler) SearchTools(ctx context.Context, req *pb.SearchToolsRequest) (*pb.SearchToolsResponse, error) {
	tools, count, err := h.toolSvc.SearchTools(ctx, req.OrganizationId, req.Query, req.Categories, req.MaxPrice, req.Condition.String(), req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	protoTools := make([]*pb.Tool, len(tools))
	for i, t := range tools {
		protoTools[i] = MapDomainToolToProto(&t)
	}
	return &pb.SearchToolsResponse{
		Tools:      protoTools,
		TotalCount: count,
	}, nil
}

func (h *ToolHandler) ListToolCategories(ctx context.Context, req *pb.ListToolCategoriesRequest) (*pb.ListToolCategoriesResponse, error) {
	cats, err := h.toolSvc.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ListToolCategoriesResponse{Categories: cats}, nil
}
