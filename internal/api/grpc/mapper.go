package grpc

import (
	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
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
		CreatedOn: u.CreatedOn.Format("2006-01-02"),
	}
}

// MapDomainOrgToProto converts domain Organization to protobuf Organization
// userRole is optional - pass empty string to leave user_role field empty
func MapDomainOrgToProto(o *domain.Organization, userRole string) *pb.Organization {
	if o == nil {
		return nil
	}
	return &pb.Organization{
		Id:          o.ID,
		Name:        o.Name,
		Description: o.Description,
		Address:     o.Address,
		Metro:       o.Metro,
		AdminEmail:  o.AdminEmail,
		AdminPhone:  o.AdminPhoneNumber,
		CreatedOn:   o.CreatedOn.Format("2006-01-02"),
		UserRole:    userRole,
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
		PricePerDayCents:     t.PricePerDayCents,
		PricePerWeekCents:    t.PricePerWeekCents,
		PricePerMonthCents:   t.PricePerMonthCents,
		ReplacementCostCents: t.ReplacementCostCents,
		Condition:            MapDomainToolConditionToProto(t.Condition),
		Owner:                MapDomainUserToProto(t.Owner),
		Metro:                t.Metro,
		Status:               MapDomainToolStatusToProto(t.Status),
		CreatedOn:            t.CreatedOn.Format("2006-01-02"),
		UpdatedOn:            t.CreatedOn.Format("2006-01-02"),
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
	return MapDomainRentalToProtoWithNames(r, "", "", "")
}

func MapDomainRentalToProtoWithNames(r *domain.Rental, renterName, ownerName, toolName string) *pb.RentalRequest {
	if r == nil {
		return nil
	}
	proto := &pb.RentalRequest{
		Id:                 r.ID,
		OrganizationId:     r.OrgID,
		ToolId:             r.ToolID,
		ToolName:           toolName,
		RenterId:           r.RenterID,
		RenterName:         renterName,
		OwnerId:            r.OwnerID,
		OwnerName:          ownerName,
		StartDate:          r.StartDate.Format("2006-01-02"),
		EndDate:            r.EndDate.Format("2006-01-02"),
		TotalCostCents:     r.TotalCostCents,
		Status:             MapDomainRentalStatusToProto(r.Status),
		PickupInstructions: r.PickupNote,
		CreatedOn:          r.CreatedOn.Format("2006-01-02"),
		UpdatedOn:          r.UpdatedOn.Format("2006-01-02"),
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
		ChargedOn:      t.ChargedOn.Format("2006-01-02"),
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
		CreatedOn:      n.CreatedOn.Format("2006-01-02"),
	}
}

func MapDomainMemberProfileToProto(u domain.User, uo domain.UserOrg) *pb.MemberProfile {
	proto := &pb.MemberProfile{
		UserId:      u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Balance:     uo.BalanceCents,
		MemberSince: uo.JoinedOn.Format("2006-01-02"),
		IsBlocked:   uo.Status == domain.UserOrgStatusBlock,
		BlockReason: uo.BlockReason,
	}
	if uo.BlockedDate != nil {
		proto.BlockedDate = uo.BlockedDate.Format("2006-01-02")
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
		RequestDate: jr.CreatedOn.Format("2006-01-02"),
	}
	if jr.UserID != nil {
		proto.UserId = *jr.UserID
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
