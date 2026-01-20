package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailService struct {
	smtpHost     string
	smtpPort     string
	senderEmail  string
	senderPass   string
	senderName   string
}

func NewEmailService(host, port, email, password, name string) *EmailService {
	return &EmailService{
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
	Subject string
	Body    string
	IsHTML  bool
}

// SendEmail sends an email using SMTP
func (es *EmailService) SendEmail(msg EmailMessage) error {
	auth := smtp.PlainAuth("", es.senderEmail, es.senderPass, es.smtpHost)
	
	// Build email headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", es.senderName, es.senderEmail)
	headers["To"] = strings.Join(msg.To, ", ")
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
	
	// Send email
	addr := fmt.Sprintf("%s:%s", es.smtpHost, es.smtpPort)
	return smtp.SendMail(addr, auth, es.senderEmail, msg.To, []byte(emailBody.String()))
}

// Example usage functions for your tool sharing app

func (es *EmailService) SendToolRequestNotification(ownerEmail, requesterName, toolName string) error {
	return es.SendEmail(EmailMessage{
		To:      []string{ownerEmail},
		Subject: fmt.Sprintf("Tool Request: %s", toolName),
		Body: fmt.Sprintf(
			"Hello,\n\n%s has requested to borrow your %s.\n\nPlease log in to respond to this request.\n\nBest regards,\nTool Sharing Team",
			requesterName, toolName,
		),
		IsHTML: false,
	})
}

func (es *EmailService) SendToolApprovalNotification(requesterEmail, toolName, ownerName string) error {
	return es.SendEmail(EmailMessage{
		To:      []string{requesterEmail},
		Subject: "Tool Request Approved!",
		Body: fmt.Sprintf(
			"<html><body><h2>Great News!</h2><p>%s has approved your request to borrow their <strong>%s</strong>.</p><p>Please coordinate pickup details.</p></body></html>",
			ownerName, toolName,
		),
		IsHTML: true,
	})
}

// main.go or config initialization
/*
func main() {
	emailService := email.NewEmailService(
		"smtp.gmail.com",
		"587",
		"your-gmail@gmail.com",
		"your-app-specific-password", // Not your regular password!
		"Tool Sharing App",
	)
	
	// Test sending
	err := emailService.SendToolRequestNotification(
		"recipient@yahoo.com",
		"John Doe",
		"Power Drill",
	)
	if err != nil {
		log.Printf("Failed to send email: %v", err)
	}
}
*/