package service

import (
	"context"
	"fmt"
	"strconv"

	"gopkg.in/gomail.v2"
)

type emailService struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewEmailService(host, port, username, password, from string) EmailService {
	p, _ := strconv.Atoi(port)
	return &emailService{
		host:     host,
		port:     p,
		username: username,
		password: password,
		from:     from,
	}
}

func (s *emailService) SendInvitation(ctx context.Context, email, name, token string, orgName string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", email)
	m.SetHeader("Subject", fmt.Sprintf("Invitation to join %s", orgName))
	
	body := fmt.Sprintf("Hello %s,\n\nYou have been invited to join the organization: %s.\n\nPlease use the following token to complete your registration:\n\n%s\n\nBest regards,\nThe Ubertool Team", name, orgName, token)
	m.SetBody("text/plain", body)

	d := gomail.NewDialer(s.host, s.port, s.username, s.password)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email via gomail: %w", err)
	}

	return nil
}

func (s *emailService) SendAccountStatusNotification(ctx context.Context, email, name, orgName, status, reason string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", email)
	m.SetHeader("Subject", fmt.Sprintf("Account Status Update - %s", orgName))

	body := fmt.Sprintf("Hello %s,\n\nYour account status in the organization '%s' has been updated to: %s.", name, orgName, status)
	if reason != "" {
		body += fmt.Sprintf("\n\nReason: %s", reason)
	}
	body += "\n\nBest regards,\nThe Ubertool Team"

	m.SetBody("text/plain", body)

	d := gomail.NewDialer(s.host, s.port, s.username, s.password)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send account status notification: %w", err)
	}

	return nil
}
