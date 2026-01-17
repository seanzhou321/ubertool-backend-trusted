package grpc

import (
	"context"

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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.CreateRentalRequest(ctx, userID, req.ToolId, req.OrganizationId, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	return &pb.CreateRentalRequestResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}

func (h *RentalHandler) ApproveRentalRequest(ctx context.Context, req *pb.ApproveRentalRequestRequest) (*pb.ApproveRentalRequestResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.ApproveRentalRequest(ctx, userID, req.RequestId, req.PickupInstructions)
	if err != nil {
		return nil, err
	}
	return &pb.ApproveRentalRequestResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}

func (h *RentalHandler) RejectRentalRequest(ctx context.Context, req *pb.RejectRentalRequestRequest) (*pb.RejectRentalRequestResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.RejectRentalRequest(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.RejectRentalRequestResponse{
		Success:       true,
		RentalRequest: MapDomainRentalToProto(rt),
	}, nil
}

func (h *RentalHandler) FinalizeRentalRequest(ctx context.Context, req *pb.FinalizeRentalRequestRequest) (*pb.FinalizeRentalRequestResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.FinalizeRentalRequest(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.FinalizeRentalRequestResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}

func (h *RentalHandler) CompleteRental(ctx context.Context, req *pb.CompleteRentalRequest) (*pb.CompleteRentalResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.CompleteRental(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.CompleteRentalResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}

func (h *RentalHandler) ListMyRentals(ctx context.Context, req *pb.ListMyRentalsRequest) (*pb.ListRentalsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	statusStr := MapProtoRentalStatusToDomain(req.Status)
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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	statusStr := MapProtoRentalStatusToDomain(req.Status)
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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rt, err := h.rentalSvc.GetRental(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.GetRentalResponse{RentalRequest: MapDomainRentalToProto(rt)}, nil
}
