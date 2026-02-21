package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
)

type AuthService interface {
	ValidateInvite(ctx context.Context, inviteCode, email string) (bool, string, *domain.User, error)
	RequestToJoin(ctx context.Context, orgID int32, name, email, note, adminEmail string) error
	Signup(ctx context.Context, inviteToken, name, email, phone, password string) error
	Login(ctx context.Context, email, password string) (string, string, string, bool, error) // access, refresh, session, requires2FA
	Verify2FA(ctx context.Context, userID int32, code string) (string, string, *domain.User, error)
	RefreshToken(ctx context.Context, refresh string) (string, string, error)
	Logout(ctx context.Context, refresh string) error
}

type UserService interface {
	GetUserProfile(ctx context.Context, userID int32) (*domain.User, []domain.Organization, []domain.UserOrg, error)
	UpdateProfile(ctx context.Context, userID int32, name, email, phone, avatarURL string) error
}

type OrganizationService interface {
	ListOrganizations(ctx context.Context) ([]domain.Organization, error)
	GetOrganization(ctx context.Context, id int32, callingUserID int32) (*domain.Organization, *domain.UserOrg, error)
	CreateOrganization(ctx context.Context, userID int32, org *domain.Organization) error
	SearchOrganizations(ctx context.Context, name, metro string) ([]domain.Organization, error)
	UpdateOrganization(ctx context.Context, org *domain.Organization) error
	ListMyOrganizations(ctx context.Context, userID int32) ([]domain.Organization, []domain.UserOrg, error)
	JoinOrganizationWithInvite(ctx context.Context, userID int32, inviteCode string) (*domain.Organization, *domain.User, error)
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
	GetTool(ctx context.Context, id, requestingUserID int32) (*domain.Tool, []domain.ToolImage, error)
	UpdateTool(ctx context.Context, tool *domain.Tool) error
	DeleteTool(ctx context.Context, id int32) error
	ListTools(ctx context.Context, orgID, requestingUserID int32, page, pageSize int32) ([]domain.Tool, int32, error)
	ListMyTools(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Tool, int32, error)
	SearchTools(ctx context.Context, userID, orgID int32, metro, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error)
	ListCategories(ctx context.Context) ([]string, error)
}

type RentalService interface {
	CreateRentalRequest(ctx context.Context, renterID, toolID, orgID int32, startDate, endDate string) (*domain.Rental, error)
	ApproveRentalRequest(ctx context.Context, ownerID, rentalID int32, pickupNote string) (*domain.Rental, error)
	RejectRentalRequest(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error)
	CancelRental(ctx context.Context, renterID, rentalID int32, reason string) (*domain.Rental, error)
	FinalizeRentalRequest(ctx context.Context, renterID, rentalID int32) (*domain.Rental, []domain.Rental, []domain.Rental, error)
	CompleteRental(ctx context.Context, ownerID, rentalID int32, returnCondition string, surchargeOrCreditCents int32, notes string) (*domain.Rental, error)
	Update(ctx context.Context, rt *domain.Rental) error
	ListRentals(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error)
	ListLendings(ctx context.Context, userID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error)
	GetRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error)

	// New methods
	ActivateRental(ctx context.Context, userID, rentalID int32) (*domain.Rental, error)
	ChangeRentalDates(ctx context.Context, userID, rentalID int32, newStart, newEnd, oldStart, oldEnd string) (*domain.Rental, error)
	ApproveReturnDateChange(ctx context.Context, ownerID, rentalID int32) (*domain.Rental, error)
	RejectReturnDateChange(ctx context.Context, ownerID, rentalID int32, reason, newEndDate string) (*domain.Rental, error)
	AcknowledgeReturnDateRejection(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error)
	CancelReturnDateChange(ctx context.Context, renterID, rentalID int32) (*domain.Rental, error)
	ListToolRentals(ctx context.Context, ownerID, toolID, orgID int32, statuses []string, page, pageSize int32) ([]domain.Rental, int32, error)
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
	ApproveJoinRequest(ctx context.Context, adminID, orgID, joinRequestID int32) (invitationCode string, err error)
	BlockUser(ctx context.Context, adminID, userID, orgID int32, blockRenting, blockLending bool, reason string) error
	ListMembers(ctx context.Context, orgID int32) ([]domain.User, []domain.UserOrg, error)
	SearchUsers(ctx context.Context, orgID int32, query string) ([]domain.User, []domain.UserOrg, error)
	ListJoinRequests(ctx context.Context, orgID int32) ([]domain.JoinRequest, error)
	RejectJoinRequest(ctx context.Context, adminID, orgID, joinRequestID int32, reason string) error
	SendInvitation(ctx context.Context, adminID, orgID int32, email, name string) (string, error)
	GetMemberProfile(ctx context.Context, orgID, userID int32) (*domain.User, *domain.UserOrg, error)
}

type BillSplitService interface {
	GetGlobalBillSplitSummary(ctx context.Context, userID int32) (paymentsToMake, receiptsToVerify, paymentsInDispute, receiptsInDispute int32, err error)
	GetOrganizationBillSplitSummary(ctx context.Context, userID int32) ([]domain.Organization, []int32, []int32, []int32, []int32, error)
	ListPayments(ctx context.Context, userID, orgID int32, showHistory bool) ([]domain.Bill, error)
	GetPaymentDetail(ctx context.Context, userID, paymentID int32) (*domain.Bill, []domain.BillAction, bool, error)
	AcknowledgePayment(ctx context.Context, userID, paymentID int32) error
	ListDisputedPayments(ctx context.Context, adminID, orgID int32) ([]domain.Bill, error)
	ListResolvedDisputes(ctx context.Context, adminID, orgID int32) ([]domain.Bill, error)
	ResolveDispute(ctx context.Context, adminID, paymentID int32, resolution, notes string) error
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
	SendRentalPickupNotification(ctx context.Context, email, name, toolName, startDate, endDate string) error
	SendReturnDateRejectionNotification(ctx context.Context, renterEmail, toolName, newEndDate, reason string, totalCostCents int32) error

	// Admin Notifications
	SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error

	// Bill Split Notifications
	SendBillPaymentNotice(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error
	SendBillPaymentAcknowledgment(ctx context.Context, creditorEmail, creditorName, debtorName string, amountCents int32, settlementMonth string, orgName string) error
	SendBillReceiptConfirmation(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error
	SendBillDisputeNotification(ctx context.Context, email, name, otherPartyName string, amountCents int32, reason string, orgName string) error
	SendBillDisputeResolutionNotification(ctx context.Context, email, name string, amountCents int32, resolution, notes string, orgName string) error
}
