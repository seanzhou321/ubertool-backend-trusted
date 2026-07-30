package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type ledgerService struct {
	ledgerRepo repository.LedgerRepository
}

func NewLedgerService(ledgerRepo repository.LedgerRepository) LedgerService {
	return &ledgerService{ledgerRepo: ledgerRepo}
}

func (s *ledgerService) GetBalance(ctx context.Context, userID, orgID int32) (int32, error) {
	return s.ledgerRepo.GetBalance(ctx, userID, orgID)
}

func (s *ledgerService) GetTransactions(ctx context.Context, userID, orgID int32, page, pageSize int32) ([]domain.LedgerTransaction, int32, error) {
	return s.ledgerRepo.ListTransactions(ctx, userID, orgID, page, pageSize)
}

func (s *ledgerService) GetLedgerSummary(ctx context.Context, userID, orgID int32) (*domain.LedgerSummary, error) {
	// FR-004 (specs/007-ledger, multi-org): organization_id omitted (0) means "roll up across
	// every org the caller belongs to" rather than a literal org_id=0 lookup, which matches no
	// row and previously surfaced as an opaque "no rows" error.
	if orgID == 0 {
		return s.ledgerRepo.GetSummaryAllOrgs(ctx, userID)
	}
	return s.ledgerRepo.GetSummary(ctx, userID, orgID)
}
