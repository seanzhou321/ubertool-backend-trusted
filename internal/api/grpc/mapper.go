package grpc

import (
	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
)

func MapDomainUserToProto(u *domain.User) *pb.User {
	if u == nil {
		return nil
	}
	return &pb.User{
		Id:          u.ID,
		Email:       u.Email,
		Phone:       u.PhoneNumber,
		Name:        u.Name,
		AvatarUrl:   u.AvatarURL,
		CreatedOn:   u.CreatedOn.Format("2006-01-02"),
	}
}

func MapDomainOrgToProto(o *domain.Organization) *pb.Organization {
	if o == nil {
		return nil
	}
	return &pb.Organization{
		Id:               o.ID,
		Name:             o.Name,
		Description:      o.Description,
		Address:          o.Address,
		Metro:            o.Metro,
		CreatedOn:        o.CreatedOn.Format("2006-01-02"),
	}
}

func MapDomainToolToProto(t *domain.Tool) *pb.Tool {
	if t == nil {
		return nil
	}
	return &pb.Tool{
		Id:                   t.ID,
		OwnerId:              t.OwnerID,
		Name:                 t.Name,
		Description:          t.Description,
		Categories:           t.Categories,
		PricePerDayCents:    t.PricePerDayCents,
		PricePerWeekCents:   t.PricePerWeekCents,
		PricePerMonthCents:  t.PricePerMonthCents,
		ReplacementCostCents: t.ReplacementCostCents,
		Condition:            MapDomainToolConditionToProto(t.Condition),
		Metro:                t.Metro,
		Status:               MapDomainToolStatusToProto(t.Status),
		CreatedOn:            t.CreatedOn.Format("2006-01-02"),
		UpdatedOn:            t.CreatedOn.Format("2006-01-02"),
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
	if r == nil {
		return nil
	}
	proto := &pb.RentalRequest{
		Id:                 r.ID,
		OrganizationId:     r.OrgID,
		ToolId:             r.ToolID,
		RenterId:           r.RenterID,
		OwnerId:            r.OwnerID,
		StartDate:          r.StartDate.Format("2006-01-02"),
		EndDate:            r.ScheduledEndDate.Format("2006-01-02"),
		TotalCost:          r.TotalCostCents,
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
	case domain.RentalStatusScheduled:
		return pb.RentalStatus_RENTAL_STATUS_SCHEDULED
	case domain.RentalStatusActive:
		return pb.RentalStatus_RENTAL_STATUS_ACTIVE
	case domain.RentalStatusCompleted:
		return pb.RentalStatus_RENTAL_STATUS_COMPLETED
	case domain.RentalStatusCancelled:
		return pb.RentalStatus_RENTAL_STATUS_CANCELLED
	default:
		return pb.RentalStatus_RENTAL_STATUS_UNSPECIFIED
	}
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
		proto.RelatedRentalId = *t.RelatedRentalID
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
