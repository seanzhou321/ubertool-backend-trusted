package email

import (
	"fmt"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type SendGridService struct {
	apiKey      string
	fromEmail   string
	fromName    string
}

func NewSendGridService(apiKey, fromEmail, fromName string) *SendGridService {
	return &SendGridService{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

func (s *SendGridService) SendEmail(to, toName, subject, plainText, htmlContent string) error {
	from := mail.NewEmail(s.fromName, s.fromEmail)
	recipient := mail.NewEmail(toName, to)
	
	message := mail.NewSingleEmail(from, subject, recipient, plainText, htmlContent)
	
	client := sendgrid.NewSendClient(s.apiKey)
	response, err := client.Send(message)
	
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d, body: %s", response.StatusCode, response.Body)
	}
	
	return nil
}

// Template-based sending
func (s *SendGridService) SendTemplateEmail(to, toName, templateID string, dynamicData map[string]interface{}) error {
	from := mail.NewEmail(s.fromName, s.fromEmail)
	recipient := mail.NewEmail(toName, to)
	
	message := mail.NewV3Mail()
	message.SetFrom(from)
	message.SetTemplateID(templateID)
	
	personalization := mail.NewPersonalization()
	personalization.AddTos(recipient)
	
	for key, value := range dynamicData {
		personalization.SetDynamicTemplateData(key, value)
	}
	
	message.AddPersonalizations(personalization)
	
	client := sendgrid.NewSendClient(s.apiKey)
	response, err := client.Send(message)
	
	if err != nil {
		return fmt.Errorf("failed to send template email: %w", err)
	}
	
	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d", response.StatusCode)
	}
	
	return nil
}

// Example usage for tool sharing
func (s *SendGridService) NotifyToolRequest(ownerEmail, ownerName, requesterName, toolName string) error {
	subject := fmt.Sprintf("New Tool Request: %s", toolName)
	plainText := fmt.Sprintf("%s wants to borrow your %s", requesterName, toolName)
	htmlContent := fmt.Sprintf(`
		<html>
			<body>
				<h2>New Tool Request</h2>
				<p><strong>%s</strong> has requested to borrow your <strong>%s</strong>.</p>
				<p><a href="https://yourapp.com/requests">View Request</a></p>
			</body>
		</html>
	`, requesterName, toolName)
	
	return s.SendEmail(ownerEmail, ownerName, subject, plainText, htmlContent)
}

/*
// go.mod requirement:
require github.com/sendgrid/sendgrid-go v3.14.0+incompatible

// Usage:
emailService := email.NewSendGridService(
	os.Getenv("SENDGRID_API_KEY"),
	"noreply@toolsharing.com",
	"Tool Sharing App",
)
*/