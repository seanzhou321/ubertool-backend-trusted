package unit

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/api/grpc"
	"ubertool-backend-trusted/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestMapDomainUserToProto(t *testing.T) {
	now := time.Now()
	u := &domain.User{
		ID:          1,
		Email:       "test@example.com",
		PhoneNumber: "1234567890",
		Name:        "Test User",
		AvatarURL:   "http://avatar.com",
		CreatedOn:   now,
	}

	proto := grpc.MapDomainUserToProto(u)

	assert.NotNil(t, proto)
	assert.Equal(t, u.ID, proto.Id)
	assert.Equal(t, u.Email, proto.Email)
	assert.Equal(t, u.PhoneNumber, proto.Phone)
	assert.Equal(t, u.Name, proto.Name)
	assert.Equal(t, u.AvatarURL, proto.AvatarUrl)
	assert.Equal(t, u.CreatedOn.Format("2006-01-02"), proto.CreatedOn)

	assert.Nil(t, grpc.MapDomainUserToProto(nil))
}

func TestMapDomainOrgToProto(t *testing.T) {
	now := time.Now()
	o := &domain.Organization{
		ID:          1,
		Name:        "Test Org",
		Description: "Test Desc",
		Address:     "123 Test St",
		Metro:       "Test Metro",
		CreatedOn:   now,
	}

	proto := grpc.MapDomainOrgToProto(o)

	assert.NotNil(t, proto)
	assert.Equal(t, o.ID, proto.Id)
	assert.Equal(t, o.Name, proto.Name)
	assert.Equal(t, o.Description, proto.Description)
	assert.Equal(t, o.Address, proto.Address)
	assert.Equal(t, o.Metro, proto.Metro)
	assert.Equal(t, o.CreatedOn.Format("2006-01-02"), proto.CreatedOn)

	assert.Nil(t, grpc.MapDomainOrgToProto(nil))
}

func TestMapDomainToolToProto(t *testing.T) {
	now := time.Now()
	tool := &domain.Tool{
		ID:                   1,
		OwnerID:              2,
		Name:                 "Test Tool",
		Description:          "Test Desc",
		Categories:           []string{"Power Tools"},
		PricePerDayCents:    1000,
		ReplacementCostCents: 5000,
		Condition:            domain.ToolConditionExcellent,
		Status:               domain.ToolStatusAvailable,
		CreatedOn:            now,
	}

	proto := grpc.MapDomainToolToProto(tool)

	assert.NotNil(t, proto)
	assert.Equal(t, tool.ID, proto.Id)
	assert.Equal(t, tool.OwnerID, proto.OwnerId)
	assert.Equal(t, tool.Name, proto.Name)
	assert.Equal(t, pb.ToolCondition_TOOL_CONDITION_EXCELLENT, proto.Condition)
	assert.Equal(t, pb.ToolStatus_TOOL_STATUS_AVAILABLE, proto.Status)

	assert.Nil(t, grpc.MapDomainToolToProto(nil))
}

func TestMapDomainRentalToProto(t *testing.T) {
	now := time.Now()
	r := &domain.Rental{
		ID:               1,
		OrgID:            2,
		ToolID:           3,
		RenterID:         4,
		OwnerID:          5,
		StartDate:        now,
		ScheduledEndDate: now.Add(24 * time.Hour),
		TotalCostCents:   2000,
		Status:           domain.RentalStatusApproved,
		PickupNote:       "Leave at front door",
		CreatedOn:        now,
		UpdatedOn:        now,
	}

	proto := grpc.MapDomainRentalToProto(r)

	assert.NotNil(t, proto)
	assert.Equal(t, r.ID, proto.Id)
	assert.Equal(t, r.OrgID, proto.OrganizationId)
	assert.Equal(t, pb.RentalStatus_RENTAL_STATUS_APPROVED, proto.Status)
	assert.Equal(t, r.PickupNote, proto.PickupInstructions)

	assert.Nil(t, grpc.MapDomainRentalToProto(nil))
}

func TestMapDomainTransactionToProto(t *testing.T) {
	now := time.Now()
	rentalID := int32(10)
	tx := &domain.LedgerTransaction{
		ID:              1,
		OrgID:           2,
		UserID:          3,
		Amount:          -500,
		Type:            domain.TransactionTypeRentalDebit,
		Description:     "Test Tx",
		ChargedOn:       now,
		RelatedRentalID: &rentalID,
	}

	proto := grpc.MapDomainTransactionToProto(tx)

	assert.NotNil(t, proto)
	assert.Equal(t, tx.ID, proto.Id)
	assert.Equal(t, pb.TransactionType_TRANSACTION_TYPE_RENTAL_DEBIT, proto.Type)
	assert.Equal(t, pb.TransactionType_TRANSACTION_TYPE_RENTAL_DEBIT, proto.Type)
	// assert.Equal(t, rentalID, proto.RelatedRentalId) // Proto has RelatedRental object, not ID

	assert.Nil(t, grpc.MapDomainTransactionToProto(nil))
}
