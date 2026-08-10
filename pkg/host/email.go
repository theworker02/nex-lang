package host

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// SendEmail delivers a plain-text message via SMTP when configured; otherwise logs to the host logger.
func (h *Host) SendEmail(to, subject, body string) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	if to == "" {
		return fmt.Errorf("email recipient required")
	}

	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASSWORD")
	if port == "" {
		port = "587"
	}
	if from == "" {
		from = "noreply@localhost"
	}

	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	if host == "" {
		h.Logger.Info("email (console)",
			"to", to,
			"subject", subject,
			"body", body,
		)
		fmt.Printf("\n===== EMAIL (no SMTP configured) =====\nTo: %s\nSubject: %s\n\n%s\n===== END EMAIL =====\n\n", to, subject, body)
		return nil
	}

	addr := net.JoinHostPort(host, port)
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
	if err != nil {
		h.Logger.Error("smtp send failed", "to", to, "error", err)
		return err
	}
	h.Logger.Info("email sent", "to", to, "subject", subject, "at", time.Now().UTC().Format(time.RFC3339))
	return nil
}
