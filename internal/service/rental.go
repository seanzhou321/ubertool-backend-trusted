package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
	"ubertool-backend-trusted/internal/utils"
)

type rentalService struct {
	rentalRepo repository.RentalRepository
	toolRepo   repository.ToolRepository
	ledgerRepo repository.LedgerRepository
	userRepo   repository.UserRepository
	emailSvc   EmailService
	noteRepo   repository.NotificationRepository
}

func NewRentalService(
	rentalRepo repository.RentalRepository,
	toolRepo repository.ToolRepository,
	ledgerRepo repository.LedgerRepository,
	userRepo repository.UserRepository,
	emailSvc EmailService,
	noteRepo repository.NotificationRepository,
) RentalService {
	return &rentalService{
		rentalRepo: rentalRepo,
		toolRepo:   toolRepo,
		ledgerRepo: ledgerRepo,
		userRepo:   userRepo,
		emailSvc:   emailSvc,
		noteRepo:   noteRepo,
	}
}

func (s *rentalService) CreateRentalRequest(ctx context.Context, renterID, toolID, orgID int32, startDateStr, endDateStr string) (*domain.Rental, error) {
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, err
	}
	// Verify tool availability (simplified: check status)
	// Ideally check if tool is already rented in this period.

	start, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return nil, err
	}

	// Calculate cost using tiered pricing algorithm
	totalCost, err := utils.CalculateRentalCost(start, end, tool)
	if err != nil {
		return nil, err
	}

	// Check balance - DISABLED FOR NOW
	// TODO: Re-enable balance check once payment system is implemented
	// balance, err := s.ledgerRepo.GetBalance(ctx, renterID, orgID)
	// if err != nil {
	// 	return nil, err
	// }
	// if balance < totalCost {
	// 	return nil, status.Errorf(codes.FailedPrecondition, "insufficient balance: need %d cents, have %d cents", totalCost, balance)
	// }

	rental := &domain.Rental{
		OrgID:          orgID,
		ToolID:         toolID,
		RenterID:       renterID,
		OwnerID:        tool.OwnerID,
		StartDate:      start.Format("2006-01-02"),
		EndDate:        end.Format("2006-01-02"),
		TotalCostCents: totalCost,
		Status:         domain.RentalStatusPending,
	}

	if err := s.rentalRepo.Create(ctx, rental); err != nil {
		return nil, err
	}

	// Notify owner
	owner, _ := s.userRepo.GetByID(ctx, tool.OwnerID)
	renter, _ := s.userRepo.GetByID(ctx, renterID)
	if owner != nil && renter != nil {
		_ = s.emailSvc.SendRentalRequestNotification(ctx, owner.Email, renter.Name, tool.Name, renter.Email)

		notif := &domain.Notification{
			UserID:  owner.ID,
			OrgID:   orgID,
			Title:   "New Rental Request",
			Message: fmt.Sprintf("%s requested to rent %s", renter.Name, tool.Name),
			Attributes: map[string]string{
				"type":      "RENTAL_REQUEST",
				"rental_id": fmt.Sprintf("%d", rental.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}

	return rental, nil
}

func (s *rentalService) ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusPending {
		return nil, errors.New("rental is not pending")
	}

	rt.Status = domain.RentalStatusApproved
	rt.PickupNote = pickupNote
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify renter
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	owner, _ := s.userRepo.GetByID(ctx, ownerID)
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)

	if renter != nil && owner != nil && tool != nil {
		_ = s.emailSvc.SendRentalApprovalNotification(ctx, renter.Email, tool.Name, owner.Name, pickupNote, owner.Email)

		notif := &domain.Notification{
			UserID:  renter.ID,
			OrgID:   rt.OrgID,
			Title:   "Rental Approved",
			Message: fmt.Sprintf("Your rental request for %s by %s was approved", tool.Name, owner.Name),
			Attributes: map[string]string{
				"type":      "RENTAL_APPROVED",
				"rental_id": fmt.Sprintf("%d", rt.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}

	return rt, nil
}

func (s *rentalService) RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}

	rt.Status = domain.RentalStatusRejected
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify renter
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	owner, _ := s.userRepo.GetByID(ctx, ownerID)
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)

	if renter != nil && owner != nil && tool != nil {
		_ = s.emailSvc.SendRentalRejectionNotification(ctx, renter.Email, tool.Name, owner.Name, owner.Email)

		notif := &domain.Notification{
			UserID:  renter.ID,
			OrgID:   rt.OrgID,
			Title:   "Rental Rejected",
			Message: fmt.Sprintf("Your rental request for %s by %s was rejected", tool.Name, owner.Name),
			Attributes: map[string]string{
				"type":      "RENTAL_REJECTED",
				"rental_id": fmt.Sprintf("%d", rt.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}
	return rt, nil
}

func (s *rentalService) CancelRental(ctx context.Context, renterID, rentalID int32, reason string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != renterID {
		return nil, errors.New("unauthorized")
	}

	rt.Status = domain.RentalStatusCancelled
	// rt.CancelReason = reason // Not in domain
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify owner
	renter, _ := s.userRepo.GetByID(ctx, renterID)
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)

	if renter != nil && owner != nil && tool != nil {
		_ = s.emailSvc.SendRentalCancellationNotification(ctx, owner.Email, renter.Name, tool.Name, reason, renter.Email)

		notif := &domain.Notification{
			UserID:  owner.ID,
			OrgID:   rt.OrgID,
			Title:   "Rental Cancelled",
			Message: fmt.Sprintf("%s cancelled rental request for %s", renter.Name, tool.Name),
			Attributes: map[string]string{
				"type":      "RENTAL_CANCELLED",
				"rental_id": fmt.Sprintf("%d", rt.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}

	return rt, nil
}

func (s *rentalService) FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) (*domain.Rental, []domain.Rental, []domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, nil, nil, err
	}
	if rt.RenterID != renterID {
		return nil, nil, nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusApproved {
		return nil, nil, nil, errors.New("rental is not approved by owner")
	}

	// No payment deduction at this stage - all transactions happen at rental completion

	// Update rental
	rt.Status = domain.RentalStatusScheduled
	// Copy EndDate to LastAgreedEndDate since renter has confirmed the rental
	rt.LastAgreedEndDate = &rt.EndDate
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, nil, nil, err
	}

	// Update tool status
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)
	if tool != nil {
		tool.Status = domain.ToolStatusRented
		_ = s.toolRepo.Update(ctx, tool)
	}

	// Notify owner
	renter, _ := s.userRepo.GetByID(ctx, renterID)
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)

	if renter != nil && owner != nil && tool != nil {
		_ = s.emailSvc.SendRentalConfirmationNotification(ctx, owner.Email, renter.Name, tool.Name, renter.Email)

		notif := &domain.Notification{
			UserID:  owner.ID,
			OrgID:   rt.OrgID,
			Title:   "Rental Confirmed",
			Message: fmt.Sprintf("%s confirmed rental for %s", renter.Name, tool.Name),
			Attributes: map[string]string{
				"type":      "RENTAL_CONFIRMED",
				"rental_id": fmt.Sprintf("%d", rt.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}

	// Search other rentals for lists
	approved, _, err := s.rentalRepo.ListByTool(ctx, rt.ToolID, 0, []string{string(domain.RentalStatusApproved)}, 1, 100)
	if err != nil {
		// Log error but don't fail?
		approved = []domain.Rental{}
	}
	pending, _, err := s.rentalRepo.ListByTool(ctx, rt.ToolID, 0, []string{string(domain.RentalStatusPending)}, 1, 100)
	if err != nil {
		pending = []domain.Rental{}
	}

	return rt, approved, pending, nil
}

func (s *rentalService) ActivateRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.Status != domain.RentalStatusScheduled {
		return nil, errors.New("rental is not in scheduled status")
	}
	if rt.RenterID != userID && rt.OwnerID != userID {
		return nil, errors.New("unauthorized")
	}

	rt.Status = domain.RentalStatusActive
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify other party
	var otherID int32
	var otherEmail, otherName, myName string
	user, _ := s.userRepo.GetByID(ctx, userID)
	if user != nil {
		myName = user.Name
	}

	if userID == rt.RenterID {
		otherID = rt.OwnerID
		otherUser, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
		if otherUser != nil {
			otherEmail = otherUser.Email
			otherName = otherUser.Name
		}
	} else {
		otherID = rt.RenterID
		otherUser, _ := s.userRepo.GetByID(ctx, rt.RenterID)
		if otherUser != nil {
			otherEmail = otherUser.Email
			otherName = otherUser.Name
		}
	}

	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)
	toolName := "Unknown Tool"
	if tool != nil {
		toolName = tool.Name
	}

	if otherEmail != "" {
		_ = s.emailSvc.SendRentalPickupNotification(ctx, otherEmail, otherName, toolName, rt.StartDate, rt.EndDate)

		notif := &domain.Notification{
			UserID:  otherID,
			OrgID:   rt.OrgID,
			Title:   "Rental Picked Up",
			Message: fmt.Sprintf("Rental for %s has been picked up by %s", toolName, myName),
			Attributes: map[string]string{
				"type":      "RENTAL_PICKUP",
				"rental_id": fmt.Sprintf("%d", rt.ID),
			},
		}
		_ = s.noteRepo.Create(ctx, notif)
	}

	return rt, nil
}

func (s *rentalService) ChangeRentalDates(ctx context.Context, userID, rentalID int32, newStart, newEnd, oldStart, oldEnd string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	// Determine role
	var isRenter, isOwner bool
	if userID == rt.RenterID {
		isRenter = true
	} else if userID == rt.OwnerID {
		isOwner = true
	} else {
		return nil, errors.New("unauthorized")
	}

	tool, err := s.toolRepo.GetByID(ctx, rt.ToolID)
	if err != nil {
		return nil, err
	}

	// Parse dates
	var nStart, nEnd time.Time
	// var oStart, oEnd time.Time // Unused for now
	if newStart != "" {
		nStart, _ = time.Parse("2006-01-02", newStart)
	} else {
		nStart, _ = time.Parse("2006-01-02", rt.StartDate)
	}
	if newEnd != "" {
		nEnd, _ = time.Parse("2006-01-02", newEnd)
	} else {
		nEnd, _ = time.Parse("2006-01-02", rt.EndDate)
	}
	// Verify old dates match (optimistic locking check) - skipping for simplicity as per requirement focus

	// Calculate new cost using tiered pricing algorithm
	newCost, err := utils.CalculateRentalCost(nStart, nEnd, tool)
	if err != nil {
		return nil, err
	}

	// Logic Branching
	if rt.Status == domain.RentalStatusPending || rt.Status == domain.RentalStatusApproved || rt.Status == domain.RentalStatusScheduled {
		// Pre-active changes
		rt.StartDate = nStart.Format("2006-01-02")
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost

		if isRenter {
			rt.Status = domain.RentalStatusPending
			// Notify Owner
			owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
			if owner != nil {
				// Send email/notif about date change requiring approval
				// Simplified notification dispatch
				notif := &domain.Notification{
					UserID:     owner.ID,
					OrgID:      rt.OrgID,
					Title:      "Rental Dates Changed",
					Message:    fmt.Sprintf("Renter changed dates for %s. Please re-approve.", tool.Name),
					Attributes: map[string]string{"type": "RENTAL_DATE_CHANGE", "rental_id": fmt.Sprintf("%d", rt.ID)},
				}
				s.noteRepo.Create(ctx, notif)
			}
		} else if isOwner {
			rt.Status = domain.RentalStatusApproved
			// Notify Renter
			renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
			if renter != nil {
				// Send email/notif about date change
				notif := &domain.Notification{
					UserID:     renter.ID,
					OrgID:      rt.OrgID,
					Title:      "Rental Dates Updated",
					Message:    fmt.Sprintf("Owner updated dates for %s. Please confirm.", tool.Name),
					Attributes: map[string]string{"type": "RENTAL_DATE_CHANGE", "rental_id": fmt.Sprintf("%d", rt.ID)},
				}
				s.noteRepo.Create(ctx, notif)
			}
		}
	} else if (rt.Status == domain.RentalStatusActive || rt.Status == domain.RentalStatusOverdue) && isRenter {
		// Extension request
		if newStart != "" && nStart.Format("2006-01-02") != rt.StartDate {
			return nil, errors.New("cannot change start date of active rental")
		}

		// Store the requested new end date in the end_date field
		// The last_agreed_end_date stores the original agreed date for potential rollback
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost
		rt.Status = domain.RentalStatusReturnDateChanged

		// Notify Owner
		owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
		if owner != nil {
			notif := &domain.Notification{
				UserID:     owner.ID,
				OrgID:      rt.OrgID,
				Title:      "Return Date Extension Request",
				Message:    fmt.Sprintf("Renter requests to extend return date for %s to %s.", tool.Name, nEnd.Format("2006-01-02")),
				Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_REQUEST", "rental_id": fmt.Sprintf("%d", rt.ID)},
			}
			s.noteRepo.Create(ctx, notif)
		}
	} else if rt.Status == domain.RentalStatusReturnDateChanged && isRenter {
		// Renter is updating their pending extension request
		if newStart != "" && nStart.Format("2006-01-02") != rt.StartDate {
			return nil, errors.New("cannot change start date of active rental")
		}

		// Update the requested new end date in the end_date field
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost
		// Status remains RETURN_DATE_CHANGED

		// Notify Owner about the updated request
		owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
		if owner != nil {
			notif := &domain.Notification{
				UserID:     owner.ID,
				OrgID:      rt.OrgID,
				Title:      "Extension Request Updated",
				Message:    fmt.Sprintf("Renter updated their extension request for %s to %s.", tool.Name, nEnd.Format("2006-01-02")),
				Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_REQUEST_UPDATED", "rental_id": fmt.Sprintf("%d", rt.ID)},
			}
			s.noteRepo.Create(ctx, notif)
		}
	} else {
		return nil, errors.New("cannot change dates in current status and role")
	}

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func (s *rentalService) ApproveReturnDateChange(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusReturnDateChanged {
		return nil, errors.New("invalid status")
	}

	// Copy EndDate to LastAgreedEndDate to save the new agreed date
	rt.LastAgreedEndDate = &rt.EndDate
	rt.Status = domain.RentalStatusActive

	// Check overdue?
	endDate, _ := time.Parse("2006-01-02", rt.EndDate)
	if time.Now().After(endDate) {
		rt.Status = domain.RentalStatusOverdue
	}

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify Renter
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	if renter != nil && tool != nil {
		notif := &domain.Notification{
			UserID: renter.ID, OrgID: rt.OrgID, Title: "Extension Approved",
			Message:    fmt.Sprintf("Extension for %s approved.", tool.Name),
			Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_APPROVED", "rental_id": fmt.Sprintf("%d", rt.ID)},
		}
		s.noteRepo.Create(ctx, notif)
	}
	return rt, nil
}

func (s *rentalService) RejectReturnDateChange(ctx context.Context, ownerID, rentalID int32, reason, newEndDateStr string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusReturnDateChanged {
		return nil, errors.New("invalid status")
	}

	// Validate new_end_date is provided (mandatory)
	if newEndDateStr == "" {
		return nil, errors.New("new end date is required")
	}

	// Parse and validate date format
	newEndDate, err := time.Parse("2006-01-02", newEndDateStr)
	if err != nil {
		return nil, errors.New("invalid date format, expected YYYY-MM-DD")
	}

	// Validate new_end_date is different from requested date
	if newEndDate.Format("2006-01-02") == rt.EndDate {
		return nil, errors.New("new end date must be different from the requested date")
	}

	// Get tool for cost recalculation
	tool, err := s.toolRepo.GetByID(ctx, rt.ToolID)
	if err != nil {
		return nil, err
	}

	// Update status and rejection reason
	rt.Status = domain.RentalStatusReturnDateChangeRejected
	rt.RejectionReason = reason

	// Update end_date with owner's counter-proposal
	rt.EndDate = newEndDate.Format("2006-01-02")

	// Recalculate total_cost_cents based on new end date using tiered pricing
	startDate, _ := time.Parse("2006-01-02", rt.StartDate)
	newCost, err := utils.CalculateRentalCost(startDate, newEndDate, tool)
	if err != nil {
		return nil, err
	}
	rt.TotalCostCents = newCost

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify Renter with counter-proposal details
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	if renter != nil && tool != nil {
		notif := &domain.Notification{
			UserID: renter.ID, OrgID: rt.OrgID,
			Title: "Extension Rejected - Counter-Proposal",
			Message: fmt.Sprintf("Extension for %s rejected. Owner set new return date: %s. Reason: %s. Updated cost: $%.2f",
				tool.Name, newEndDate.Format("2006-01-02"), reason, float64(rt.TotalCostCents)/100),
			Attributes: map[string]string{
				"type":             "RETURN_DATE_CHANGE_REJECTED",
				"rental_id":        fmt.Sprintf("%d", rt.ID),
				"new_end_date":     newEndDate.Format("2006-01-02"),
				"total_cost_cents": fmt.Sprintf("%d", rt.TotalCostCents),
			},
		}
		s.noteRepo.Create(ctx, notif)

		// Send email notification to renter
		_ = s.emailSvc.SendReturnDateRejectionNotification(ctx, renter.Email, tool.Name, newEndDate.Format("2006-01-02"), reason, rt.TotalCostCents)
	}
	return rt, nil
}

func (s *rentalService) AcknowledgeReturnDateRejection(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != renterID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusReturnDateChangeRejected {
		return nil, errors.New("invalid status")
	}

	// Get tool for cost recalculation
	tool, err := s.toolRepo.GetByID(ctx, rt.ToolID)
	if err != nil {
		return nil, err
	}

	// Rollback: Copy LastAgreedEndDate back to EndDate
	if rt.LastAgreedEndDate != nil {
		rt.EndDate = *rt.LastAgreedEndDate
		// Recalculate cost using the last agreed end date
		startDate, _ := time.Parse("2006-01-02", rt.StartDate)
		endDate, _ := time.Parse("2006-01-02", rt.EndDate)
		originalCost, err := utils.CalculateRentalCost(startDate, endDate, tool)
		if err != nil {
			return nil, err
		}
		rt.TotalCostCents = originalCost
	}

	rt.RejectionReason = ""

	endDate, _ := time.Parse("2006-01-02", rt.EndDate)
	if time.Now().After(endDate) {
		rt.Status = domain.RentalStatusOverdue
	} else {
		rt.Status = domain.RentalStatusActive
	}

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify Owner
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
	if owner != nil {
		notif := &domain.Notification{
			UserID: owner.ID, OrgID: rt.OrgID, Title: "Rejection Acknowledged",
			Message:    "Renter acknowledged extension rejection.",
			Attributes: map[string]string{"type": "RETURN_DATE_REJECTION_ACKNOWLEDGED", "rental_id": fmt.Sprintf("%d", rt.ID)},
		}
		s.noteRepo.Create(ctx, notif)
	}
	return rt, nil
}

func (s *rentalService) CancelReturnDateChange(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != renterID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusReturnDateChanged {
		return nil, errors.New("invalid status")
	}

	// Get tool for cost recalculation
	tool, err := s.toolRepo.GetByID(ctx, rt.ToolID)
	if err != nil {
		return nil, err
	}

	// Rollback: Copy LastAgreedEndDate back to EndDate
	if rt.LastAgreedEndDate != nil {
		rt.EndDate = *rt.LastAgreedEndDate
		// Recalculate cost using the last agreed end date
		startDate, _ := time.Parse("2006-01-02", rt.StartDate)
		endDate, _ := time.Parse("2006-01-02", rt.EndDate)
		originalCost, err := utils.CalculateRentalCost(startDate, endDate, tool)
		if err != nil {
			return nil, err
		}
		rt.TotalCostCents = originalCost
	}

	endDate, _ := time.Parse("2006-01-02", rt.EndDate)
	if time.Now().After(endDate) {
		rt.Status = domain.RentalStatusOverdue
	} else {
		rt.Status = domain.RentalStatusActive
	}

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Notify Owner
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
	if owner != nil {
		notif := &domain.Notification{
			UserID: owner.ID, OrgID: rt.OrgID, Title: "Extension Request Cancelled",
			Message:    "Renter cancelled extension request.",
			Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_CANCELLED", "rental_id": fmt.Sprintf("%d", rt.ID)},
		}
		s.noteRepo.Create(ctx, notif)
	}
	return rt, nil
}

func (s *rentalService) ListToolRentals(ctx context.Context, ownerID, toolID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	// Verify ownership
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, 0, err
	}
	if tool.OwnerID != ownerID {
		return nil, 0, errors.New("unauthorized")
	}

	return s.rentalRepo.ListByTool(ctx, toolID, orgID, statuses, page, pageSize)
}

func (s *rentalService) Update(ctx context.Context, rt *domain.Rental) error {
	return s.rentalRepo.Update(ctx, rt)
}

func (s *rentalService) CompleteRental(ctx context.Context, ownerID, rentalID int32, returnCondition string, surchargeOrCreditCents int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != ownerID {
		return nil, errors.New("unauthorized")
	}

	// Set return condition and surcharge/credit
	rt.ReturnCondition = returnCondition
	rt.SurchargeOrCreditCents = surchargeOrCreditCents

	// Calculate the full settlement amount (base rental + surcharge or - credit)
	settlementAmount := rt.TotalCostCents + surchargeOrCreditCents

	// In double-entry bookkeeping, credit and debit must run in pairs
	// All transactions happen at rental completion (not at finalize)

	// Credit owner with full settlement amount
	ownerCredit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.OwnerID,
		Amount:          settlementAmount,
		Type:            domain.TransactionTypeLendingCredit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Earnings from rental of tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, ownerCredit); err != nil {
		return nil, err
	}

	// Debit renter with full settlement amount (paired transaction)
	renterDebit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.RenterID,
		Amount:          -settlementAmount,
		Type:            domain.TransactionTypeLendingDebit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Settlement for rental of tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, renterDebit); err != nil {
		return nil, err
	}

	now := time.Now()
	rt.EndDate = now.Format("2006-01-02")
	rt.Status = domain.RentalStatusCompleted
	rt.CompletedBy = &ownerID
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Update tool status check logic
	// Simplified: Set Available.
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)
	if tool != nil {
		tool.Status = domain.ToolStatusAvailable
		_ = s.toolRepo.Update(ctx, tool)
	}

	// Notify both
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	owner, _ := s.userRepo.GetByID(ctx, ownerID)

	if renter != nil && owner != nil && tool != nil {
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, owner.Email, "Owner", tool.Name, settlementAmount)
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, renter.Email, "Renter", tool.Name, settlementAmount)

		// Notifications...
	}

	return rt, nil
}

func (s *rentalService) ListRentals(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByRenter(ctx, userID, orgID, statuses, page, pageSize)
}

func (s *rentalService) ListLendings(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error) {
	return s.rentalRepo.ListByOwner(ctx, userID, orgID, statuses, page, pageSize)
}

func (s *rentalService) GetRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.RenterID != userID && rt.OwnerID != userID {
		return nil, errors.New("unauthorized")
	}
	return rt, nil
}
