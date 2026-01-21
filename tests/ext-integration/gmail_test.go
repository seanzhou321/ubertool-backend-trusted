package extintegration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGmailIntegration tests email sending via Gmail SMTP
// This test requires valid Gmail credentials to be set in environment variables:
// - GMAIL_SMTP_USER: Gmail email address
// - GMAIL_SMTP_PASSWORD: Gmail app password (not regular password)
func TestGmailIntegration(t *testing.T) {
	// Check if credentials are available
	smtpUser := os.Getenv("GMAIL_SMTP_USER")
	smtpPassword := os.Getenv("GMAIL_SMTP_PASSWORD")

	if smtpUser == "" || smtpPassword == "" {
		t.Skip("Skipping Gmail integration test: GMAIL_SMTP_USER and GMAIL_SMTP_PASSWORD environment variables not set")
	}

	// Initialize Gmail SMTP email service
	emailService := service.NewEmailService(
		"smtp.gmail.com",
		"587",
		smtpUser,
		smtpPassword,
		smtpUser, // from address
	)

	t.Run("SendInvitation via Gmail", func(t *testing.T) {
		testEmail := smtpUser // Send to self for testing
		invitationCode := fmt.Sprintf("TEST-INVITE-%d", time.Now().Unix())
		orgName := "Test Organization"

		err := emailService.SendInvitation(context.Background(), testEmail, "Test User", invitationCode, orgName, "")
		require.NoError(t, err, "Failed to send invitation email via Gmail")

		t.Logf("✅ Successfully sent invitation email to %s via Gmail", testEmail)
		t.Logf("   Invitation Code: %s", invitationCode)
		t.Logf("   Organization: %s", orgName)
	})

	t.Run("SendRentalRequestNotification via Gmail", func(t *testing.T) {
		testEmail := smtpUser
		renterName := "Test Renter"
		toolName := "Power Drill"

		err := emailService.SendRentalRequestNotification(context.Background(), testEmail, renterName, toolName, "")
		require.NoError(t, err, "Failed to send rental request notification via Gmail")

		t.Logf("✅ Successfully sent rental request notification to %s via Gmail", testEmail)
		t.Logf("   Renter: %s", renterName)
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendRentalApprovalNotification via Gmail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Power Drill"
		pickupNote := "Please pick up the tool from my garage at 123 Main St. Available after 5 PM."

		err := emailService.SendRentalApprovalNotification(context.Background(), testEmail, toolName, "Owner Name", pickupNote, "")
		require.NoError(t, err, "Failed to send rental approval notification via Gmail")

		t.Logf("✅ Successfully sent rental approval notification to %s via Gmail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Pickup Note: %s", pickupNote)
	})

	t.Run("SendRentalRejectionNotification via Gmail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Power Drill"
		reason := "Tool is currently under maintenance and not available."

		err := emailService.SendRentalRejectionNotification(context.Background(), testEmail, toolName, "Owner Name", "")
		require.NoError(t, err, "Failed to send rental rejection notification via Gmail")

		t.Logf("✅ Successfully sent rental rejection notification to %s via Gmail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Reason: %s", reason)
	})

	t.Run("SendAccountStatusNotification via Gmail", func(t *testing.T) {
		testEmail := smtpUser
		orgName := "Test Organization"
		status := "ACTIVE"
		reason := "Your account has been reactivated after review."

		err := emailService.SendAccountStatusNotification(context.Background(), testEmail, "User Name", orgName, status, reason)
		require.NoError(t, err, "Failed to send account status notification via Gmail")

		t.Logf("✅ Successfully sent account status notification to %s via Gmail", testEmail)
		t.Logf("   Organization: %s", orgName)
		t.Logf("   Status: %s", status)
	})

	t.Run("SendAdminNotification via Gmail", func(t *testing.T) {
		testEmail := smtpUser
		subject := "New Join Request"
		message := "User john.doe@example.com has requested to join your organization."

		err := emailService.SendAdminNotification(context.Background(), testEmail, subject, message)
		require.NoError(t, err, "Failed to send admin notification via Gmail")

		t.Logf("✅ Successfully sent admin notification to %s via Gmail", testEmail)
		t.Logf("   Subject: %s", subject)
	})

	t.Log("\n📧 Gmail Integration Test Summary:")
	t.Log("   All email types sent successfully via Gmail SMTP")
	t.Log("   Please check your inbox to verify email delivery and formatting")
}

// TestGmailConnectionFailure tests error handling for invalid credentials
func TestGmailConnectionFailure(t *testing.T) {
	// Initialize with invalid credentials
	emailService := service.NewEmailService(
		"smtp.gmail.com",
		"587",
		"invalid@gmail.com",
		"wrongpassword",
		"invalid@gmail.com",
	)

	err := emailService.SendInvitation(context.Background(), "test@example.com", "Name", "TEST-CODE", "Test Org", "")
	assert.Error(t, err, "Should fail with invalid credentials")
	t.Logf("✅ Correctly handled authentication failure: %v", err)
}
