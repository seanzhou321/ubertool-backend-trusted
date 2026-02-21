package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
)

type RentalHandler struct {
	pb.UnimplementedRentalServiceServer
	rentalSvc service.RentalService
	userSvc   service.UserService
	toolSvc   service.ToolService
	orgSvc    service.OrganizationService
}

func NewRentalHandler(rentalSvc service.RentalService, userSvc service.UserService, toolSvc service.ToolService, orgSvc service.OrganizationService) *RentalHandler {
	return &RentalHandler{
		rentalSvc: rentalSvc,
		userSvc:   userSvc,
		toolSvc:   toolSvc,
		orgSvc:    orgSvc,
	}
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
	protoRental := h.populateRentalNames(ctx, rt)
	return &pb.CreateRentalRequestResponse{RentalRequest: protoRental}, nil
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
	protoRental := h.populateRentalNames(ctx, rt)
	return &pb.ApproveRentalRequestResponse{RentalRequest: protoRental}, nil
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
	protoRental := h.populateRentalNames(ctx, rt)
	return &pb.RejectRentalRequestResponse{
		Success:       true,
		RentalRequest: protoRental,
	}, nil
}

func (h *RentalHandler) FinalizeRentalRequest(ctx context.Context, req *pb.FinalizeRentalRequestRequest) (*pb.FinalizeRentalRequestResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, approved, pending, err := h.rentalSvc.FinalizeRentalRequest(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}

	protoApproved := make([]*pb.RentalRequest, len(approved))
	for i, r := range approved {
		protoApproved[i] = h.populateRentalNames(ctx, &r)
	}

	protoPending := make([]*pb.RentalRequest, len(pending))
	for i, r := range pending {
		protoPending[i] = h.populateRentalNames(ctx, &r)
	}

	return &pb.FinalizeRentalRequestResponse{
		RentalRequest:   h.populateRentalNames(ctx, rt),
		ApprovedRentals: protoApproved,
		PendingRentals:  protoPending,
	}, nil
}

func (h *RentalHandler) CompleteRental(ctx context.Context, req *pb.CompleteRentalRequest) (*pb.CompleteRentalResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.CompleteRental(ctx, userID, req.RequestId, req.ReturnCondition, req.SurchargeOrCreditCents, req.Notes)
	if err != nil {
		return nil, err
	}
	return &pb.CompleteRentalResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) ListMyRentals(ctx context.Context, req *pb.ListMyRentalsRequest) (*pb.ListRentalsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	statuses := MapProtoRentalStatusesToDomain(req.Status)
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	rentals, count, err := h.rentalSvc.ListRentals(ctx, userID, req.OrganizationId, statuses, page, pageSize)
	if err != nil {
		return nil, err
	}
	protoRentals := make([]*pb.RentalRequest, len(rentals))
	for i, r := range rentals {
		protoRentals[i] = h.populateRentalNames(ctx, &r)
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
	statuses := MapProtoRentalStatusesToDomain(req.Status)
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	rentals, count, err := h.rentalSvc.ListLendings(ctx, userID, req.OrganizationId, statuses, page, pageSize)
	if err != nil {
		return nil, err
	}
	protoRentals := make([]*pb.RentalRequest, len(rentals))
	for i, r := range rentals {
		protoRentals[i] = h.populateRentalNames(ctx, &r)
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
	return &pb.GetRentalResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}
func (h *RentalHandler) CancelRental(ctx context.Context, req *pb.CancelRentalRequest) (*pb.CancelRentalResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.CancelRental(ctx, userID, req.RequestId, req.Reason)
	if err != nil {
		return nil, err
	}
	return &pb.CancelRentalResponse{
		Success:       true,
		RentalRequest: h.populateRentalNames(ctx, rt),
	}, nil
}

// populateRentalNames fetches user, tool, and org names to populate the proto RentalRequest
func (h *RentalHandler) populateRentalNames(ctx context.Context, rental *domain.Rental) *pb.RentalRequest {
	var renterName, ownerName, toolName, toolCondition, orgName string

	// Fetch renter name
	if renter, _, _, err := h.userSvc.GetUserProfile(ctx, rental.RenterID); err == nil && renter != nil {
		renterName = renter.Name
	}

	// Fetch owner name
	if owner, _, _, err := h.userSvc.GetUserProfile(ctx, rental.OwnerID); err == nil && owner != nil {
		ownerName = owner.Name
	}

	// Fetch tool name and condition
	if tool, _, err := h.toolSvc.GetTool(ctx, rental.ToolID, rental.RenterID); err == nil && tool != nil {
		toolName = tool.Name
		toolCondition = string(tool.Condition)
	}

	// Fetch organization name
	if org, err := h.orgSvc.GetOrganization(ctx, rental.OrgID); err == nil && org != nil {
		orgName = org.Name
	}

	return MapDomainRentalToProtoWithNames(rental, renterName, ownerName, toolName, orgName, toolCondition)
}

func (h *RentalHandler) ActivateRental(ctx context.Context, req *pb.ActivateRentalRequest) (*pb.ActivateRentalResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.ActivateRental(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.ActivateRentalResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) ChangeRentalDates(ctx context.Context, req *pb.ChangeRentalDatesRequest) (*pb.ChangeRentalDatesResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.ChangeRentalDates(ctx, userID, req.RequestId, req.NewStartDate, req.NewEndDate, req.OldStartDate, req.OldEndDate)
	if err != nil {
		return nil, err
	}
	return &pb.ChangeRentalDatesResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) ApproveReturnDateChange(ctx context.Context, req *pb.ApproveReturnDateChangeRequest) (*pb.ApproveReturnDateChangeResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.ApproveReturnDateChange(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.ApproveReturnDateChangeResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) RejectReturnDateChange(ctx context.Context, req *pb.RejectReturnDateChangeRequest) (*pb.RejectReturnDateChangeResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.RejectReturnDateChange(ctx, userID, req.RequestId, req.Reason, req.NewEndDate)
	if err != nil {
		return nil, err
	}
	return &pb.RejectReturnDateChangeResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) AcknowledgeReturnDateRejection(ctx context.Context, req *pb.AcknowledgeReturnDateRejectionRequest) (*pb.AcknowledgeReturnDateRejectionResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.AcknowledgeReturnDateRejection(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.AcknowledgeReturnDateRejectionResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) CancelReturnDateChange(ctx context.Context, req *pb.CancelReturnDateChangeRequest) (*pb.CancelReturnDateChangeResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	rt, err := h.rentalSvc.CancelReturnDateChange(ctx, userID, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.CancelReturnDateChangeResponse{RentalRequest: h.populateRentalNames(ctx, rt)}, nil
}

func (h *RentalHandler) ListToolRentals(ctx context.Context, req *pb.ListToolRentalsRequest) (*pb.ListRentalsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	statuses := MapProtoRentalStatusesToDomain(req.Status)
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	rentals, count, err := h.rentalSvc.ListToolRentals(ctx, userID, req.ToolId, req.OrganizationId, statuses, page, pageSize)
	if err != nil {
		return nil, err
	}

	protoRentals := make([]*pb.RentalRequest, len(rentals))
	for i, r := range rentals {
		protoRentals[i] = h.populateRentalNames(ctx, &r)
	}
	return &pb.ListRentalsResponse{Rentals: protoRentals, TotalCount: count}, nil
}
