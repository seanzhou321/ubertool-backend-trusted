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
	err = h.toolSvc.AddTool(ctx, tool, req.ImageUrl)
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

func (h *ToolHandler) ListMyTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	tools, count, err := h.toolSvc.ListMyTools(ctx, userID, page, pageSize)
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
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

func (h *ToolHandler) SearchTools(ctx context.Context, req *pb.SearchToolsRequest) (*pb.SearchToolsResponse, error) {
	userID, err := GetUserIDFromContext(ctx) // Optional? No, required for org check
	if err != nil {
		// For search, maybe allow public search if orgID is not set? 
		// But interface requires userID now.
		// If orgID is set, we need userID. If not, maybe pass 0?
		// Design says "Input: organization_id, query... Business Logic: if organization_id is given, verify user_id..."
		// So if orgID is missing, userID might not be strictly needed for check, but authentication is usually required for API.
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	tools, count, err := h.toolSvc.SearchTools(ctx, userID, req.OrganizationId, req.Query, req.Categories, req.MaxPrice, req.Condition.String(), page, pageSize)
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
