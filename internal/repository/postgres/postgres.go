package postgres

import (
	"database/sql"
	"ubertool-backend-trusted/internal/repository"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
	repository.UserRepository
	repository.OrganizationRepository
	repository.ToolRepository
	repository.RentalRepository
	repository.LedgerRepository
	repository.NotificationRepository
	repository.InvitationRepository
	repository.JoinRequestRepository
	repository.BillRepository
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db:                     db,
		UserRepository:         NewUserRepository(db),
		OrganizationRepository: NewOrganizationRepository(db),
		ToolRepository:         NewToolRepository(db),
		RentalRepository:       NewRentalRepository(db),
		LedgerRepository:       NewLedgerRepository(db),
		NotificationRepository: NewNotificationRepository(db),
		InvitationRepository:   NewInvitationRepository(db),
		JoinRequestRepository:  NewJoinRequestRepository(db),
		BillRepository:         NewBillRepository(db),
	}
}
