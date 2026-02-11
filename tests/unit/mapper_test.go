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
		CreatedOn:   now.Format("2006-01-02"),
	}

	proto := grpc.MapDomainUserToProto(u)

	assert.NotNil(t, proto)
	assert.Equal(t, u.ID, proto.Id)
	assert.Equal(t, u.Email, proto.Email)
	assert.Equal(t, u.PhoneNumber, proto.Phone)
	assert.Equal(t, u.Name, proto.Name)
	assert.Equal(t, u.AvatarURL, proto.AvatarUrl)
	assert.Equal(t, u.CreatedOn, proto.CreatedOn)

	assert.Nil(t, grpc.MapDomainUserToProto(nil))
}

func TestMapDomainUserToProto_WithOrgs(t *testing.T) {
	now := time.Now()
	u := &domain.User{
		ID:        1,
		Email:     "test@example.com",
		Name:      "Test User",
		CreatedOn: now.Format("2006-01-02"),
		Orgs: []domain.Organization{
			{ID: 1, Name: "Org 1", Metro: "Metro 1", CreatedOn: now.Format("2006-01-02")},
			{ID: 2, Name: "Org 2", Metro: "Metro 2", CreatedOn: now.Format("2006-01-02")},
		},
	}

	proto := grpc.MapDomainUserToProto(u)

	assert.NotNil(t, proto)
	assert.Equal(t, u.ID, proto.Id)
	assert.NotNil(t, proto.Orgs, "Orgs should not be nil")
	assert.Len(t, proto.Orgs, 2, "Should have 2 orgs")
	assert.Equal(t, int32(1), proto.Orgs[0].Id)
	assert.Equal(t, "Org 1", proto.Orgs[0].Name)
	assert.Equal(t, int32(2), proto.Orgs[1].Id)
	assert.Equal(t, "Org 2", proto.Orgs[1].Name)
}

func TestMapDomainOrgToProto(t *testing.T) {
	now := time.Now()
	o := &domain.Organization{
		ID:          1,
		Name:        "Test Org",
		Description: "Test Desc",
		Address:     "123 Test St",
		Metro:       "Test Metro",
		CreatedOn:   now.Format("2006-01-02"),
	}

	// Test with user role
	proto := grpc.MapDomainOrgToProto(o, "ADMIN")

	assert.NotNil(t, proto)
	assert.Equal(t, o.ID, proto.Id)
	assert.Equal(t, o.Name, proto.Name)
	assert.Equal(t, o.Description, proto.Description)
	assert.Equal(t, o.Address, proto.Address)
	assert.Equal(t, o.Metro, proto.Metro)
	assert.Equal(t, o.CreatedOn, proto.CreatedOn)
	assert.Equal(t, "ADMIN", proto.UserRole)

	// Test without user role (empty string)
	protoNoRole := grpc.MapDomainOrgToProto(o, "")
	assert.Equal(t, "", protoNoRole.UserRole)

	// Test nil
	assert.Nil(t, grpc.MapDomainOrgToProto(nil, ""))
}

func TestMapDomainToolToProto(t *testing.T) {
	now := time.Now()
	tool := &domain.Tool{
		ID:                   1,
		OwnerID:              2,
		Name:                 "Test Tool",
		Description:          "Test Desc",
		Categories:           []string{"Power Tools"},
		PricePerDayCents:     1000,
		ReplacementCostCents: 5000,
		Condition:            domain.ToolConditionExcellent,
		Status:               domain.ToolStatusAvailable,
		CreatedOn:            now.Format("2006-01-02"),
	}

	proto := grpc.MapDomainToolToProto(tool)

	assert.NotNil(t, proto)
	assert.Equal(t, tool.ID, proto.Id)
	assert.Nil(t, proto.Owner) // Owner is nil because tool.Owner is not populated
	assert.Equal(t, tool.Name, proto.Name)
	assert.Equal(t, pb.ToolCondition_TOOL_CONDITION_EXCELLENT, proto.Condition)
	assert.Equal(t, pb.ToolStatus_TOOL_STATUS_AVAILABLE, proto.Status)

	assert.Nil(t, grpc.MapDomainToolToProto(nil))
}

func TestMapDomainRentalToProto(t *testing.T) {
	now := time.Now()
	r := &domain.Rental{
		ID:             1,
		OrgID:          2,
		ToolID:         3,
		RenterID:       4,
		OwnerID:        5,
		StartDate:      now.Format("2006-01-02"),
		EndDate:        now.Add(24 * time.Hour).Format("2006-01-02"),
		TotalCostCents: 2000,
		Status:         domain.RentalStatusApproved,
		PickupNote:     "Leave at front door",
		CreatedOn:      now.Format("2006-01-02"),
		UpdatedOn:      now.Format("2006-01-02"),
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
		ChargedOn:       now.Format("2006-01-02"),
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
