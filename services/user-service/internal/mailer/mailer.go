package mailer

import (
	"context"
	"sync"

	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
)

type Mailer struct {
	host     string
	port     string
	username string
	password string
	from     string
	baseURL  string
}

func New(host, port, username, password, from, baseURL string) *Mailer {
	if from == "" {
		from = username
	}
	return &Mailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		baseURL:  baseURL,
	}
}

type loginAuth struct{ username, password string }

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch string(fromServer) {
	case "Username:":
		return []byte(a.username), nil
	case "Password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
	}
}

// Mail metrics — same lazy-init pattern as shared/tracing kafka metrics.
// Prometheus sees: mail_send_total{status="ok|error"}. Der Tiago-Incident
// (Mail-Fehler nur als warn-Log, 3 gestapelte Ursachen unbemerkt) ist der
// Grund: Fehler muessen zaehlbar + alarmierbar sein.
var (
	mailMetricsOnce sync.Once
	mailSends       metric.Int64Counter
)

func recordMailSend(err error) {
	mailMetricsOnce.Do(func() {
		meter := otel.Meter("drova/mailer")
		mailSends, _ = meter.Int64Counter("mail.send.total",
			metric.WithDescription("Activation emails attempted (status=ok|error)"))
	})
	status := "ok"
	if err != nil {
		status = "error"
	}
	mailSends.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", status)))
}

func (m *Mailer) SendActivation(toEmail, token string) error {
	err := m.sendActivation(toEmail, token)
	recordMailSend(err)
	return err
}

func (m *Mailer) sendActivation(toEmail, token string) error {
	toEmail = strings.ReplaceAll(strings.ReplaceAll(toEmail, "\r", ""), "\n", "")
	link := fmt.Sprintf("%s/v1/users/activate/%s", m.baseURL, token)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"></head>
<body style="font-family:system-ui,sans-serif;max-width:480px;margin:40px auto;padding:0 16px;color:#111827;background:#fff;">
  <div style="border:1px solid #e5e7eb;border-radius:12px;padding:32px;">
    <h2 style="margin:0 0 8px;font-size:22px;">Welcome to Drova</h2>
    <p style="color:#6b7280;margin:0 0 28px;font-size:15px;">
      Confirm your email address to activate your account.
    </p>
    <a href="%s"
       style="display:inline-block;background:#111827;color:#fff;padding:13px 28px;
              border-radius:8px;text-decoration:none;font-weight:600;font-size:15px;">
      Confirm email
    </a>
    <p style="margin:28px 0 0;color:#9ca3af;font-size:12px;line-height:1.6;">
      This link is valid for 72 hours.<br>
      If you didn't sign up, you can safely ignore this email.
    </p>
  </div>
</body>
</html>`, link)

	subject := "Drova – Confirm your email address"
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.from, toEmail, subject, html,
	)

	addr := net.JoinHostPort(m.host, m.port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smtp_dial error: %v\n", err)
		return err
	}

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smtp_client error: %v\n", err)
		return err
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
		fmt.Fprintf(os.Stderr, "smtp_starttls error: %v\n", err)
		return err
	}
	if err := client.Auth(&loginAuth{m.username, m.password}); err != nil {
		fmt.Fprintf(os.Stderr, "smtp_auth error: %v\n", err)
		return err
	}
	if err := client.Mail(m.username); err != nil {
		return err
	}
	if err := client.Rcpt(toEmail); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
