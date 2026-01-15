package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type LedgerHandler struct {
	pb.UnimplementedLedgerServiceServer
	ledgerSvc service.LedgerService
}

func NewLedgerHandler(ledgerSvc service.LedgerService) *LedgerHandler {
	return &LedgerHandler{ledgerSvc: ledgerSvc}
}

func (h *LedgerHandler) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	userID := int32(1) // Placeholder
	balance, err := h.ledgerSvc.GetBalance(ctx, userID, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	return &pb.GetBalanceResponse{Balance: balance}, nil
}

func (h *LedgerHandler) GetTransactions(ctx context.Context, req *pb.GetTransactionsRequest) (*pb.GetTransactionsResponse, error) {
	userID := int32(1) // Placeholder
	txs, count, err := h.ledgerSvc.GetTransactions(ctx, userID, req.OrganizationId, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	protoTxs := make([]*pb.Transaction, len(txs))
	for i, t := range txs {
		protoTxs[i] = mapDomainTransactionToProto(&t)
	}
	return &pb.GetTransactionsResponse{
		Transactions: protoTxs,
		TotalCount:   count,
	}, nil
}

func (h *LedgerHandler) GetLedgerSummary(ctx context.Context, req *pb.GetLedgerSummaryRequest) (*pb.GetLedgerSummaryResponse, error) {
	return &pb.GetLedgerSummaryResponse{}, nil
}
