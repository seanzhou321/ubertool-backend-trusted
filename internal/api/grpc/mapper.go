package grpc

import (
	"context"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
)

func MapDomainUserToProto(u *domain.User) *pb.User {
	if u == nil {
		return nil
	}

	// Map organizations if present
	var protoOrgs []*pb.Organization
	if u.Orgs != nil {
		protoOrgs = make([]*pb.Organization, len(u.Orgs))
		for i, org := range u.Orgs {
			protoOrgs[i] = MapDomainOrgToProto(&org, "")
		}
	}

	return &pb.User{
		Id:        u.ID,
		Email:     u.Email,
		Phone:     u.PhoneNumber,
		Name:      u.Name,
		AvatarUrl: u.AvatarURL,
		Orgs:      protoOrgs,
		CreatedOn: u.CreatedOn,
	}
}

// MapDomainOrgToProto converts domain Organization to protobuf Organization
// userRole is optional - pass empty string to leave user_role field empty
func MapDomainOrgToProto(o *domain.Organization, userRole string) *pb.Organization {
	if o == nil {
		return nil
	}

	// Map admins if present
	var protoAdmins []*pb.User
	if o.Admins != nil {
		protoAdmins = make([]*pb.User, len(o.Admins))
		for i, admin := range o.Admins {
			protoAdmins[i] = MapDomainUserToProto(&admin)
		}
	}

	return &pb.Organization{
		Id:          o.ID,
		Name:        o.Name,
		Description: o.Description,
		Address:     o.Address,
		Metro:       o.Metro,
		MemberCount: o.MemberCount,
		AdminEmail:  o.AdminEmail,
		AdminPhone:  o.AdminPhoneNumber,
		CreatedOn:   o.CreatedOn,
		UserRole:    userRole,
		Admins:      protoAdmins,
	}
}

func MapDomainToolToProto(t *domain.Tool) *pb.Tool {
	if t == nil {
		return nil
	}
	return &pb.Tool{
		Id:                   t.ID,
		Name:                 t.Name,
		Description:          t.Description,
		Categories:           t.Categories,
		DurationUnit:         string(t.DurationUnit),
		PricePerDayCents:     t.PricePerDayCents,
		PricePerWeekCents:    t.PricePerWeekCents,
		PricePerMonthCents:   t.PricePerMonthCents,
		ReplacementCostCents: t.ReplacementCostCents,
		Condition:            MapDomainToolConditionToProto(t.Condition),
		Owner:                MapDomainUserToProto(t.Owner),
		Metro:                t.Metro,
		Status:               MapDomainToolStatusToProto(t.Status),
		CreatedOn:            t.CreatedOn,
		UpdatedOn:            t.CreatedOn,
	}
}

func MapProtoToolConditionToDomain(c pb.ToolCondition) domain.ToolCondition {
	switch c {
	case pb.ToolCondition_TOOL_CONDITION_EXCELLENT:
		return domain.ToolConditionExcellent
	case pb.ToolCondition_TOOL_CONDITION_GOOD:
		return domain.ToolConditionGood
	case pb.ToolCondition_TOOL_CONDITION_ACCEPTABLE:
		return domain.ToolConditionAcceptable
	case pb.ToolCondition_TOOL_CONDITION_DAMAGED__NEEDS_REPAIR:
		return domain.ToolConditionDamaged
	default:
		return domain.ToolConditionExcellent // Default to excellent if unspecified
	}
}

func MapDomainToolConditionToProto(c domain.ToolCondition) pb.ToolCondition {
	switch c {
	case domain.ToolConditionExcellent:
		return pb.ToolCondition_TOOL_CONDITION_EXCELLENT
	case domain.ToolConditionGood:
		return pb.ToolCondition_TOOL_CONDITION_GOOD
	case domain.ToolConditionAcceptable:
		return pb.ToolCondition_TOOL_CONDITION_ACCEPTABLE
	case domain.ToolConditionDamaged:
		return pb.ToolCondition_TOOL_CONDITION_DAMAGED__NEEDS_REPAIR
	default:
		return pb.ToolCondition_TOOL_CONDITION_UNSPECIFIED
	}
}

func MapDomainToolStatusToProto(s domain.ToolStatus) pb.ToolStatus {
	switch s {
	case domain.ToolStatusAvailable:
		return pb.ToolStatus_TOOL_STATUS_AVAILABLE
	case domain.ToolStatusUnavailable:
		return pb.ToolStatus_TOOL_STATUS_UNAVAILABLE
	case domain.ToolStatusRented:
		return pb.ToolStatus_TOOL_STATUS_RENTED
	default:
		return pb.ToolStatus_TOOL_STATUS_UNSPECIFIED
	}
}

func MapDomainRentalToProto(r *domain.Rental) *pb.RentalRequest {
	return MapDomainRentalToProtoWithNames(r, "", "", "", "", "")
}

func MapDomainRentalToProtoWithNames(r *domain.Rental, renterName, ownerName, toolName, orgName, toolCondition string) *pb.RentalRequest {
	if r == nil {
		return nil
	}
	proto := &pb.RentalRequest{
		Id:                     r.ID,
		OrganizationId:         r.OrgID,
		ToolId:                 r.ToolID,
		ToolName:               toolName,
		RenterId:               r.RenterID,
		RenterName:             renterName,
		OwnerId:                r.OwnerID,
		OwnerName:              ownerName,
		StartDate:              r.StartDate,
		EndDate:                r.EndDate,
		TotalCostCents:         r.TotalCostCents,
		Status:                 MapDomainRentalStatusToProto(r.Status),
		PickupInstructions:     r.PickupNote,
		ReturnCondition:        r.ReturnCondition,
		SurchargeOrCreditCents: r.SurchargeOrCreditCents,
		CreatedOn:              r.CreatedOn,
		UpdatedOn:              r.UpdatedOn,
		DurationUnit:           r.DurationUnit,
		DailyPriceCents:        r.DailyPriceCents,
		WeeklyPriceCents:       r.WeeklyPriceCents,
		MonthlyPriceCents:      r.MonthlyPriceCents,
		ReplacementCostCents:   r.ReplacementCostCents,
		OrganizationName:       orgName,
		ToolCondition:          toolCondition,
	}
	return proto
}

func MapDomainRentalStatusToProto(s domain.RentalStatus) pb.RentalStatus {
	switch s {
	case domain.RentalStatusPending:
		return pb.RentalStatus_RENTAL_STATUS_PENDING
	case domain.RentalStatusApproved:
		return pb.RentalStatus_RENTAL_STATUS_APPROVED
	case domain.RentalStatusRejected:
		return pb.RentalStatus_RENTAL_STATUS_REJECTED
	case domain.RentalStatusScheduled:
		return pb.RentalStatus_RENTAL_STATUS_SCHEDULED
	case domain.RentalStatusActive:
		return pb.RentalStatus_RENTAL_STATUS_ACTIVE
	case domain.RentalStatusCompleted:
		return pb.RentalStatus_RENTAL_STATUS_COMPLETED
	case domain.RentalStatusCancelled:
		return pb.RentalStatus_RENTAL_STATUS_CANCELLED
	case domain.RentalStatusOverdue:
		return pb.RentalStatus_RENTAL_STATUS_OVERDUE
	case domain.RentalStatusReturnDateChanged:
		return pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGED
	case domain.RentalStatusReturnDateChangeRejected:
		return pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGE_REJECTED
	default:
		return pb.RentalStatus_RENTAL_STATUS_UNSPECIFIED
	}
}

func MapProtoRentalStatusToDomain(s pb.RentalStatus) string {
	switch s {
	case pb.RentalStatus_RENTAL_STATUS_PENDING:
		return string(domain.RentalStatusPending)
	case pb.RentalStatus_RENTAL_STATUS_APPROVED:
		return string(domain.RentalStatusApproved)
	case pb.RentalStatus_RENTAL_STATUS_REJECTED:
		return string(domain.RentalStatusRejected)
	case pb.RentalStatus_RENTAL_STATUS_SCHEDULED:
		return string(domain.RentalStatusScheduled)
	case pb.RentalStatus_RENTAL_STATUS_ACTIVE:
		return string(domain.RentalStatusActive)
	case pb.RentalStatus_RENTAL_STATUS_COMPLETED:
		return string(domain.RentalStatusCompleted)
	case pb.RentalStatus_RENTAL_STATUS_CANCELLED:
		return string(domain.RentalStatusCancelled)
	case pb.RentalStatus_RENTAL_STATUS_OVERDUE:
		return string(domain.RentalStatusOverdue)
	case pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGED:
		return string(domain.RentalStatusReturnDateChanged)
	case pb.RentalStatus_RENTAL_STATUS_RETURN_DATE_CHANGE_REJECTED:
		return string(domain.RentalStatusReturnDateChangeRejected)
	default:
		return ""
	}
}

func MapProtoRentalStatusesToDomain(statuses []pb.RentalStatus) []string {
	if len(statuses) == 0 {
		return nil
	}
	result := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if statusStr := MapProtoRentalStatusToDomain(s); statusStr != "" {
			result = append(result, statusStr)
		}
	}
	return result
}

func MapDomainTransactionToProto(t *domain.LedgerTransaction) *pb.Transaction {
	if t == nil {
		return nil
	}
	proto := &pb.Transaction{
		Id:             t.ID,
		OrganizationId: t.OrgID,
		UserId:         t.UserID,
		Amount:         t.Amount,
		Type:           MapDomainTransactionTypeToProto(t.Type),
		Description:    t.Description,
		ChargedOn:      t.ChargedOn,
	}
	if t.RelatedRentalID != nil {
		// proto.RelatedRentalId = *t.RelatedRentalID // Error: proto expects Object, domain has ID
	}
	return proto
}

func MapDomainTransactionTypeToProto(t domain.TransactionType) pb.TransactionType {
	switch t {
	case domain.TransactionTypeRentalDebit:
		return pb.TransactionType_TRANSACTION_TYPE_RENTAL_DEBIT
	case domain.TransactionTypeLendingCredit:
		return pb.TransactionType_TRANSACTION_TYPE_LENDING_CREDIT
	case domain.TransactionTypeLendingDebit:
		return pb.TransactionType_TRANSACTION_TYPE_LENDING_DEBIT
	case domain.TransactionTypeRefund:
		return pb.TransactionType_TRANSACTION_TYPE_REFUND
	case domain.TransactionTypeAdjustment:
		return pb.TransactionType_TRANSACTION_TYPE_ADJUSTMENT
	default:
		return pb.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func MapDomainNotificationToProto(n *domain.Notification) *pb.Notification {
	if n == nil {
		return nil
	}
	return &pb.Notification{
		Id:             n.ID,
		UserId:         n.UserID,
		OrganizationId: n.OrgID,
		Title:          n.Title,
		Message:        n.Message,
		Read:           n.IsRead,
		Attributes:     n.Attributes,
		CreatedOn:      n.CreatedOn,
	}
}

func MapDomainMemberProfileToProto(u domain.User, uo domain.UserOrg) *pb.MemberProfile {
	proto := &pb.MemberProfile{
		UserId:         u.ID,
		Name:           u.Name,
		Email:          u.Email,
		BalanceCents:   uo.BalanceCents,
		MemberSince:    uo.JoinedOn,
		IsBlocked:      uo.RentingBlocked || uo.LendingBlocked,
		BlockReason:    uo.BlockedReason,
		Phone:          u.PhoneNumber,
		Role:           string(uo.Role),
		AvatarUrl:      u.AvatarURL,
		RentingBlocked: uo.RentingBlocked,
		LendingBlocked: uo.LendingBlocked,
		Status:         string(uo.Status),
	}
	if uo.BlockedOn != nil {
		proto.BlockedOn = *uo.BlockedOn
	}
	return proto
}

func MapDomainJoinRequestProfileToProto(jr *domain.JoinRequest) *pb.JoinRequestProfile {
	if jr == nil {
		return nil
	}
	proto := &pb.JoinRequestProfile{
		RequestId:   jr.ID,
		Name:        jr.Name,
		Email:       jr.Email,
		Message:     jr.Note,
		RequestDate: jr.CreatedOn,
		Status:      string(jr.Status),
		Reason:      jr.Reason,
		RejectedBy:  jr.RejectedBy,
	}
	if jr.UserID != nil {
		proto.UserId = *jr.UserID
	}
	if jr.UsedOn != nil {
		proto.UsedOn = *jr.UsedOn
	}
	return proto
}

func MapDomainLedgerSummaryToProto(s *domain.LedgerSummary) *pb.GetLedgerSummaryResponse {
	if s == nil {
		return &pb.GetLedgerSummaryResponse{}
	}
	return &pb.GetLedgerSummaryResponse{
		Balance:     s.Balance,
		StatusCount: s.StatusCount,
	}
}

func MapDomainToolImageToProto(t *domain.ToolImage) *pb.ToolImage {
	if t == nil {
		return nil
	}

	var createdOn, confirmedOn string
	if !t.CreatedOn.IsZero() {
		createdOn = t.CreatedOn.Format("2006-01-02T15:04:05Z")
	}
	if t.ConfirmedOn != nil && !t.ConfirmedOn.IsZero() {
		confirmedOn = t.ConfirmedOn.Format("2006-01-02T15:04:05Z")
	}

	return &pb.ToolImage{
		Id:            t.ID,
		ToolId:        t.ToolID,
		FileName:      t.FileName,
		FilePath:      t.FilePath,
		ThumbnailPath: t.ThumbnailPath,
		FileSize:      t.FileSize,
		IsPrimary:     t.IsPrimary,
		DisplayOrder:  t.DisplayOrder,
		Status:        t.Status,
		CreatedOn:     createdOn,
		ConfirmedOn:   confirmedOn,
	}
}

// Bill Split Mappers

func MapDomainBillToPaymentItem(ctx context.Context, bill *domain.Bill, userID int32, userSvc service.UserService) (*pb.PaymentItem, error) {
	if bill == nil {
		return nil, nil
	}

	// Get debtor and creditor names
	debtor, _, _, err := userSvc.GetUserProfile(ctx, bill.DebtorUserID)
	if err != nil {
		return nil, err
	}
	creditor, _, _, err := userSvc.GetUserProfile(ctx, bill.CreditorUserID)
	if err != nil {
		return nil, err
	}

	debtorName := ""
	creditorName := ""
	if debtor != nil {
		debtorName = debtor.Name
	}
	if creditor != nil {
		creditorName = creditor.Name
	}

	category := MapDomainPaymentCategoryToProto(bill.GetPaymentCategory(userID))

	payment := &pb.PaymentItem{
		PaymentId:         bill.ID,
		DebtorId:          bill.DebtorUserID,
		DebtorName:        debtorName,
		CreditorId:        bill.CreditorUserID,
		CreditorName:      creditorName,
		AmountCents:       bill.AmountCents,
		SettlementMonth:   bill.SettlementMonth,
		Status:            string(bill.Status),
		Category:          category,
		DisputeReason:     bill.DisputeReason,
		ResolutionOutcome: bill.ResolutionOutcome,
		ResolutionNotes:   bill.ResolutionNotes,
		CreatedAt:         timeToEpochMillis(bill.CreatedAt),
	}

	if bill.NoticeSentAt != nil {
		payment.NoticeSentAt = timeToEpochMillis(*bill.NoticeSentAt)
	}
	if bill.DebtorAcknowledgedAt != nil {
		payment.DebtorAcknowledgedAt = timeToEpochMillis(*bill.DebtorAcknowledgedAt)
	}
	if bill.CreditorAcknowledgedAt != nil {
		payment.CreditorAcknowledgedAt = timeToEpochMillis(*bill.CreditorAcknowledgedAt)
	}
	if bill.DisputedAt != nil {
		payment.DisputedAt = timeToEpochMillis(*bill.DisputedAt)
	}
	if bill.ResolvedAt != nil {
		payment.ResolvedAt = timeToEpochMillis(*bill.ResolvedAt)
	}

	return payment, nil
}

func MapDomainBillActionToProto(ctx context.Context, action *domain.BillAction, userSvc service.UserService) (*pb.PaymentAction, error) {
	if action == nil {
		return nil, nil
	}

	actorName := "System"
	if action.ActorUserID != nil {
		actor, _, _, err := userSvc.GetUserProfile(ctx, *action.ActorUserID)
		if err == nil && actor != nil {
			actorName = actor.Name
		}
	}

	actorUserID := int32(0)
	if action.ActorUserID != nil {
		actorUserID = *action.ActorUserID
	}

	return &pb.PaymentAction{
		ActorUserId:       actorUserID,
		ActorName:         actorName,
		ActionType:        string(action.ActionType),
		Notes:             action.Notes,
		ActionDetailsJson: action.ActionDetails,
		CreatedAt:         timeToEpochMillis(action.CreatedAt),
	}, nil
}

func MapDomainBillToDisputedPaymentItem(ctx context.Context, bill *domain.Bill, userSvc service.UserService) (*pb.DisputedPaymentItem, error) {
	if bill == nil {
		return nil, nil
	}

	// Get debtor and creditor names
	debtor, _, _, err := userSvc.GetUserProfile(ctx, bill.DebtorUserID)
	if err != nil {
		return nil, err
	}
	creditor, _, _, err := userSvc.GetUserProfile(ctx, bill.CreditorUserID)
	if err != nil {
		return nil, err
	}

	debtorName := ""
	creditorName := ""
	if debtor != nil {
		debtorName = debtor.Name
	}
	if creditor != nil {
		creditorName = creditor.Name
	}

	item := &pb.DisputedPaymentItem{
		PaymentId:    bill.ID,
		DebtorId:     bill.DebtorUserID,
		DebtorName:   debtorName,
		CreditorId:   bill.CreditorUserID,
		CreditorName: creditorName,
		AmountCents:  bill.AmountCents,
		Reason:       bill.DisputeReason,
		IsResolved:   bill.ResolvedAt != nil,
		Resolution:   bill.ResolutionOutcome,
	}

	if bill.DisputedAt != nil {
		item.DisputedAt = timeToEpochMillis(*bill.DisputedAt)
	}
	if bill.ResolvedAt != nil {
		item.ResolvedAt = timeToEpochMillis(*bill.ResolvedAt)
	}

	return item, nil
}

func MapDomainPaymentCategoryToProto(category string) pb.PaymentCategory {
	switch category {
	case "PAYMENT_TO_MAKE":
		return pb.PaymentCategory_PAYMENT_TO_MAKE
	case "RECEIPT_TO_VERIFY":
		return pb.PaymentCategory_RECEIPT_TO_VERIFY
	case "PAYMENT_IN_DISPUTE":
		return pb.PaymentCategory_PAYMENT_IN_DISPUTE
	case "RECEIPT_IN_DISPUTE":
		return pb.PaymentCategory_RECEIPT_IN_DISPUTE
	case "COMPLETED":
		return pb.PaymentCategory_COMPLETED
	default:
		return pb.PaymentCategory_PAYMENT_CATEGORY_UNSPECIFIED
	}
}

func MapProtoDisputeResolutionToDomain(resolution pb.DisputeResolution) string {
	switch resolution {
	case pb.DisputeResolution_DEBTOR_AT_FAULT:
		return string(domain.ResolutionOutcomeDebtorFault)
	case pb.DisputeResolution_CREDITOR_AT_FAULT:
		return string(domain.ResolutionOutcomeCreditorFault)
	case pb.DisputeResolution_BOTH_AT_FAULT:
		return string(domain.ResolutionOutcomeBothFault)
	default:
		return string(domain.ResolutionOutcomeGraceful)
	}
}

func timeToEpochMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / 1000000
}
