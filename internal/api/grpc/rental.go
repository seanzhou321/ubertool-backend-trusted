package grpc

import (
	"context"
	"fmt"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type RentalHandler struct {
	pb.UnimplementedRentalServiceServer
	rentalSvc service.RentalService
}

func NewRentalHandler(rentalSvc service.RentalService) *RentalHandler {
	return &RentalHandler{rentalSvc: rentalSvc}
}

func (h *RentalHandler) CreateRentalRequest(ctx context.Context, req *pb.CreateRentalRequestRequest) (*pb.CreateRentalRequestResponse, error) {
	// Need userID from context in real app
	userID := int32(1) // Placeholder
	rt, err := h.rentalSvc.CreateRentalRequest(ctx, userID, req.ToolId, req.OrganizationId, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	return &pb.CreateRentalRequestResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}

func (h *RentalHandler) ApproveRentalRequest(ctx context.Context, req *pb.ApproveRentalRequestRequest) (*pb.ApproveRentalRequestResponse, error) {
	userID := int32(1) // Placeholder
	err := h.rentalSvc.ApproveRentalRequest(ctx, userID, req.RequestId, req.PickupInstructions)
	if err != nil {
		return nil, err
	}
	// Note: proto expects the updated request back, but service doesn't return it yet.
	// For now we'll just return a success stub or fetch it.
	return &pb.ApproveRentalRequestResponse{}, nil
}

func (h *RentalHandler) RejectRentalRequest(ctx context.Context, req *pb.RejectRentalRequestRequest) (*pb.RejectRentalRequestResponse, error) {
	userID := int32(1) // Placeholder
	err := h.rentalSvc.RejectRentalRequest(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.RejectRentalRequestResponse{Success: true}, nil
}

func (h *RentalHandler) FinalizeRentalRequest(ctx context.Context, req *pb.FinalizeRentalRequestRequest) (*pb.FinalizeRentalRequestResponse, error) {
	userID := int32(1) // Placeholder
	err := h.rentalSvc.FinalizeRentalRequest(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.FinalizeRentalRequestResponse{}, nil
}

func (h *RentalHandler) CompleteRental(ctx context.Context, req *pb.CompleteRentalRequest) (*pb.CompleteRentalResponse, error) {
	userID := int32(1) // Placeholder
	// request_id is string in proto for some reason in CompleteRentalRequest? 
	// Let's check proto again. Row 76. Yes, string.
	// We'll convert it.
	var rid int32
	fmt.Sscanf(req.RequestId, "%d", &rid)
	
	err := h.rentalSvc.CompleteRental(ctx, userID, rid)
	if err != nil {
		return nil, err
	}
	return &pb.CompleteRentalResponse{}, nil
}

func (h *RentalHandler) ListMyRentals(ctx context.Context, req *pb.ListMyRentalsRequest) (*pb.ListRentalsResponse, error) {
	userID := int32(1) // Placeholder
	statusStr := "" // Map req.Status to string if needed
	rentals, count, err := h.rentalSvc.ListRentals(ctx, userID, req.OrganizationId, statusStr, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	protoRentals := make([]*pb.RentalRequest, len(rentals))
	for i, r := range rentals {
		protoRentals[i] = MapDomainRentalToProto(&r)
	}
	return &pb.ListRentalsResponse{
		Rentals:    protoRentals,
		TotalCount: count,
	}, nil
}

func (h *RentalHandler) ListMyLendings(ctx context.Context, req *pb.ListMyLendingsRequest) (*pb.ListRentalsResponse, error) {
	userID := int32(1) // Placeholder
	statusStr := ""
	rentals, count, err := h.rentalSvc.ListLendings(ctx, userID, req.OrganizationId, statusStr, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	protoRentals := make([]*pb.RentalRequest, len(rentals))
	for i, r := range rentals {
		protoRentals[i] = MapDomainRentalToProto(&r)
	}
	return &pb.ListRentalsResponse{
		Rentals:    protoRentals,
		TotalCount: count,
	}, nil
}

func (h *RentalHandler) GetRental(ctx context.Context, req *pb.GetRentalRequest) (*pb.GetRentalResponse, error) {
	return &pb.GetRentalResponse{}, nil
}
