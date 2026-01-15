package repository

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id int32) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	
	// User Organizations
	AddUserToOrg(ctx context.Context, userOrg *domain.UserOrg) error
	GetUserOrg(ctx context.Context, userID, orgID int32) (*domain.UserOrg, error)
	ListUserOrgs(ctx context.Context, userID int32) ([]domain.UserOrg, error)
	UpdateUserOrg(ctx context.Context, userOrg *domain.UserOrg) error
}

type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	GetByID(ctx context.Context, id int32) (*domain.Organization, error)
	List(ctx context.Context) ([]domain.Organization, error)
	Search(ctx context.Context, name, metro string) ([]domain.Organization, error)
}

type ToolRepository interface {
	Create(ctx context.Context, tool *domain.Tool) error
	GetByID(ctx context.Context, id int32) (*domain.Tool, error)
	Update(ctx context.Context, tool *domain.Tool) error
	Delete(ctx context.Context, id int32) error
	ListByOrg(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error)
	Search(ctx context.Context, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error)
	
	AddImage(ctx context.Context, image *domain.ToolImage) error
	GetImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error)
}

type RentalRepository interface {
	Create(ctx context.Context, rental *domain.Rental) error
	GetByID(ctx context.Context, id int32) (*domain.Rental, error)
	Update(ctx context.Context, rental *domain.Rental) error
	ListByRenter(ctx context.Context, renterID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error)
	ListByOwner(ctx context.Context, ownerID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error)
}

type LedgerRepository interface {
	CreateTransaction(ctx context.Context, tx *domain.LedgerTransaction) error
	GetBalance(ctx context.Context, userID, orgID int32) (int32, error)
	ListTransactions(ctx context.Context, userID, orgID int32, page, pageSize int32) ([]domain.LedgerTransaction, int32, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, note *domain.Notification) error
	List(ctx context.Context, userID int32, limit, offset int32) ([]domain.Notification, int32, error)
	MarkAsRead(ctx context.Context, id, userID int32) error
}

type InvitationRepository interface {
	Create(ctx context.Context, invite *domain.Invitation) error
	GetByToken(ctx context.Context, token string) (*domain.Invitation, error)
	Update(ctx context.Context, invite *domain.Invitation) error
}

type JoinRequestRepository interface {
	Create(ctx context.Context, req *domain.JoinRequest) error
	GetByID(ctx context.Context, id int32) (*domain.JoinRequest, error)
	Update(ctx context.Context, req *domain.JoinRequest) error
	ListByOrg(ctx context.Context, orgID int32) ([]domain.JoinRequest, error)
}
