package service

import (
	"context"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
)

type billSplitService struct {
	billRepo repository.BillRepository
	userRepo repository.UserRepository
	orgRepo  repository.OrganizationRepository
	noteSvc  NotificationService
	emailSvc EmailService
}

func NewBillSplitService(
	billRepo repository.BillRepository,
	userRepo repository.UserRepository,
	orgRepo repository.OrganizationRepository,
	noteSvc NotificationService,
	emailSvc EmailService,
) BillSplitService {
	return &billSplitService{
		billRepo: billRepo,
		userRepo: userRepo,
		orgRepo:  orgRepo,
		noteSvc:  noteSvc,
		emailSvc: emailSvc,
	}
}

func (s *billSplitService) GetGlobalBillSplitSummary(ctx context.Context, userID int32) (int32, int32, int32, int32, error) {
	logger.EnterMethod("billSplitService.GetGlobalBillSplitSummary", "userID", userID)

	// Get all organizations for the user
	userOrgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetGlobalBillSplitSummary", err, "userID", userID)
		return 0, 0, 0, 0, err
	}

	var paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute int32

	for _, userOrg := range userOrgs {
		p, r, pd, rd, err := s.getOrgSummary(ctx, userID, userOrg.OrgID)
		if err != nil {
			continue // Skip this org if there's an error
		}
		paymentsToMake += p
		receiptsToVerify += r
		paymentsInDispute += pd
		receiptsInDispute += rd
	}

	logger.ExitMethod("billSplitService.GetGlobalBillSplitSummary", "userID", userID,
		"paymentsToMake", paymentsToMake, "receiptsToVerify", receiptsToVerify,
		"paymentsInDispute", paymentsInDispute, "receiptsInDispute", receiptsInDispute)

	return paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, nil
}

func (s *billSplitService) GetOrganizationBillSplitSummary(ctx context.Context, userID int32) ([]domain.Organization, []int32, []int32, []int32, []int32, error) {
	logger.EnterMethod("billSplitService.GetOrganizationBillSplitSummary", "userID", userID)

	// Get all organizations for the user
	userOrgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetOrganizationBillSplitSummary", err, "userID", userID)
		return nil, nil, nil, nil, nil, err
	}

	orgs := make([]domain.Organization, 0, len(userOrgs))
	paymentsToMake := make([]int32, 0, len(userOrgs))
	receiptsToVerify := make([]int32, 0, len(userOrgs))
	paymentsInDispute := make([]int32, 0, len(userOrgs))
	receiptsInDispute := make([]int32, 0, len(userOrgs))

	for _, userOrg := range userOrgs {
		org, err := s.orgRepo.GetByID(ctx, userOrg.OrgID)
		if err != nil {
			continue
		}

		p, r, pd, rd, err := s.getOrgSummary(ctx, userID, userOrg.OrgID)
		if err != nil {
			continue
		}

		orgs = append(orgs, *org)
		paymentsToMake = append(paymentsToMake, p)
		receiptsToVerify = append(receiptsToVerify, r)
		paymentsInDispute = append(paymentsInDispute, pd)
		receiptsInDispute = append(receiptsInDispute, rd)
	}

	logger.ExitMethod("billSplitService.GetOrganizationBillSplitSummary", "userID", userID, "orgCount", len(orgs))
	return orgs, paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, nil
}

func (s *billSplitService) getOrgSummary(ctx context.Context, userID, orgID int32) (int32, int32, int32, int32, error) {
	// Get all bills for this user in this org
	bills, err := s.billRepo.ListByUser(ctx, userID, orgID, nil)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	var paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute int32

	for _, bill := range bills {
		isDebtor := bill.DebtorUserID == userID
		isCreditor := bill.CreditorUserID == userID

		switch bill.Status {
		case domain.BillStatusPending:
			if isDebtor && bill.DebtorAcknowledgedAt == nil {
				paymentsToMake++
			} else if isCreditor && bill.DebtorAcknowledgedAt != nil {
				receiptsToVerify++
			}
		case domain.BillStatusDisputed:
			if isDebtor {
				paymentsInDispute++
			} else if isCreditor {
				receiptsInDispute++
			}
		}
	}

	return paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute, nil
}

func (s *billSplitService) ListPayments(ctx context.Context, userID, orgID int32, showHistory bool) ([]domain.Bill, error) {
	logger.EnterMethod("billSplitService.ListPayments", "userID", userID, "orgID", orgID, "showHistory", showHistory)

	// Verify user is a member of the organization
	userOrg, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.ListPayments", err, "userID", userID, "orgID", orgID)
		return nil, fmt.Errorf("user is not a member of this organization")
	}
	if userOrg == nil {
		return nil, fmt.Errorf("user is not a member of this organization")
	}

	// Get all bills for this user in this org
	var bills []domain.Bill
	if showHistory {
		// Return completed bills
		bills, err = s.billRepo.ListByUser(ctx, userID, orgID, []domain.BillStatus{
			domain.BillStatusPaid,
			domain.BillStatusAdminResolved,
			domain.BillStatusSystemDefaultAction,
		})
	} else {
		// Return active bills (pending, disputed)
		bills, err = s.billRepo.ListByUser(ctx, userID, orgID, []domain.BillStatus{
			domain.BillStatusPending,
			domain.BillStatusDisputed,
		})
	}

	if err != nil {
		logger.ExitMethodWithError("billSplitService.ListPayments", err, "userID", userID, "orgID", orgID)
		return nil, err
	}

	logger.ExitMethod("billSplitService.ListPayments", "userID", userID, "orgID", orgID, "count", len(bills))
	return bills, nil
}

func (s *billSplitService) GetPaymentDetail(ctx context.Context, userID, paymentID int32) (*domain.Bill, []domain.BillAction, bool, error) {
	logger.EnterMethod("billSplitService.GetPaymentDetail", "userID", userID, "paymentID", paymentID)

	// Get the bill
	bill, err := s.billRepo.GetByID(ctx, paymentID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetPaymentDetail", err, "paymentID", paymentID)
		return nil, nil, false, err
	}

	// Verify user is involved (debtor, creditor, or admin)
	isInvolved := bill.DebtorUserID == userID || bill.CreditorUserID == userID
	if !isInvolved {
		// Check if user is admin
		userOrg, err := s.userRepo.GetUserOrg(ctx, userID, bill.OrgID)
		if err != nil || userOrg == nil {
			return nil, nil, false, fmt.Errorf("unauthorized to view this payment")
		}
		if userOrg.Role != domain.UserOrgRoleAdmin && userOrg.Role != domain.UserOrgRoleSuperAdmin {
			return nil, nil, false, fmt.Errorf("unauthorized to view this payment")
		}
	}

	// Get bill actions history
	actions, err := s.billRepo.ListActionsByBill(ctx, paymentID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.GetPaymentDetail", err, "paymentID", paymentID)
		return nil, nil, false, err
	}

	// Determine if user can acknowledge
	canAcknowledge := false
	if bill.DebtorUserID == userID && (bill.Status == domain.BillStatusPending || bill.Status == domain.BillStatusDisputed) && bill.DebtorAcknowledgedAt == nil {
		canAcknowledge = true
	} else if bill.CreditorUserID == userID && (bill.Status == domain.BillStatusPending || bill.Status == domain.BillStatusDisputed) && bill.DebtorAcknowledgedAt != nil && bill.CreditorAcknowledgedAt == nil {
		canAcknowledge = true
	}

	logger.ExitMethod("billSplitService.GetPaymentDetail", "paymentID", paymentID, "canAcknowledge", canAcknowledge)
	return bill, actions, canAcknowledge, nil
}

func (s *billSplitService) AcknowledgePayment(ctx context.Context, userID, paymentID int32) error {
	logger.EnterMethod("billSplitService.AcknowledgePayment", "userID", userID, "paymentID", paymentID)

	bill, err := s.billRepo.GetByID(ctx, paymentID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.AcknowledgePayment", err, "paymentID", paymentID)
		return err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	now := time.Now()

	switch {
	case bill.DebtorUserID == userID:
		err = s.acknowledgeAsDebtor(ctx, bill, user, now)
	case bill.CreditorUserID == userID:
		err = s.acknowledgeAsCreditor(ctx, bill, user, now)
	default:
		return fmt.Errorf("user is not involved in this payment")
	}

	if err != nil {
		logger.ExitMethodWithError("billSplitService.AcknowledgePayment", err, "paymentID", paymentID)
		return err
	}

	logger.ExitMethod("billSplitService.AcknowledgePayment", "paymentID", paymentID, "success", true)
	return nil
}

func (s *billSplitService) updateBalances(ctx context.Context, bill *domain.Bill) error {
	// Update creditor's balance (add amount)
	creditorUserOrg, err := s.userRepo.GetUserOrg(ctx, bill.CreditorUserID, bill.OrgID)
	if err != nil {
		return err
	}
	creditorUserOrg.BalanceCents += bill.AmountCents
	nowDate := time.Now().Format("2006-01-02")
	creditorUserOrg.LastBalanceUpdateOn = &nowDate
	if err := s.userRepo.UpdateUserOrg(ctx, creditorUserOrg); err != nil {
		return err
	}

	// Update debtor's balance (subtract amount)
	debtorUserOrg, err := s.userRepo.GetUserOrg(ctx, bill.DebtorUserID, bill.OrgID)
	if err != nil {
		return err
	}
	debtorUserOrg.BalanceCents -= bill.AmountCents
	debtorUserOrg.LastBalanceUpdateOn = &nowDate
	if err := s.userRepo.UpdateUserOrg(ctx, debtorUserOrg); err != nil {
		return err
	}

	return nil
}

func (s *billSplitService) ListDisputedPayments(ctx context.Context, adminID, orgID int32) ([]domain.Bill, error) {
	logger.EnterMethod("billSplitService.ListDisputedPayments", "adminID", adminID, "orgID", orgID)

	if err := s.verifyAdminRights(ctx, adminID, orgID); err != nil {
		logger.ExitMethodWithError("billSplitService.ListDisputedPayments", err, "adminID", adminID, "orgID", orgID)
		return nil, err
	}

	// Get disputed bills, excluding those involving this admin
	bills, err := s.billRepo.ListDisputedByOrg(ctx, orgID, &adminID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.ListDisputedPayments", err, "adminID", adminID, "orgID", orgID)
		return nil, err
	}

	logger.ExitMethod("billSplitService.ListDisputedPayments", "adminID", adminID, "orgID", orgID, "count", len(bills))
	return bills, nil
}

func (s *billSplitService) ListResolvedDisputes(ctx context.Context, adminID, orgID int32) ([]domain.Bill, error) {
	logger.EnterMethod("billSplitService.ListResolvedDisputes", "adminID", adminID, "orgID", orgID)

	if err := s.verifyAdminRights(ctx, adminID, orgID); err != nil {
		logger.ExitMethodWithError("billSplitService.ListResolvedDisputes", err, "adminID", adminID, "orgID", orgID)
		return nil, err
	}

	bills, err := s.billRepo.ListResolvedDisputesByOrg(ctx, orgID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.ListResolvedDisputes", err, "adminID", adminID, "orgID", orgID)
		return nil, err
	}

	logger.ExitMethod("billSplitService.ListResolvedDisputes", "adminID", adminID, "orgID", orgID, "count", len(bills))
	return bills, nil
}

func (s *billSplitService) ResolveDispute(ctx context.Context, adminID, paymentID int32, resolution, notes string) error {
	logger.EnterMethod("billSplitService.ResolveDispute", "adminID", adminID, "paymentID", paymentID, "resolution", resolution, "notes", notes)

	bill, err := s.billRepo.GetByID(ctx, paymentID)
	if err != nil {
		logger.ExitMethodWithError("billSplitService.ResolveDispute", err, "paymentID", paymentID)
		return err
	}

	if err := s.verifyAdminRights(ctx, adminID, bill.OrgID); err != nil {
		logger.ExitMethodWithError("billSplitService.ResolveDispute", err, "adminID", adminID, "orgID", bill.OrgID)
		return err
	}

	if bill.DebtorUserID == adminID || bill.CreditorUserID == adminID {
		return fmt.Errorf("admins cannot resolve disputes they are involved in")
	}
	if bill.Status != domain.BillStatusDisputed {
		return fmt.Errorf("payment is not in disputed status")
	}

	now := time.Now()
	bill.Status = domain.BillStatusAdminResolved
	bill.ResolvedAt = &now
	bill.ResolutionOutcome = resolution
	bill.ResolutionNotes = notes

	switch resolution {
	case string(domain.ResolutionOutcomeDebtorFault):
		if err := s.updateBalances(ctx, bill); err != nil {
			return fmt.Errorf("failed to update balances: %w", err)
		}
		s.blockDebtorFromRenting(ctx, bill.DebtorUserID, bill.OrgID, bill, "Blocked due to unresolved payment dispute (debtor at fault)")

	case string(domain.ResolutionOutcomeCreditorFault):
		s.penalizeAndBlockCreditorFromLending(ctx, bill.CreditorUserID, bill.OrgID, bill, "Blocked due to dispute resolution (creditor at fault)")

	case string(domain.ResolutionOutcomeBothFault):
		s.penalizeAndBlockDebtorFromRenting(ctx, bill.DebtorUserID, bill.OrgID, bill, "Blocked due to unresolved payment dispute (both at fault)")
		s.penalizeAndBlockCreditorFromLending(ctx, bill.CreditorUserID, bill.OrgID, bill, "Blocked due to unresolved payment dispute (both at fault)")

	case string(domain.ResolutionOutcomeGraceful):
		if err := s.updateBalances(ctx, bill); err != nil {
			return fmt.Errorf("failed to update balances: %w", err)
		}

	default:
		return fmt.Errorf("invalid resolution type: %s", resolution)
	}

	if err := s.billRepo.Update(ctx, bill); err != nil {
		logger.ExitMethodWithError("billSplitService.ResolveDispute", err, "paymentID", paymentID)
		return err
	}

	action := &domain.BillAction{
		BillID:      bill.ID,
		ActorUserID: &adminID,
		ActionType:  domain.BillActionTypeAdminResolution,
		Notes:       notes,
		CreatedAt:   now,
	}
	_ = s.billRepo.CreateAction(ctx, action)

	orgName := s.getOrgName(ctx, bill.OrgID)
	debtor, _ := s.userRepo.GetByID(ctx, bill.DebtorUserID)
	creditor, _ := s.userRepo.GetByID(ctx, bill.CreditorUserID)
	if debtor != nil {
		s.sendDisputeResolutionNotification(ctx, debtor, bill, resolution, notes, orgName)
	}
	if creditor != nil {
		s.sendDisputeResolutionNotification(ctx, creditor, bill, resolution, notes, orgName)
	}

	logger.ExitMethod("billSplitService.ResolveDispute", "paymentID", paymentID, "success", true)
	return nil
}

func (s *billSplitService) getOrgName(ctx context.Context, orgID int32) string {
	org, _ := s.orgRepo.GetByID(ctx, orgID)
	if org != nil {
		return org.Name
	}
	return ""
}

func (s *billSplitService) verifyAdminRights(ctx context.Context, adminID, orgID int32) error {
	userOrg, err := s.userRepo.GetUserOrg(ctx, adminID, orgID)
	if err != nil {
		return fmt.Errorf("unauthorized: not a member of this organization")
	}
	if userOrg.Role != domain.UserOrgRoleAdmin && userOrg.Role != domain.UserOrgRoleSuperAdmin {
		return fmt.Errorf("unauthorized: admin privileges required")
	}
	return nil
}

func (s *billSplitService) blockDebtorFromRenting(ctx context.Context, debtorID, orgID int32, bill *domain.Bill, reason string) {
	userOrg, err := s.userRepo.GetUserOrg(ctx, debtorID, orgID)
	if err == nil {
		userOrg.RentingBlocked = true
		userOrg.BlockedDueToBillID = &bill.ID
		userOrg.BlockedReason = reason
		_ = s.userRepo.UpdateUserOrg(ctx, userOrg)
	}
}

func (s *billSplitService) penalizeAndBlockDebtorFromRenting(ctx context.Context, debtorID, orgID int32, bill *domain.Bill, reason string) {
	userOrg, err := s.userRepo.GetUserOrg(ctx, debtorID, orgID)
	if err == nil {
		userOrg.BalanceCents -= bill.AmountCents
		nowDate := time.Now().Format("2006-01-02")
		userOrg.LastBalanceUpdateOn = &nowDate
		userOrg.RentingBlocked = true
		userOrg.BlockedDueToBillID = &bill.ID
		userOrg.BlockedReason = reason
		_ = s.userRepo.UpdateUserOrg(ctx, userOrg)
	}
}

func (s *billSplitService) penalizeAndBlockCreditorFromLending(ctx context.Context, creditorID, orgID int32, bill *domain.Bill, reason string) {
	userOrg, err := s.userRepo.GetUserOrg(ctx, creditorID, orgID)
	if err == nil {
		userOrg.BalanceCents -= bill.AmountCents
		nowDate := time.Now().Format("2006-01-02")
		userOrg.LastBalanceUpdateOn = &nowDate
		userOrg.LendingBlocked = true
		userOrg.BlockedDueToBillID = &bill.ID
		userOrg.BlockedReason = reason
		_ = s.userRepo.UpdateUserOrg(ctx, userOrg)
	}
}

func (s *billSplitService) sendDisputeResolutionNotification(ctx context.Context, user *domain.User, bill *domain.Bill, resolution, notes, orgName string) {
	notification := &domain.Notification{
		UserID:  user.ID,
		OrgID:   bill.OrgID,
		Title:   "Dispute Resolved",
		Message: fmt.Sprintf("Dispute resolved by admin: %s", resolution),
		Attributes: map[string]string{
			"topic":      "bill_dispute_resolved",
			"bill_id":    fmt.Sprintf("%d", bill.ID),
			"resolution": resolution,
			"channel_id": string(domain.ChannelDispute),
		},
	}
	_ = s.noteSvc.Dispatch(ctx, notification)
	_ = s.emailSvc.SendBillDisputeResolutionNotification(ctx, user.Email, user.Name, bill.AmountCents, resolution, notes, orgName)
}

func (s *billSplitService) acknowledgeAsDebtor(ctx context.Context, bill *domain.Bill, user *domain.User, now time.Time) error {
	if bill.Status != domain.BillStatusPending && bill.Status != domain.BillStatusDisputed {
		return fmt.Errorf("payment is not in pending or disputed status")
	}
	if bill.DebtorAcknowledgedAt != nil {
		return fmt.Errorf("payment already acknowledged by debtor")
	}

	bill.DebtorAcknowledgedAt = &now
	if err := s.billRepo.Update(ctx, bill); err != nil {
		return err
	}

	action := &domain.BillAction{
		BillID:      bill.ID,
		ActorUserID: &user.ID,
		ActionType:  domain.BillActionTypeDebtorAcknowledged,
		Notes:       "Debtor acknowledged payment sent",
		CreatedAt:   now,
	}
	_ = s.billRepo.CreateAction(ctx, action)

	creditor, err := s.userRepo.GetByID(ctx, bill.CreditorUserID)
	if err == nil {
		orgName := s.getOrgName(ctx, bill.OrgID)
		notification := &domain.Notification{
			UserID:  creditor.ID,
			OrgID:   bill.OrgID,
			Title:   "Payment Acknowledged",
			Message: fmt.Sprintf("%s acknowledged sending payment of $%.2f for %s settlement", user.Name, float64(bill.AmountCents)/100, bill.SettlementMonth),
			Attributes: map[string]string{
				"topic":        "bill_payment_acknowledged",
				"bill_id":      fmt.Sprintf("%d", bill.ID),
				"debtor_id":    fmt.Sprintf("%d", bill.DebtorUserID),
				"amount_cents": fmt.Sprintf("%d", bill.AmountCents),
				"channel_id":   string(domain.ChannelBillSplitting),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notification)
		_ = s.emailSvc.SendBillPaymentAcknowledgment(ctx, creditor.Email, creditor.Name, user.Name, bill.AmountCents, bill.SettlementMonth, orgName)
	}
	return nil
}

func (s *billSplitService) acknowledgeAsCreditor(ctx context.Context, bill *domain.Bill, user *domain.User, now time.Time) error {
	if bill.Status != domain.BillStatusPending && bill.Status != domain.BillStatusDisputed {
		return fmt.Errorf("payment is not in pending or disputed status")
	}
	if bill.DebtorAcknowledgedAt == nil {
		return fmt.Errorf("debtor has not acknowledged payment yet")
	}
	if bill.CreditorAcknowledgedAt != nil {
		return fmt.Errorf("payment already acknowledged by creditor")
	}

	bill.CreditorAcknowledgedAt = &now
	bill.Status = domain.BillStatusPaid
	bill.ResolvedAt = &now
	bill.ResolutionOutcome = string(domain.ResolutionOutcomeGraceful)
	if err := s.billRepo.Update(ctx, bill); err != nil {
		return err
	}

	action := &domain.BillAction{
		BillID:      bill.ID,
		ActorUserID: &user.ID,
		ActionType:  domain.BillActionTypeCreditorAcknowledged,
		Notes:       "Creditor acknowledged payment received",
		CreatedAt:   now,
	}
	_ = s.billRepo.CreateAction(ctx, action)

	if err := s.updateBalances(ctx, bill); err != nil {
		return fmt.Errorf("failed to update balances: %w", err)
	}

	debtor, err := s.userRepo.GetByID(ctx, bill.DebtorUserID)
	if err == nil {
		orgName := s.getOrgName(ctx, bill.OrgID)
		notification := &domain.Notification{
			UserID:  debtor.ID,
			OrgID:   bill.OrgID,
			Title:   "Payment Receipt Confirmed",
			Message: fmt.Sprintf("%s confirmed receiving payment of $%.2f for %s settlement", user.Name, float64(bill.AmountCents)/100, bill.SettlementMonth),
			Attributes: map[string]string{
				"topic":        "bill_receipt_confirmed",
				"bill_id":      fmt.Sprintf("%d", bill.ID),
				"creditor_id":  fmt.Sprintf("%d", bill.CreditorUserID),
				"amount_cents": fmt.Sprintf("%d", bill.AmountCents),
				"channel_id":   string(domain.ChannelBillSplitting),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notification)
		_ = s.emailSvc.SendBillReceiptConfirmation(ctx, debtor.Email, debtor.Name, user.Name, bill.AmountCents, bill.SettlementMonth, orgName)
	}
	return nil
}
