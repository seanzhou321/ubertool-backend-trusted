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
	noteSvc    NotificationService
}

func NewRentalService(
	rentalRepo repository.RentalRepository,
	toolRepo repository.ToolRepository,
	ledgerRepo repository.LedgerRepository,
	userRepo repository.UserRepository,
	emailSvc EmailService,
	noteSvc NotificationService,
) RentalService {
	return &rentalService{
		rentalRepo: rentalRepo,
		toolRepo:   toolRepo,
		ledgerRepo: ledgerRepo,
		userRepo:   userRepo,
		emailSvc:   emailSvc,
		noteSvc:    noteSvc,
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
	if !end.After(start) {
		return nil, errors.New("end date must be after start date (minimum 1 day rental)")
	}

	// Build price snapshot from tool at the time of rental creation
	snapshot := utils.RentalPriceSnapshot{
		DurationUnit:       tool.DurationUnit,
		PricePerDayCents:   tool.PricePerDayCents,
		PricePerWeekCents:  tool.PricePerWeekCents,
		PricePerMonthCents: tool.PricePerMonthCents,
	}

	totalCost, err := utils.CalculateRentalCost(start, end, snapshot)
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
		OrgID:                orgID,
		ToolID:               toolID,
		RenterID:             renterID,
		OwnerID:              tool.OwnerID,
		StartDate:            start.Format("2006-01-02"),
		EndDate:              end.Format("2006-01-02"),
		DurationUnit:         string(tool.DurationUnit),
		DailyPriceCents:      tool.PricePerDayCents,
		WeeklyPriceCents:     tool.PricePerWeekCents,
		MonthlyPriceCents:    tool.PricePerMonthCents,
		ReplacementCostCents: tool.ReplacementCostCents,
		TotalCostCents:       totalCost,
		Status:               domain.RentalStatusPending,
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
				"type":       "RENTAL_REQUEST",
				"rental_id":  fmt.Sprintf("%d", rental.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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
				"type":       "RENTAL_APPROVED",
				"rental_id":  fmt.Sprintf("%d", rt.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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
				"type":       "RENTAL_REJECTED",
				"rental_id":  fmt.Sprintf("%d", rt.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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
				"type":       "RENTAL_CANCELLED",
				"rental_id":  fmt.Sprintf("%d", rt.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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
				"type":       "RENTAL_CONFIRMED",
				"rental_id":  fmt.Sprintf("%d", rt.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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
				"type":       "RENTAL_PICKUP",
				"rental_id":  fmt.Sprintf("%d", rt.ID),
				"channel_id": string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
	}

	return rt, nil
}

func (s *rentalService) ChangeRentalDates(ctx context.Context, userID, rentalID int32, newStart, newEnd, oldStart, oldEnd string) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}

	isRenter, isOwner, err := s.resolveRentalRole(userID, rt)
	if err != nil {
		return nil, err
	}

	tool, err := s.toolRepo.GetByID(ctx, rt.ToolID)
	if err != nil {
		return nil, err
	}

	nStart, nEnd, err := s.resolveNewDates(newStart, newEnd, rt)
	if err != nil {
		return nil, err
	}

	newCost, err := s.calcCost(rt, nStart.Format("2006-01-02"), nEnd.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}

	if err := s.applyDateChange(ctx, rt, tool, isRenter, isOwner, newStart, nStart, nEnd, newCost); err != nil {
		return nil, err
	}

	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}
	return rt, nil
}

// resolveRentalRole returns (isRenter, isOwner) for the given user, or an error if they are not a participant.
func (s *rentalService) resolveRentalRole(userID int32, rt *domain.Rental) (isRenter, isOwner bool, err error) {
	switch userID {
	case rt.RenterID:
		return true, false, nil
	case rt.OwnerID:
		return false, true, nil
	default:
		return false, false, errors.New("unauthorized")
	}
}

// resolveNewDates normalises the requested start/end dates against the rental's current dates and
// validates that end is strictly after start.
func (s *rentalService) resolveNewDates(newStart, newEnd string, rt *domain.Rental) (nStart, nEnd time.Time, err error) {
	startStr := rt.StartDate
	if newStart != "" {
		startStr = newStart
	}
	endStr := rt.EndDate
	if newEnd != "" {
		endStr = newEnd
	}
	nStart, err = time.Parse("2006-01-02", startStr)
	if err != nil {
		return nStart, nEnd, fmt.Errorf("invalid start date: %w", err)
	}
	nEnd, err = time.Parse("2006-01-02", endStr)
	if err != nil {
		return nStart, nEnd, fmt.Errorf("invalid end date: %w", err)
	}
	if !nEnd.After(nStart) {
		return nStart, nEnd, errors.New("end date must be after start date (minimum 1 day rental)")
	}
	return nStart, nEnd, nil
}

// applyDateChange mutates rt in-place based on the transition rule that applies to the current
// status and caller role, and dispatches the appropriate notification.
func (s *rentalService) applyDateChange(ctx context.Context, rt *domain.Rental, tool *domain.Tool, isRenter, isOwner bool, newStart string, nStart, nEnd time.Time, newCost int32) error {
	rentalIDStr := fmt.Sprintf("%d", rt.ID)

	switch {
	case isPreActive(rt.Status):
		// Both renter and owner can adjust dates before the rental goes active.
		rt.StartDate = nStart.Format("2006-01-02")
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost
		return s.notifyPreActiveDateChange(ctx, rt, tool, isRenter, isOwner, rentalIDStr)

	case isRenter && isActiveOrOverdue(rt.Status):
		// Active/overdue: renter may only extend the end date (start is locked).
		if newStart != "" && nStart.Format("2006-01-02") != rt.StartDate {
			return errors.New("cannot change start date of active rental")
		}
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost
		rt.Status = domain.RentalStatusReturnDateChanged
		return s.notifyOwnerExtensionRequest(ctx, rt, tool, rentalIDStr, nEnd, "Return Date Extension Request",
			fmt.Sprintf("Renter requests to extend return date for %s to %s.", tool.Name, nEnd.Format("2006-01-02")),
			"RETURN_DATE_CHANGE_REQUEST")

	case isRenter && rt.Status == domain.RentalStatusReturnDateChanged:
		// Renter is amending their pending extension request.
		if newStart != "" && nStart.Format("2006-01-02") != rt.StartDate {
			return errors.New("cannot change start date of active rental")
		}
		rt.EndDate = nEnd.Format("2006-01-02")
		rt.TotalCostCents = newCost
		// Status stays RETURN_DATE_CHANGED.
		return s.notifyOwnerExtensionRequest(ctx, rt, tool, rentalIDStr, nEnd, "Extension Request Updated",
			fmt.Sprintf("Renter updated their extension request for %s to %s.", tool.Name, nEnd.Format("2006-01-02")),
			"RETURN_DATE_CHANGE_REQUEST_UPDATED")

	default:
		return errors.New("cannot change dates in current status and role")
	}
}

// notifyPreActiveDateChange sends the appropriate notification when dates are changed before pickup.
// A renter change resets status to PENDING; an owner change keeps it APPROVED.
func (s *rentalService) notifyPreActiveDateChange(ctx context.Context, rt *domain.Rental, tool *domain.Tool, isRenter, isOwner bool, rentalIDStr string) error {
	if isRenter {
		rt.Status = domain.RentalStatusPending
		owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
		if owner != nil {
			_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
				UserID:  owner.ID,
				OrgID:   rt.OrgID,
				Title:   "Rental Dates Changed",
				Message: fmt.Sprintf("Renter changed dates for %s. Please re-approve.", tool.Name),
				Attributes: map[string]string{
					"type": "RENTAL_DATE_CHANGE", "rental_id": rentalIDStr,
					"channel_id": string(domain.ChannelRentalRequest),
				},
			})
		}
	} else if isOwner {
		rt.Status = domain.RentalStatusApproved
		renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
		if renter != nil {
			_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
				UserID:  renter.ID,
				OrgID:   rt.OrgID,
				Title:   "Rental Dates Updated",
				Message: fmt.Sprintf("Owner updated dates for %s. Please confirm.", tool.Name),
				Attributes: map[string]string{
					"type": "RENTAL_DATE_CHANGE", "rental_id": rentalIDStr,
					"channel_id": string(domain.ChannelRentalRequest),
				},
			})
		}
	}
	return nil
}

// notifyOwnerExtensionRequest notifies the owner about a new or updated return-date extension request.
func (s *rentalService) notifyOwnerExtensionRequest(ctx context.Context, rt *domain.Rental, tool *domain.Tool, rentalIDStr string, nEnd time.Time, title, message, notifType string) error {
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
	if owner != nil {
		_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
			UserID:  owner.ID,
			OrgID:   rt.OrgID,
			Title:   title,
			Message: message,
			Attributes: map[string]string{
				"type": notifType, "rental_id": rentalIDStr,
				"channel_id": string(domain.ChannelRentalRequest),
			},
		})
	}
	return nil
}

// isPreActive reports whether the rental is in a state that precedes the tool being picked up.
func isPreActive(status domain.RentalStatus) bool {
	return status == domain.RentalStatusPending ||
		status == domain.RentalStatusApproved ||
		status == domain.RentalStatusScheduled
}

// isActiveOrOverdue reports whether the rental is currently active or overdue.
func isActiveOrOverdue(status domain.RentalStatus) bool {
	return status == domain.RentalStatusActive || status == domain.RentalStatusOverdue
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
			UserID:     renter.ID,
			OrgID:      rt.OrgID,
			Title:      "Extension Approved",
			Message:    fmt.Sprintf("Extension for %s approved.", tool.Name),
			Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_APPROVED", "rental_id": fmt.Sprintf("%d", rt.ID), "channel_id": string(domain.ChannelRentalRequest)},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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

	newCost, err := s.calcCost(rt, "", "")
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
			UserID: renter.ID,
			OrgID:  rt.OrgID,
			Title:  "Extension Rejected - Counter-Proposal",
			Message: fmt.Sprintf("Extension for %s rejected. Owner set new return date: %s. Reason: %s. Updated cost: $%.2f",
				tool.Name, newEndDate.Format("2006-01-02"), reason, float64(rt.TotalCostCents)/100),
			Attributes: map[string]string{
				"type":             "RETURN_DATE_CHANGE_REJECTED",
				"rental_id":        fmt.Sprintf("%d", rt.ID),
				"new_end_date":     newEndDate.Format("2006-01-02"),
				"total_cost_cents": fmt.Sprintf("%d", rt.TotalCostCents),
				"channel_id":       string(domain.ChannelRentalRequest),
			},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)

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

	// Rollback: restore the last agreed end date and recalculate cost from the rental's snapshot.
	if rt.LastAgreedEndDate != nil {
		rt.EndDate = *rt.LastAgreedEndDate
		originalCost, err := s.calcCost(rt, "", "")
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
			UserID:     owner.ID,
			OrgID:      rt.OrgID,
			Title:      "Rejection Acknowledged",
			Message:    "Renter acknowledged extension rejection.",
			Attributes: map[string]string{"type": "RETURN_DATE_REJECTION_ACKNOWLEDGED", "rental_id": fmt.Sprintf("%d", rt.ID), "channel_id": string(domain.ChannelRentalRequest)},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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

	// Rollback: restore the last agreed end date and recalculate cost from the rental's snapshot.
	if rt.LastAgreedEndDate != nil {
		rt.EndDate = *rt.LastAgreedEndDate
		originalCost, err := s.calcCost(rt, "", "")
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
			UserID:     owner.ID,
			OrgID:      rt.OrgID,
			Title:      "Extension Request Cancelled",
			Message:    "Renter cancelled extension request.",
			Attributes: map[string]string{"type": "RETURN_DATE_CHANGE_CANCELLED", "rental_id": fmt.Sprintf("%d", rt.ID), "channel_id": string(domain.ChannelRentalRequest)},
		}
		_ = s.noteSvc.Dispatch(ctx, notif)
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

// CompleteRental marks a rental as returned, settles balances/ledger if applicable, and
// fires off notifications and emails asynchronously so the caller gets a response immediately.
func (s *rentalService) CompleteRental(ctx context.Context, userID, rentalID int32, returnCondition string, surchargeOrCreditCents int32, notes string, chargeBillsplit bool) (*domain.Rental, error) {
	// Steps 1-2: Authorise and verify status.
	rt, err := s.loadAndValidateRental(ctx, userID, rentalID)
	if err != nil {
		return nil, err
	}

	// Steps 3-5: Compute cost from price snapshot; settlement includes any surcharge/credit.
	totalCostCents, err := s.calcCost(rt, "", "")
	if err != nil {
		return nil, err
	}
	settlementCents := totalCostCents + surchargeOrCreditCents

	// Step 6: Persist completion details.
	rt.ReturnCondition = returnCondition
	rt.SurchargeOrCreditCents = surchargeOrCreditCents
	rt.TotalCostCents = totalCostCents
	rt.Notes = notes
	rt.ChargeBillsplit = chargeBillsplit
	rt.Status = domain.RentalStatusCompleted
	rt.CompletedBy = &userID
	if err := s.rentalRepo.Update(ctx, rt); err != nil {
		return nil, err
	}

	// Steps 7, 11: Apply financial settlement (balance + ledger) — skipped when chargeBillsplit=false.
	ownerLedgerID, err := s.applyOwnerSettlement(ctx, rt, settlementCents, chargeBillsplit)
	if err != nil {
		return nil, err
	}
	renterLedgerID, err := s.applyRenterSettlement(ctx, rt, settlementCents, chargeBillsplit)
	if err != nil {
		return nil, err
	}

	// Step 15: Set tool status to AVAILABLE or RENTED based on remaining active rentals.
	tool, _ := s.toolRepo.GetByID(ctx, rt.ToolID)
	toolName := ""
	if tool != nil {
		toolName = tool.Name
		_, activeCount, _ := s.rentalRepo.ListByTool(ctx, rt.ToolID, rt.OrgID, []string{
			string(domain.RentalStatusActive), string(domain.RentalStatusScheduled),
		}, 1, 1)
		if activeCount > 0 {
			tool.Status = domain.ToolStatusRented
		} else {
			tool.Status = domain.ToolStatusAvailable
		}
		_ = s.toolRepo.Update(ctx, tool)
	}

	// Steps 8-14, 16-21: Notifications and emails are fire-and-forget.
	// A detached context is used so cancellation of the request context does not abort delivery.
	owner, _ := s.userRepo.GetByID(ctx, rt.OwnerID)
	renter, _ := s.userRepo.GetByID(ctx, rt.RenterID)
	detachedCtx := context.WithoutCancel(ctx)
	go s.dispatchSettlementNotifications(detachedCtx, rt, settlementCents, chargeBillsplit, owner, renter, ownerLedgerID, renterLedgerID, toolName)
	go s.dispatchCompletionNotifications(detachedCtx, rt, settlementCents, chargeBillsplit, owner, renter, toolName)

	return rt, nil
}

// loadAndValidateRental fetches the rental and checks that the caller is a participant and the
// rental is in a state that allows completion (steps 1-2).
func (s *rentalService) loadAndValidateRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error) {
	rt, err := s.rentalRepo.GetByID(ctx, rentalID)
	if err != nil {
		return nil, err
	}
	if rt.OwnerID != userID && rt.RenterID != userID {
		return nil, errors.New("unauthorized")
	}
	if rt.Status != domain.RentalStatusActive && rt.Status != domain.RentalStatusScheduled && rt.Status != domain.RentalStatusOverdue {
		return nil, fmt.Errorf("rental cannot be completed: status is %s", rt.Status)
	}
	return rt, nil
}

// calcCost computes the rental cost using the rental's stored price snapshot.
// Pass empty strings for startStr/endStr to fall back to the rental's own StartDate/EndDate.
func (s *rentalService) calcCost(rt *domain.Rental, startStr, endStr string) (int32, error) {
	if startStr == "" {
		startStr = rt.StartDate
	}
	if endStr == "" {
		endStr = rt.EndDate
	}
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return 0, fmt.Errorf("invalid start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return 0, fmt.Errorf("invalid end date: %w", err)
	}
	return utils.CalculateRentalCost(start, end, utils.RentalPriceSnapshot{
		DurationUnit:       domain.ToolDurationUnit(rt.DurationUnit),
		PricePerDayCents:   rt.DailyPriceCents,
		PricePerWeekCents:  rt.WeeklyPriceCents,
		PricePerMonthCents: rt.MonthlyPriceCents,
	})
}

// applyOwnerSettlement creates a LENDING_CREDIT ledger entry for the owner.
// Balance is updated automatically by the DB trigger on ledger_transactions.
// No-ops when chargeBillsplit=false (step 7). Returns the new ledger transaction ID (0 if skipped).
func (s *rentalService) applyOwnerSettlement(ctx context.Context, rt *domain.Rental, settlementCents int32, chargeBillsplit bool) (int32, error) {
	if !chargeBillsplit {
		return 0, nil
	}
	ownerCredit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.OwnerID,
		Amount:          settlementCents,
		Type:            domain.TransactionTypeLendingCredit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Earnings from rental of tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, ownerCredit); err != nil {
		return 0, err
	}
	return ownerCredit.ID, nil
}

// applyRenterSettlement creates a LENDING_DEBIT ledger entry for the renter.
// Balance is updated automatically by the DB trigger on ledger_transactions.
// No-ops when chargeBillsplit=false (step 11). Returns the new ledger transaction ID (0 if skipped).
func (s *rentalService) applyRenterSettlement(ctx context.Context, rt *domain.Rental, settlementCents int32, chargeBillsplit bool) (int32, error) {
	if !chargeBillsplit {
		return 0, nil
	}
	renterDebit := &domain.LedgerTransaction{
		OrgID:           rt.OrgID,
		UserID:          rt.RenterID,
		Amount:          -settlementCents,
		Type:            domain.TransactionTypeLendingDebit,
		RelatedRentalID: &rt.ID,
		Description:     fmt.Sprintf("Settlement for rental of tool %d", rt.ToolID),
	}
	if err := s.ledgerRepo.CreateTransaction(ctx, renterDebit); err != nil {
		return 0, err
	}
	return renterDebit.ID, nil
}

// dispatchSettlementNotifications sends credit/debit update notifications and emails to the owner
// and renter (steps 8-14). When chargeBillsplit=false the notification body includes a highlighted
// reminder that settlement should happen directly between the parties.
// Intended to be called as a goroutine.
func (s *rentalService) dispatchSettlementNotifications(ctx context.Context, rt *domain.Rental, settlementCents int32, chargeBillsplit bool, owner, renter *domain.User, ownerLedgerID, renterLedgerID int32, toolName string) {
	rentalIDStr := fmt.Sprintf("%d", rt.ID)
	settlementStr := fmt.Sprintf("%d", settlementCents)
	chargeBSStr := fmt.Sprintf("%t", chargeBillsplit)

	const ownerReminder = " Reminder: The rental payment of this transaction should be settled directly between you and the renter. This transaction is not included in the monthly billsplit."
	const renterReminder = " Reminder: The rental payment of this transaction should be settled directly between you and the owner. This transaction is not included in the monthly billsplit."

	// Steps 8-10: Owner credit/balance update notification + email + push (via Dispatch).
	ownerCreditAttrs := map[string]string{
		"topic":            "rental_credit_update",
		"amount":           settlementStr,
		"rental":           rentalIDStr,
		"charge_billsplit": chargeBSStr,
	}
	if chargeBillsplit {
		ownerCreditAttrs["transaction"] = fmt.Sprintf("%d", ownerLedgerID)
	}
	ownerCreditMsg := fmt.Sprintf("Your rental has been credited %d cents.", settlementCents)
	if !chargeBillsplit {
		ownerCreditMsg += ownerReminder
	}
	_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
		UserID:     rt.OwnerID,
		OrgID:      rt.OrgID,
		Title:      "Rental Credit Update",
		Message:    ownerCreditMsg,
		Attributes: ownerCreditAttrs,
	})
	if owner != nil {
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, owner.Email, "Owner", toolName, settlementCents)
	}

	// Steps 12-14: Renter debit/balance update notification + email + push (via Dispatch).
	renterDebitAttrs := map[string]string{
		"topic":            "rental_debit_update",
		"amount":           settlementStr,
		"rental":           rentalIDStr,
		"charge_billsplit": chargeBSStr,
	}
	if chargeBillsplit {
		renterDebitAttrs["transaction"] = fmt.Sprintf("%d", renterLedgerID)
	}
	renterDebitMsg := fmt.Sprintf("Your rental has been debited %d cents.", settlementCents)
	if !chargeBillsplit {
		renterDebitMsg += renterReminder
	}
	_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
		UserID:     rt.RenterID,
		OrgID:      rt.OrgID,
		Title:      "Rental Debit Update",
		Message:    renterDebitMsg,
		Attributes: renterDebitAttrs,
	})
	if renter != nil {
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, renter.Email, "Renter", toolName, settlementCents)
	}
}

// dispatchCompletionNotifications sends rental-completion notifications and emails to the owner
// and renter (steps 16-21). Intended to be called as a goroutine.
func (s *rentalService) dispatchCompletionNotifications(ctx context.Context, rt *domain.Rental, settlementCents int32, chargeBillsplit bool, owner, renter *domain.User, toolName string) {
	completionAttrs := map[string]string{
		"topic":            "rental_completion",
		"rental":           fmt.Sprintf("%d", rt.ID),
		"charge_billsplit": fmt.Sprintf("%t", chargeBillsplit),
	}

	// Steps 16-18: Owner completion notification + email + push (via Dispatch).
	_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
		UserID:     rt.OwnerID,
		OrgID:      rt.OrgID,
		Title:      "Rental Completed",
		Message:    "The rental has been completed and the tool status has been updated.",
		Attributes: completionAttrs,
	})
	if owner != nil {
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, owner.Email, "Owner", toolName, settlementCents)
	}

	// Steps 19-21: Renter completion notification + email + push (via Dispatch).
	_ = s.noteSvc.Dispatch(ctx, &domain.Notification{
		UserID:     rt.RenterID,
		OrgID:      rt.OrgID,
		Title:      "Rental Completed",
		Message:    "The rental has been completed.",
		Attributes: completionAttrs,
	})
	if renter != nil {
		_ = s.emailSvc.SendRentalCompletionNotification(ctx, renter.Email, "Renter", toolName, settlementCents)
	}
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
