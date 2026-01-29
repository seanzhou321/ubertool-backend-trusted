// config/security_config.go
package config

type SecurityLevel int

const (
	SecurityPublic  SecurityLevel = iota // No authentication
	Security2FA                          // 2FA token required
	SecurityRefresh                      // Refresh token required
	SecurityAccess                       // Access token required
)

// EndpointSecurityConfig maps methods to their required security level
var EndpointSecurityConfig = map[string]SecurityLevel{
	// AuthService - Public
	"/ubertool.trusted.api.v1.AuthService/UserSignup":                SecurityPublic,
	"/ubertool.trusted.api.v1.AuthService/ValidateInvite":            SecurityPublic,
	"/ubertool.trusted.api.v1.AuthService/RequestToJoinOrganization": SecurityPublic,
	// TODO: add health check to auth service
	// "/ubertool.trusted.api.v1.AuthService/HealthCheck":           SecurityPublic,
	"/ubertool.trusted.api.v1.AuthService/Login": SecurityPublic,

	// AuthService - 2FA Protected
	"/ubertool.trusted.api.v1.AuthService/Verify2FA": Security2FA,
	// TODO: add resend code to auth service
	// "/ubertool.trusted.api.v1.AuthService/ResendCode":   Security2FA,

	// AuthService - Refresh Protected
	"/ubertool.trusted.api.v1.AuthService/RefreshToken": SecurityRefresh,

	// AuthService - Access Protected
	"/ubertool.trusted.api.v1.AuthService/Logout": SecurityAccess,
	// TODO: add change password to auth service
	// "/ubertool.trusted.api.v1.AuthService/ChangePassword": SecurityAccess,

	// OrganizationService - Public
	"/ubertool.trusted.api.v1.OrganizationService/SearchOrganizations": SecurityPublic,

	// OrganizationService - Access Protected
	"/ubertool.trusted.api.v1.OrganizationService/GetOrganization":     SecurityAccess,
	"/ubertool.trusted.api.v1.OrganizationService/CreateOrganization":  SecurityAccess,
	"/ubertool.trusted.api.v1.OrganizationService/UpdateOrganization":  SecurityAccess,
	"/ubertool.trusted.api.v1.OrganizationService/ListMyOrganizations": SecurityAccess,

	// UserService - All Access Protected
	"/ubertool.trusted.api.v1.UserService/GetUser":       SecurityAccess,
	"/ubertool.trusted.api.v1.UserService/UpdateProfile": SecurityAccess,

	// AdminService - All Access Protected
	"/ubertool.trusted.api.v1.AdminService/ApproveRequestToJoin":  SecurityAccess,
	"/ubertool.trusted.api.v1.AdminService/AdminBlockUserAccount": SecurityAccess,
	"/ubertool.trusted.api.v1.AdminService/ListMembers":           SecurityAccess,
	"/ubertool.trusted.api.v1.AdminService/SearchUsers":           SecurityAccess,
	"/ubertool.trusted.api.v1.AdminService/ListJoinRequests":      SecurityAccess,

	// ImageStorageService - Access Protected
	"/ubertool.trusted.api.v1.ImageStorageService/GetUploadUrl": SecurityAccess,

	// LedgerService - Access Protected
	"/ubertool.trusted.api.v1.LedgerService/GetBalance":       SecurityAccess,
	"/ubertool.trusted.api.v1.LedgerService/GetTransactions":  SecurityAccess,
	"/ubertool.trusted.api.v1.LedgerService/GetLedgerSummary": SecurityAccess,

	// NotificationService - Access Protected
	"/ubertool.trusted.api.v1.NotificationService/GetNotifications":     SecurityAccess,
	"/ubertool.trusted.api.v1.NotificationService/MarkNotificationRead": SecurityAccess,

	// RentalService - Access Protected
	"/ubertool.trusted.api.v1.RentalService/ApproveRentalRequest":  SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/RejectRentalRequest":   SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/ListMyLendings":        SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/CompleteRental":        SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/GetRental":             SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/CreateRentalRequest":   SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/FinalizeRentalRequest": SecurityAccess,
	"/ubertool.trusted.api.v1.RentalService/ListMyRentals":         SecurityAccess,

	// ToolService - Access Protected
	"/ubertool.trusted.api.v1.ToolService/ListTools":          SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/GetTool":            SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/AddTool":            SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/UpdateTool":         SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/DeleteTool":         SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/SearchTools":        SecurityAccess,
	"/ubertool.trusted.api.v1.ToolService/ListToolCategories": SecurityAccess,
}

// GetSecurityLevel returns the security level for a given method
func GetSecurityLevel(method string) SecurityLevel {
	if level, exists := EndpointSecurityConfig[method]; exists {
		return level
	}
	// Default to highest security for unknown endpoints
	return SecurityAccess
}
