package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

type emailService struct {
	smtpHost    string
	smtpPort    string
	senderEmail string
	senderPass  string
	senderName  string
}

func NewEmailService(host, port, email, password, name string) EmailService {
	return &emailService{
		smtpHost:    host,
		smtpPort:    port,
		senderEmail: email,
		senderPass:  password,
		senderName:  name,
	}
}

// EmailMessage represents an email to be sent
type EmailMessage struct {
	To      []string
	Cc      []string // CC recipients
	Subject string
	Body    string
	IsHTML  bool
}

// SendEmail sends an email using SMTP (internal helper)
// SendEmail sends an email using SMTP (internal helper)
func (s *emailService) sendEmail(msg EmailMessage) error {
	if s.smtpHost == "" || s.smtpHost == "mock" || s.smtpHost == "localhost" {
		log.Printf("[MOCK EMAIL] To: %v, Subject: %s", msg.To, msg.Subject)
		return nil
	}

	// Build email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", s.senderName, s.senderEmail)
	headers["To"] = strings.Join(msg.To, ", ")
	if len(msg.Cc) > 0 {
		headers["Cc"] = strings.Join(msg.Cc, ", ")
	}
	headers["Subject"] = msg.Subject

	if msg.IsHTML {
		headers["MIME-Version"] = "1.0"
		headers["Content-Type"] = "text/html; charset=UTF-8"
	}

	// Construct message
	var emailBody strings.Builder
	for k, v := range headers {
		emailBody.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	emailBody.WriteString("\r\n")
	emailBody.WriteString(msg.Body)

	// Combine To and Cc for SMTP recipients
	recipients := append([]string{}, msg.To...)
	recipients = append(recipients, msg.Cc...)

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.senderEmail, s.senderPass, s.smtpHost)

	// TLS Configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         s.smtpHost,
	}

	var client *smtp.Client
	var err error

	if s.smtpPort == "465" {
		// Implicit TLS (SMTPS)
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to dial TLS: %v", err)
		}
		client, err = smtp.NewClient(conn, s.smtpHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %v", err)
		}
	} else {
		// STARTTLS (usually port 587)
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP: %v", err)
		}

		// Perform STARTTLS if supported/needed
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return fmt.Errorf("failed to start TLS: %v", err)
			}
		}
	}
	defer client.Quit()

	// Authenticate
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	// Set sender and recipients
	if err = client.Mail(s.senderEmail); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}
	for _, recipient := range recipients {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %v", recipient, err)
		}
	}

	// Send body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %v", err)
	}
	_, err = w.Write([]byte(emailBody.String()))
	if err != nil {
		return fmt.Errorf("failed to write email body: %v", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %v", err)
	}

	return nil
}

func (s *emailService) SendInvitation(ctx context.Context, email, name, token string, orgName string, ccEmail string) error {
	subject := fmt.Sprintf("Invitation to join %s", orgName)
	body := fmt.Sprintf("Hello %s,\n\nYou have been invited to join %s.\nYour invitation code is: %s\n\nPlease use this code to sign up.", name, orgName, token)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendAccountStatusNotification(ctx context.Context, email, name, orgName, status, reason string) error {
	subject := fmt.Sprintf("Account Status Update for %s", orgName)
	body := fmt.Sprintf("Hello %s,\n\nYour account status in %s has been updated to: %s.\nReason: %s", name, orgName, status, reason)
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

// Rental notifications

func (s *emailService) SendRentalRequestNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error {
	subject := fmt.Sprintf("New Rental Request for %s", toolName)
	body := fmt.Sprintf("Hello,\n\n%s has requested to rent your tool: %s.\nPlease log in to approve or reject the request.", renterName, toolName)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{ownerEmail},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalApprovalNotification(ctx context.Context, renterEmail, toolName, ownerName, pickupNote string, ccEmail string) error {
	subject := fmt.Sprintf("Rental Request Approved: %s", toolName)
	body := fmt.Sprintf("Hello,\n\nYour rental request for %s has been approved by %s.\n\nPickup Instructions:\n%s", toolName, ownerName, pickupNote)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{renterEmail},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalRejectionNotification(ctx context.Context, renterEmail, toolName, ownerName string, ccEmail string) error {
	subject := fmt.Sprintf("Rental Request Rejected: %s", toolName)
	body := fmt.Sprintf("Hello,\n\nYour rental request for %s has been rejected by %s.", toolName, ownerName)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{renterEmail},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalConfirmationNotification(ctx context.Context, ownerEmail, renterName, toolName string, ccEmail string) error {
	subject := fmt.Sprintf("Rental Confirmed: %s", toolName)
	body := fmt.Sprintf("Hello,\n\n%s has confirmed the rental for %s. The transaction is now scheduled.", renterName, toolName)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{ownerEmail},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalCancellationNotification(ctx context.Context, ownerEmail, renterName, toolName, reason string, ccEmail string) error {
	subject := fmt.Sprintf("Rental Canceled: %s", toolName)
	body := fmt.Sprintf("Hello,\n\n%s has canceled the rental request for %s.\nReason: %s", renterName, toolName, reason)
	var cc []string
	if ccEmail != "" {
		cc = []string{ccEmail}
	}
	return s.sendEmail(EmailMessage{
		To:      []string{ownerEmail},
		Cc:      cc,
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalCompletionNotification(ctx context.Context, email, role, toolName string, amount int32) error {
	subject := fmt.Sprintf("Rental Completed: %s", toolName)
	body := fmt.Sprintf("Hello,\n\nThe rental for %s has been completed.\nAmount: %d cents\nRole: %s", toolName, amount, role)
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendRentalPickupNotification(ctx context.Context, email, name, toolName, startDate, endDate string) error {
	subject := fmt.Sprintf("Rental Picked Up: %s", toolName)
	body := fmt.Sprintf("Hello %s,\n\nThe tool %s has been picked up.\nStart Date: %s\nScheduled End Date: %s", name, toolName, startDate, endDate)
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendReturnDateRejectionNotification(ctx context.Context, renterEmail, toolName, newEndDate, reason string, totalCostCents int32) error {
	subject := fmt.Sprintf("Return Date Extension Rejected: %s", toolName)
	costInDollars := float64(totalCostCents) / 100.0
	body := fmt.Sprintf("Hello,\n\nYour request to extend the return date for %s has been rejected.\n\nRejection Reason: %s\nNew Return Date Set by Owner: %s\nUpdated Rental Cost: $%.2f\n\nPlease acknowledge this change to continue.",
		toolName, reason, newEndDate, costInDollars)
	return s.sendEmail(EmailMessage{
		To:      []string{renterEmail},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	return s.sendEmail(EmailMessage{
		To:      []string{adminEmail},
		Subject: subject,
		Body:    message,
		IsHTML:  false,
	})
}

// Bill Split Email Methods

func (s *emailService) SendBillPaymentNotice(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error {
	subject := fmt.Sprintf("Payment Notice: $%.2f Due to %s (%s)", float64(amountCents)/100, creditorName, orgName)
	body := fmt.Sprintf("Hello %s,\n\nYou have a payment due for the %s settlement period.\n\nAmount: $%.2f\nPayable to: %s\nOrganization: %s\n\nPlease settle this payment using your mutually agreed-upon payment method, then acknowledge the payment in the app.\n\nBest regards,\nUbertool Team",
		debtorName, settlementMonth, float64(amountCents)/100, creditorName, orgName)
	return s.sendEmail(EmailMessage{
		To:      []string{debtorEmail},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendBillPaymentAcknowledgment(ctx context.Context, creditorEmail, creditorName, debtorName string, amountCents int32, settlementMonth string, orgName string) error {
	subject := fmt.Sprintf("Payment Acknowledgment: %s sent $%.2f (%s)", debtorName, float64(amountCents)/100, orgName)
	body := fmt.Sprintf("Hello %s,\n\n%s has acknowledged sending you a payment for the %s settlement period.\n\nAmount: $%.2f\nOrganization: %s\n\nPlease confirm receipt of this payment in the app once you have received it.\n\nBest regards,\nUbertool Team",
		creditorName, debtorName, settlementMonth, float64(amountCents)/100, orgName)
	return s.sendEmail(EmailMessage{
		To:      []string{creditorEmail},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendBillReceiptConfirmation(ctx context.Context, debtorEmail, debtorName, creditorName string, amountCents int32, settlementMonth string, orgName string) error {
	subject := fmt.Sprintf("Receipt Confirmed: %s received $%.2f (%s)", creditorName, float64(amountCents)/100, orgName)
	body := fmt.Sprintf("Hello %s,\n\n%s has confirmed receiving your payment for the %s settlement period.\n\nAmount: $%.2f\nOrganization: %s\n\nYour account balances have been updated accordingly.\n\nBest regards,\nUbertool Team",
		debtorName, creditorName, settlementMonth, float64(amountCents)/100, orgName)
	return s.sendEmail(EmailMessage{
		To:      []string{debtorEmail},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendBillDisputeNotification(ctx context.Context, email, name, otherPartyName string, amountCents int32, reason string, orgName string) error {
	subject := fmt.Sprintf("Payment Dispute Opened: $%.2f with %s (%s)", float64(amountCents)/100, otherPartyName, orgName)
	body := fmt.Sprintf("Hello %s,\n\nA payment dispute has been opened for a $%.2f transaction with %s.\n\nReason: %s\nOrganization: %s\n\nPlease work with the other party to resolve this dispute. If the dispute cannot be resolved, an admin may need to intervene.\n\nBest regards,\nUbertool Team",
		name, float64(amountCents)/100, otherPartyName, reason, orgName)
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}

func (s *emailService) SendBillDisputeResolutionNotification(ctx context.Context, email, name string, amountCents int32, resolution, notes string, orgName string) error {
	subject := fmt.Sprintf("Dispute Resolved: $%.2f Payment (%s)", float64(amountCents)/100, orgName)
	body := fmt.Sprintf("Hello %s,\n\nThe dispute for a $%.2f payment has been resolved by an admin.\n\nResolution: %s\nNotes: %s\nOrganization: %s\n\nPlease check the app for details and any actions you may need to take.\n\nBest regards,\nUbertool Team",
		name, float64(amountCents)/100, resolution, notes, orgName)
	return s.sendEmail(EmailMessage{
		To:      []string{email},
		Subject: subject,
		Body:    body,
		IsHTML:  false,
	})
}
