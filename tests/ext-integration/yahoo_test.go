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

// TestYahooMailIntegration tests email sending via Yahoo Mail SMTP
// This test requires valid Yahoo Mail credentials to be set in environment variables:
// - YAHOO_SMTP_USER: Yahoo email address
// - YAHOO_SMTP_PASSWORD: Yahoo app password (generate from Yahoo account settings)
func TestYahooMailIntegration(t *testing.T) {
	// Check if credentials are available
	smtpUser := os.Getenv("YAHOO_SMTP_USER")
	smtpPassword := os.Getenv("YAHOO_SMTP_PASSWORD")

	if smtpUser == "" || smtpPassword == "" {
		t.Skip("Skipping Yahoo Mail integration test: YAHOO_SMTP_USER and YAHOO_SMTP_PASSWORD environment variables not set")
	}

	// Initialize Yahoo SMTP email service
	emailService := service.NewEmailService(
		"smtp.mail.yahoo.com",
		"587",
		smtpUser,
		smtpPassword,
		smtpUser, // from address
	)

	t.Run("SendInvitation via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser // Send to self for testing
		invitationCode := fmt.Sprintf("TEST-INVITE-%d", time.Now().Unix())
		orgName := "Test Organization"

		err := emailService.SendInvitation(context.Background(), testEmail, "Test User", invitationCode, orgName, "")
		require.NoError(t, err, "Failed to send invitation email via Yahoo Mail")

		t.Logf("✅ Successfully sent invitation email to %s via Yahoo Mail", testEmail)
		t.Logf("   Invitation Code: %s", invitationCode)
		t.Logf("   Organization: %s", orgName)
	})

	t.Run("SendRentalRequestNotification via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser
		renterName := "Test Renter"
		toolName := "Circular Saw"

		err := emailService.SendRentalRequestNotification(context.Background(), testEmail, renterName, toolName, "")
		require.NoError(t, err, "Failed to send rental request notification via Yahoo Mail")

		t.Logf("✅ Successfully sent rental request notification to %s via Yahoo Mail", testEmail)
		t.Logf("   Renter: %s", renterName)
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendRentalApprovalNotification via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Circular Saw"
		pickupNote := "Tool is ready for pickup. Please bring your ID and rental confirmation."

		err := emailService.SendRentalApprovalNotification(context.Background(), testEmail, toolName, "Owner Name", pickupNote, "")
		require.NoError(t, err, "Failed to send rental approval notification via Yahoo Mail")

		t.Logf("✅ Successfully sent rental approval notification to %s via Yahoo Mail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Pickup Note: %s", pickupNote)
	})

	t.Run("SendRentalCompletionNotification via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Circular Saw"
		totalCost := "$25.00"

		err := emailService.SendRentalCompletionNotification(context.Background(), testEmail, "RENTER", toolName, 2500)
		require.NoError(t, err, "Failed to send rental completion notification via Yahoo Mail")

		t.Logf("✅ Successfully sent rental completion notification to %s via Yahoo Mail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Total Cost: %s", totalCost)
	})

	t.Run("SendRentalCancellationNotification via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Circular Saw"
		reason := "Renter changed plans and no longer needs the tool."

		err := emailService.SendRentalCancellationNotification(context.Background(), testEmail, "Renter Name", toolName, reason, "")
		require.NoError(t, err, "Failed to send rental cancellation notification via Yahoo Mail")

		t.Logf("✅ Successfully sent rental cancellation notification to %s via Yahoo Mail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Reason: %s", reason)
	})

	t.Run("SendRentalConfirmationNotification via Yahoo Mail", func(t *testing.T) {
		testEmail := smtpUser
		toolName := "Circular Saw"
		startDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
		endDate := time.Now().Add(72 * time.Hour).Format("2006-01-02")

		err := emailService.SendRentalConfirmationNotification(context.Background(), testEmail, "Renter Name", toolName, "")
		require.NoError(t, err, "Failed to send rental confirmation notification via Yahoo Mail")

		t.Logf("✅ Successfully sent rental confirmation notification to %s via Yahoo Mail", testEmail)
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Dates: %s to %s", startDate, endDate)
	})

	t.Log("\n📧 Yahoo Mail Integration Test Summary:")
	t.Log("   All email types sent successfully via Yahoo Mail SMTP")
	t.Log("   Please check your inbox to verify email delivery and formatting")
}

// TestYahooMailConnectionFailure tests error handling for invalid credentials
func TestYahooMailConnectionFailure(t *testing.T) {
	// Initialize with invalid credentials
	emailService := service.NewEmailService(
		"smtp.mail.yahoo.com",
		"587",
		"invalid@yahoo.com",
		"wrongpassword",
		"invalid@yahoo.com",
	)

	err := emailService.SendInvitation(context.Background(), "test@example.com", "Name", "TEST-CODE", "Test Org", "")
	assert.Error(t, err, "Should fail with invalid credentials")
	t.Logf("✅ Correctly handled authentication failure: %v", err)
}

// TestYahooMailVsGmail compares behavior between Yahoo and Gmail
func TestYahooMailVsGmail(t *testing.T) {
	yahooUser := os.Getenv("YAHOO_SMTP_USER")
	yahooPassword := os.Getenv("YAHOO_SMTP_PASSWORD")
	gmailUser := os.Getenv("GMAIL_SMTP_USER")
	gmailPassword := os.Getenv("GMAIL_SMTP_PASSWORD")

	if yahooUser == "" || yahooPassword == "" || gmailUser == "" || gmailPassword == "" {
		t.Skip("Skipping comparison test: Both Yahoo and Gmail credentials required")
	}

	yahooService := service.NewEmailService("smtp.mail.yahoo.com", "587", yahooUser, yahooPassword, yahooUser)
	gmailService := service.NewEmailService("smtp.gmail.com", "587", gmailUser, gmailPassword, gmailUser)

	testCode := fmt.Sprintf("COMPARE-%d", time.Now().Unix())
	subject := "Your 2FA Code"
	message := fmt.Sprintf("Your login code is: %s", testCode)

	t.Run("Send same email via both providers", func(t *testing.T) {
		// Send via Yahoo
		err1 := yahooService.SendAdminNotification(context.Background(), yahooUser, subject, message)
		require.NoError(t, err1, "Yahoo send failed")

		// Send via Gmail
		err2 := gmailService.SendAdminNotification(context.Background(), gmailUser, subject, message)
		require.NoError(t, err2, "Gmail send failed")

		t.Log("✅ Successfully sent identical emails via both Yahoo Mail and Gmail")
		t.Logf("   Yahoo recipient: %s", yahooUser)
		t.Logf("   Gmail recipient: %s", gmailUser)
		t.Log("   Please verify both emails were received and formatted correctly")
	})
}
