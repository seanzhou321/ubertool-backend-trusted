package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
)

type AuthService interface {
	ValidateInvite(ctx context.Context, token string) (*domain.Invitation, error)
	RequestToJoin(ctx context.Context, orgID int32, name, email, note string) error
	Signup(ctx context.Context, inviteToken, name, email, phone, password string) (*domain.User, string, string, error)
	Login(ctx context.Context, email, password string) (string, string, string, bool, error) // access, refresh, session, requires2FA
	Verify2FA(ctx context.Context, userID int32, code string) (string, string, error)
	RefreshToken(ctx context.Context, refresh string) (string, string, error)
	Logout(ctx context.Context, refresh string) error
}

type UserService interface {
	GetUserProfile(ctx context.Context, userID int32) (*domain.User, []domain.Organization, []domain.UserOrg, error)
	UpdateProfile(ctx context.Context, userID int32, name, email, phone, avatarURL string) error
}

type OrganizationService interface {
	ListOrganizations(ctx context.Context) ([]domain.Organization, error)
	GetOrganization(ctx context.Context, id int32) (*domain.Organization, error)
	CreateOrganization(ctx context.Context, userID int32, org *domain.Organization) error
	SearchOrganizations(ctx context.Context, name, metro string) ([]domain.Organization, error)
	UpdateOrganization(ctx context.Context, org *domain.Organization) error
	ListMyOrganizations(ctx context.Context, userID int32) ([]domain.Organization, []domain.UserOrg, error)
}

type ImageStorageService interface {
	GetUploadUrl(ctx context.Context, userID int32, filename, contentType string, toolID int32, isPrimary bool) (*domain.ToolImage, string, string, int64, error) // returns image, uploadURL, downloadURL, expiresAt, error
	ConfirmImageUpload(ctx context.Context, userID int32, imageID int32, toolID int32, fileSize int64) (*domain.ToolImage, error)
	GetDownloadUrl(ctx context.Context, userID int32, imageID int32, toolID int32, isThumbnail bool) (string, int64, error) // returns downloadURL, expiresAt, error
	GetToolImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error)
	DeleteImage(ctx context.Context, userID int32, imageID int32, toolID int32) error
	SetPrimaryImage(ctx context.Context, userID int32, toolID int32, imageID int32) error
}

type ToolService interface {
	AddTool(ctx context.Context, tool *domain.Tool, images []string) error
	GetTool(ctx context.Context, id int32) (*domain.Tool, []domain.ToolImage, error)
	UpdateTool(ctx context.Context, tool *domain.Tool) error
	DeleteTool(ctx context.Context, id int32) error
	ListTools(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error)
	ListMyTools(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Tool, int32, error)
	SearchTools(ctx context.Context, userID, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error)
	ListCategories(ctx context.Context) ([]string, error)
}

type RentalService interface {
	CreateRentalRequest(ctx context.Context, renterID, toolID, orgID int32, startDate, endDate string) (*domain.Rental, error)
	ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) (*domain.Rental, error)
	RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error)
	CancelRental(ctx context.Context, renterID, rentalID int32, reason string) (*domain.Rental, error)
	FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) (*domain.Rental, []domain.Rental, []domain.Rental, error)
	CompleteRental(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error)
	ListRentals(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error)
	ListLendings(ctx context.Context, userID, orgID int32, status string, page, pageSize int32) ([]domain.Rental, int32, error)
	GetRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error)
}

type LedgerService interface {
	GetBalance(ctx context.Context, userID, orgID int32) (int32, error)
	GetTransactions(ctx context.Context, userID, orgID int32, page, pageSize int32) ([]domain.LedgerTransaction, int32, error)
	GetLedgerSummary(ctx context.Context, userID, orgID int32) (*domain.LedgerSummary, error)
}

type NotificationService interface {
	GetNotifications(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Notification, int32, error)
	MarkAsRead(ctx context.Context, userID, notificationID int32) error
}

type AdminService interface {
	ApproveJoinRequest(ctx context.Context, adminID, orgID int32, email, name string) error
	BlockUser(ctx context.Context, adminID, userID, orgID int32, isBlock bool, reason string) error
	ListMembers(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error)
	SearchUsers(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error)
	ListJoinRequests(ctx context.Context, orgID int32) ([]domain.JoinRequest, error)
}

type EmailService interface {
	SendInvitation(ctx context.Context, email, name, token string, orgName string, ccEmail string) error
	SendAccountStatusNotification(ctx context.Context, email, name, orgName, status, reason string) error

	// Rental Notifications
	SendRentalRequestNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error
	SendRentalApprovalNotification(ctx context.Context, renterEmail, toolName, ownerName, pickupNote string, ccEmail string) error
	SendRentalRejectionNotification(ctx context.Context, renterEmail, toolName, ownerName string, ccEmail string) error
	SendRentalConfirmationNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error
	SendRentalCancellationNotification(ctx context.Context, ownerEmail, renterName, toolName, reason string, ccEmail string) error
	SendRentalCompletionNotification(ctx context.Context, email, role, toolName string, amount int32) error

	// Admin Notifications
	SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error
}
