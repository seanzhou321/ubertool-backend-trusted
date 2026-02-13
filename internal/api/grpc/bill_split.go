package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type BillSplitHandler struct {
	pb.UnimplementedBillSplitServiceServer
	billSplitSvc service.BillSplitService
	userSvc      service.UserService
}

func NewBillSplitHandler(billSplitSvc service.BillSplitService, userSvc service.UserService) *BillSplitHandler {
	return &BillSplitHandler{
		billSplitSvc: billSplitSvc,
		userSvc:      userSvc,
	}
}

func (h *BillSplitHandler) GetGlobalBillSplitSummary(ctx context.Context, req *pb.GetGlobalBillSplitSummaryRequest) (*pb.GetGlobalBillSplitSummaryResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, err := h.billSplitSvc.GetGlobalBillSplitSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &pb.GetGlobalBillSplitSummaryResponse{
		Summary: &pb.BillSplitSummary{
			PaymentsToMake:    paymentsToMake,
			ReceiptsToVerify:  receiptsToVerify,
			PaymentsInDispute: paymentsInDispute,
			ReceiptsInDispute: receiptsInDispute,
		},
	}, nil
}

func (h *BillSplitHandler) GetOrganizationBillSplitSummary(ctx context.Context, req *pb.GetOrganizationBillSplitSummaryRequest) (*pb.GetOrganizationBillSplitSummaryResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	orgs, paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, err := h.billSplitSvc.GetOrganizationBillSplitSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	orgSummaries := make([]*pb.OrganizationBillSplitSummary, len(orgs))
	for i := range orgs {
		orgSummaries[i] = &pb.OrganizationBillSplitSummary{
			OrganizationId:   orgs[i].ID,
			OrganizationName: orgs[i].Name,
			Summary: &pb.BillSplitSummary{
				PaymentsToMake:    paymentsToMake[i],
				ReceiptsToVerify:  receiptsToVerify[i],
				PaymentsInDispute: paymentsInDispute[i],
				ReceiptsInDispute: receiptsInDispute[i],
			},
		}
	}

	return &pb.GetOrganizationBillSplitSummaryResponse{
		OrgSummaries: orgSummaries,
	}, nil
}

func (h *BillSplitHandler) ListPayments(ctx context.Context, req *pb.ListPaymentsRequest) (*pb.ListPaymentsResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bills, err := h.billSplitSvc.ListPayments(ctx, userID, req.OrganizationId, req.ShowHistory)
	if err != nil {
		return nil, err
	}

	payments := make([]*pb.PaymentItem, len(bills))
	for i, bill := range bills {
		payments[i], err = MapDomainBillToPaymentItem(ctx, &bill, userID, h.userSvc)
		if err != nil {
			return nil, err
		}
	}

	return &pb.ListPaymentsResponse{
		Payments: payments,
	}, nil
}

func (h *BillSplitHandler) GetPaymentDetail(ctx context.Context, req *pb.GetPaymentDetailRequest) (*pb.GetPaymentDetailResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bill, actions, canAcknowledge, err := h.billSplitSvc.GetPaymentDetail(ctx, userID, req.PaymentId)
	if err != nil {
		return nil, err
	}

	payment, err := MapDomainBillToPaymentItem(ctx, bill, userID, h.userSvc)
	if err != nil {
		return nil, err
	}

	history := make([]*pb.PaymentAction, len(actions))
	for i, action := range actions {
		history[i], err = MapDomainBillActionToProto(ctx, &action, h.userSvc)
		if err != nil {
			return nil, err
		}
	}

	return &pb.GetPaymentDetailResponse{
		Payment:        payment,
		History:        history,
		CanAcknowledge: canAcknowledge,
	}, nil
}

func (h *BillSplitHandler) AcknowledgePayment(ctx context.Context, req *pb.AcknowledgePaymentRequest) (*pb.AcknowledgePaymentResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = h.billSplitSvc.AcknowledgePayment(ctx, userID, req.PaymentId)
	if err != nil {
		return &pb.AcknowledgePaymentResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.AcknowledgePaymentResponse{
		Success: true,
		Message: "Payment acknowledged successfully",
	}, nil
}

func (h *BillSplitHandler) ListDisputedPayments(ctx context.Context, req *pb.ListDisputedPaymentsRequest) (*pb.ListDisputedPaymentsResponse, error) {
	adminID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bills, err := h.billSplitSvc.ListDisputedPayments(ctx, adminID, req.OrganizationId)
	if err != nil {
		return nil, err
	}

	disputes := make([]*pb.DisputedPaymentItem, len(bills))
	for i, bill := range bills {
		disputes[i], err = MapDomainBillToDisputedPaymentItem(ctx, &bill, h.userSvc)
		if err != nil {
			return nil, err
		}
	}

	return &pb.ListDisputedPaymentsResponse{
		Disputes: disputes,
	}, nil
}

func (h *BillSplitHandler) ListResolvedDisputes(ctx context.Context, req *pb.ListResolvedDisputesRequest) (*pb.ListResolvedDisputesResponse, error) {
	adminID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bills, err := h.billSplitSvc.ListResolvedDisputes(ctx, adminID, req.OrganizationId)
	if err != nil {
		return nil, err
	}

	disputes := make([]*pb.DisputedPaymentItem, len(bills))
	for i, bill := range bills {
		disputes[i], err = MapDomainBillToDisputedPaymentItem(ctx, &bill, h.userSvc)
		if err != nil {
			return nil, err
		}
	}

	return &pb.ListResolvedDisputesResponse{
		Disputes: disputes,
	}, nil
}

func (h *BillSplitHandler) ResolveDispute(ctx context.Context, req *pb.ResolveDisputeRequest) (*pb.ResolveDisputeResponse, error) {
	adminID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	resolution := MapProtoDisputeResolutionToDomain(req.Resolution)
	err = h.billSplitSvc.ResolveDispute(ctx, adminID, req.PaymentId, resolution, req.Notes)
	if err != nil {
		return &pb.ResolveDisputeResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ResolveDisputeResponse{
		Success: true,
		Message: "Dispute resolved successfully",
	}, nil
}
