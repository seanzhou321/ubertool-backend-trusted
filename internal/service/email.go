package service

import (
	"context"
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
func (s *emailService) sendEmail(msg EmailMessage) error {
	if s.smtpHost == "" || s.smtpHost == "mock" || s.smtpHost == "localhost" {
		log.Printf("[MOCK EMAIL] To: %v, Subject: %s", msg.To, msg.Subject)
		return nil
	}
	auth := smtp.PlainAuth("", s.senderEmail, s.senderPass, s.smtpHost)

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

	// Send email
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	return smtp.SendMail(addr, auth, s.senderEmail, recipients, []byte(emailBody.String()))
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

func (s *emailService) SendAdminNotification(ctx context.Context, adminEmail, subject, message string) error {
	return s.sendEmail(EmailMessage{
		To:      []string{adminEmail},
		Subject: subject,
		Body:    message,
		IsHTML:  false,
	})
}
