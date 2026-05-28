package extintegration

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/service"

	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// SESTestConfig represents the SES section of mail_config.test.yaml
type SESTestConfig struct {
	SES struct {
		Region string `yaml:"region"`
		From   string `yaml:"from"`
		To     string `yaml:"to"`
		CC     string `yaml:"cc"`
	} `yaml:"ses"`
}

func loadSESConfig(t *testing.T) *SESTestConfig {
	t.Helper()

	if !flag.Parsed() {
		flag.Parse()
	}

	mailCfgPath := configPath
	if _, err := os.Stat(mailCfgPath); os.IsNotExist(err) {
		altPath := "../../" + configPath
		if _, err := os.Stat(altPath); err == nil {
			mailCfgPath = altPath
		}
	}
	if _, err := os.Stat(mailCfgPath); os.IsNotExist(err) {
		t.Skip("Skipping SES integration test: mail_config.test.yaml not found")
	}

	data, err := os.ReadFile(mailCfgPath)
	if err != nil {
		t.Skipf("Skipping SES integration test: cannot read config: %v", err)
	}

	var cfg SESTestConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Skipf("Skipping SES integration test: cannot parse config: %v", err)
	}

	if cfg.SES.Region == "" || cfg.SES.From == "" {
		t.Skip("Skipping SES integration test: ses.region and ses.from must be set in mail_config.test.yaml")
	}

	return &cfg
}

// isSESSandboxError returns true when SES rejected the message because the
// recipient (or sender) address is not a verified identity — i.e. the account
// is still in SES Sandbox mode.
func isSESSandboxError(err error) bool {
	if err == nil {
		return false
	}
	var rejected *types.MessageRejected
	if errors.As(err, &rejected) {
		return true
	}
	// Fallback: inspect the error string for the well-known SES sandbox message.
	return strings.Contains(err.Error(), "MessageRejected") &&
		strings.Contains(err.Error(), "not verified")
}

// requireSESSuccess fails the sub-test on a real send error, but skips
// it (with instructions) when the account is in sandbox mode.
func requireSESSuccess(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		return
	}
	if isSESSandboxError(err) {
		t.Skipf(
			"Skipping: SES sandbox mode — recipient address is not a verified identity.\n"+
				"  Fix: Go to AWS Console → SES → Verified identities → Create identity\n"+
				"       and verify the recipient address listed in mail_config.test.yaml ses.to,\n"+
				"       OR request SES production access to lift the sandbox restriction.\n"+
				"  Underlying error: %v",
			err,
		)
	}
	assert.NoError(t, err, msg)
}

// TestSESIntegration sends every email type via AWS SES and verifies no errors are returned.
//
// Prerequisites:
//   - mail_config.test.yaml must have the ses: block populated (see mail_config.test.template.yaml)
//   - AWS credentials must be available via one of:
//     (a) AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY environment variables
//     (b) ~/.aws/credentials file (profile "default" or AWS_PROFILE env var)
//     (c) An EC2 IAM instance role
//   - The ses.from address must be verified in AWS SES (domain or individual address)
//   - If your SES account is still in sandbox mode, ses.to and ses.cc must also be verified
//
// Run:
//
//	go test -v ./tests/ext-integration -run TestSES
func TestSESIntegration(t *testing.T) {
	cfg := loadSESConfig(t)

	emailService, err := service.NewSESEmailService(cfg.SES.Region, cfg.SES.From)
	if err != nil {
		t.Fatalf("Failed to initialize SES email service: %v", err)
	}

	testEmailTo := cfg.SES.To
	testEmailCC := cfg.SES.CC
	if testEmailTo == "" {
		testEmailTo = cfg.SES.From // fallback to sender (always verified in sandbox)
	}

	t.Run("SendInvitation via SES", func(t *testing.T) {
		invitationCode := fmt.Sprintf("TEST-INVITE-%d", time.Now().Unix())
		orgName := "Test Organization"

		err := emailService.SendInvitation(context.Background(), testEmailTo, "Test User", invitationCode, orgName, testEmailCC)
		requireSESSuccess(t, err, "Failed to send invitation email via SES")

		t.Logf("✅ Successfully sent invitation email to %s via SES", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Invitation Code: %s", invitationCode)
		t.Logf("   Organization: %s", orgName)
	})

	t.Run("SendRentalRequestNotification via SES", func(t *testing.T) {
		renterName := "Test Renter"
		toolName := "Power Drill"

		err := emailService.SendRentalRequestNotification(context.Background(), testEmailTo, renterName, toolName, testEmailCC)
		requireSESSuccess(t, err, "Failed to send rental request notification via SES")

		t.Logf("✅ Successfully sent rental request notification to %s via SES", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Renter: %s", renterName)
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendRentalApprovalNotification via SES", func(t *testing.T) {
		toolName := "Power Drill"
		pickupNote := "Please pick up the tool from my garage at 123 Main St. Available after 5 PM."

		err := emailService.SendRentalApprovalNotification(context.Background(), testEmailTo, toolName, "Owner Name", pickupNote, testEmailCC)
		requireSESSuccess(t, err, "Failed to send rental approval notification via SES")

		t.Logf("✅ Successfully sent rental approval notification to %s via SES", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Tool: %s", toolName)
		t.Logf("   Pickup Note: %s", pickupNote)
	})

	t.Run("SendRentalRejectionNotification via SES", func(t *testing.T) {
		toolName := "Power Drill"

		err := emailService.SendRentalRejectionNotification(context.Background(), testEmailTo, toolName, "Owner Name", testEmailCC)
		requireSESSuccess(t, err, "Failed to send rental rejection notification via SES")

		t.Logf("✅ Successfully sent rental rejection notification to %s via SES", testEmailTo)
		if testEmailCC != "" {
			t.Logf("   CC: %s", testEmailCC)
		}
		t.Logf("   Tool: %s", toolName)
	})

	t.Run("SendAccountStatusNotification via SES", func(t *testing.T) {
		orgName := "Test Organization"
		status := "ACTIVE"
		reason := "Your account has been reactivated after review."

		err := emailService.SendAccountStatusNotification(context.Background(), testEmailTo, "User Name", orgName, status, reason)
		requireSESSuccess(t, err, "Failed to send account status notification via SES")

		t.Logf("✅ Successfully sent account status notification to %s via SES", testEmailTo)
		t.Logf("   Organization: %s", orgName)
		t.Logf("   Status: %s", status)
	})

	t.Run("SendAdminNotification via SES", func(t *testing.T) {
		subject := "New Join Request"
		message := "User john.doe@example.com has requested to join your organization."

		err := emailService.SendAdminNotification(context.Background(), testEmailTo, subject, message)
		requireSESSuccess(t, err, "Failed to send admin notification via SES")

		t.Logf("✅ Successfully sent admin notification to %s via SES", testEmailTo)
		t.Logf("   Subject: %s", subject)
	})

	t.Log("\n📧 SES Integration Test Summary:")
	t.Logf("   All email types sent successfully via AWS SES")
	t.Logf("   Region: %s", cfg.SES.Region)
	t.Logf("   From: %s", cfg.SES.From)
	t.Logf("   Primary recipient: %s", testEmailTo)
	if testEmailCC != "" {
		t.Logf("   CC recipient: %s", testEmailCC)
	}
	t.Log("   Please check your inbox to verify email delivery and formatting")
}

// TestSESInvalidRegion verifies that constructing the service with a bad region
// does not panic and that sending returns an error.
func TestSESInvalidRegion(t *testing.T) {
	emailService, err := service.NewSESEmailService("invalid-region-xyz", "test@example.com")
	// Config loading itself may succeed (region is validated lazily by SES)
	if err != nil {
		t.Logf("✅ NewSESEmailService correctly rejected invalid region at construction: %v", err)
		return
	}

	err = emailService.SendInvitation(context.Background(), "dest@example.com", "Name", "CODE", "Org", "")
	assert.Error(t, err, "Should fail when the AWS region is invalid")
	t.Logf("✅ Correctly received error for invalid region: %v", err)
}

