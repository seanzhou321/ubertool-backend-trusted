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
	"gopkg.in/yaml.v3"
)

// GmailConfig represents the Gmail test configuration
type GmailConfig struct {
	SMTP struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		From     string `yaml:"from"`
		To       string `yaml:"to"`
		CC       string `yaml:"cc"`
	} `yaml:"smtp"`
}

func loadGmailConfig(t *testing.T) *GmailConfig {
	// Try to find config.test.mail.yaml
	configPath := "../../config/mail_config.test.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("Skipping Gmail integration test: config/config.test.mail.yaml not found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("Skipping Gmail integration test: cannot read config: %v", err)
	}

	var cfg GmailConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Skipf("Skipping Gmail integration test: cannot parse config: %v", err)
	}

	// Validate required fields
	if cfg.SMTP.Host == "" || cfg.SMTP.User == "" || cfg.SMTP.Password == "" {
		t.Skip("Skipping Gmail integration test: incomplete SMTP configuration")
	}

	return &cfg
}

// TestGmailIntegration tests email sending via Gmail SMTP
// This test requires config/config.test.mail.yaml with Gmail credentials
func TestGmailIntegration(t *testing.T) {
	cfg := loadGmailConfig(t)

	// Initialize Gmail SMTP email service
	emailService := service.NewEmailService(
		cfg.SMTP.Host,
		fmt.Sprintf("%d", cfg.SMTP.Port),
		cfg.SMTP.User,
		cfg.SMTP.Password,
		cfg.SMTP.From,
	)

	// Use configured test email addresses
	testEmailTo := cfg.SMTP.To
	testEmailCC := cfg.SMTP.CC
	if testEmailTo == "" {
		testEmailTo = cfg.SMTP.User // fallback to sender
	}

	t.Run("SendInvitation via Gmail", func(t *testing.T) {
		invitationCode := fmt.Sprintf("TEST-INVITE-%d", time.Now().Unix())
		orgName := "Test Organization"

		err := emailService.SendInvitation(context.Background(), testEmailTo, "Test User", invitationCode, orgName, testEmailCC)
		require.NoError(t, err, "Failed to send invitation email via Gmail")

		t.Logf("✅ Successfully sent invitation email to %s via Gmail", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Invitation Code: %s", invitationCode)
		t.Logf("   Organization: %s", orgName)
	})

	t.Run("SendRentalRequestNotification via Gmail", func(t *testing.T) {
		renterName := "Test Renter"
		toolName := "Power Drill"

		err := emailService.SendRentalRequestNotification(context.Background(), testEmailTo, renterName, toolName, testEmailCC)
		require.NoError(t, err, "Failed to send rental request notification via Gmail")

		t.Logf("✅ Successfully sent rental request notification to %s via Gmail", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Renter: %s", renterName)
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendRentalApprovalNotification via Gmail", func(t *testing.T) {
		toolName := "Power Drill"
		pickupNote := "Please pick up the tool from my garage at 123 Main St. Available after 5 PM."

		err := emailService.SendRentalApprovalNotification(context.Background(), testEmailTo, toolName, "Owner Name", pickupNote, testEmailCC)
		require.NoError(t, err, "Failed to send rental approval notification via Gmail")

		t.Logf("✅ Successfully sent rental approval notification to %s via Gmail", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Pickup Note: %s", pickupNote)
	})

	t.Run("SendRentalRejectionNotification via Gmail", func(t *testing.T) {
		toolName := "Power Drill"

		err := emailService.SendRentalRejectionNotification(context.Background(), testEmailTo, toolName, "Owner Name", testEmailCC)
		require.NoError(t, err, "Failed to send rental rejection notification via Gmail")

		t.Logf("✅ Successfully sent rental rejection notification to %s via Gmail", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendAccountStatusNotification via Gmail", func(t *testing.T) {
		orgName := "Test Organization"
		status := "ACTIVE"
		reason := "Your account has been reactivated after review."

		err := emailService.SendAccountStatusNotification(context.Background(), testEmailTo, "User Name", orgName, status, reason)
		require.NoError(t, err, "Failed to send account status notification via Gmail")

		t.Logf("✅ Successfully sent account status notification to %s via Gmail", testEmailTo)
		t.Logf("   Organization: %s", orgName)
		t.Logf("   Status: %s", status)
	})

	t.Run("SendAdminNotification via Gmail", func(t *testing.T) {
		subject := "New Join Request"
		message := "User john.doe@example.com has requested to join your organization."

		err := emailService.SendAdminNotification(context.Background(), testEmailTo, subject, message)
		require.NoError(t, err, "Failed to send admin notification via Gmail")

		t.Logf("✅ Successfully sent admin notification to %s via Gmail", testEmailTo)
		t.Logf("   Subject: %s", subject)
	})

	t.Log("\n📧 Gmail Integration Test Summary:")
	t.Logf("   All email types sent successfully via Gmail SMTP")
	t.Logf("   Primary recipient: %s", testEmailTo)
	if testEmailCC != "" {
		t.Logf("   CC recipient: %s", testEmailCC)
	}
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
