package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"

	"agent-stack/internal/models"
	"agent-stack/shared/config"
)

type Sender struct {
	config *config.EmailConfig
}

func NewSender(cfg *config.EmailConfig) *Sender {
	return &Sender{
		config: cfg,
	}
}

func (s *Sender) SendReport(report *models.EmailReport) error {
	if report == nil {
		return fmt.Errorf("report cannot be nil")
	}

	if len(report.Videos) == 0 {
		return nil // No videos to report
	}

	subject := fmt.Sprintf("YouTube Video Digest - %d Videos Worth Watching (%s)",
		report.Selected, report.Date.Format("Jan 2, 2006"))

	body, err := s.generateEmailBody(report)
	if err != nil {
		return fmt.Errorf("failed to generate email body: %w", err)
	}

	return s.SendHTML(subject, body)
}

// SendHTML sends an email with custom HTML content
func (s *Sender) SendHTML(subject, htmlBody string) error {
	return s.sendViaSMTP(subject, htmlBody)
}

func (s *Sender) sendViaSMTP(subject, body string) error {
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPServer)

	to := []string{s.config.ToEmail}
	msg := []byte(fmt.Sprintf(`To: %s
From: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

%s`, s.config.ToEmail, s.config.FromEmail, subject, body))

	addr := fmt.Sprintf("%s:%d", s.config.SMTPServer, s.config.SMTPPort)
	return smtp.SendMail(addr, auth, s.config.FromEmail, to, msg)
}

func (s *Sender) generateEmailBody(report *models.EmailReport) (string, error) {
	// Read template from external file
	templatePath := "agents/youtube-curator/email_template.html"
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read email template: %w", err)
	}

	tmpl := template.New("email").Funcs(template.FuncMap{
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mul":     func(a, b float64) float64 { return a * b },
		"float64": func(i int) float64 { return float64(i) },
	})

	tmpl, err = tmpl.Parse(string(tmplBytes))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}

	return buf.String(), nil
}
