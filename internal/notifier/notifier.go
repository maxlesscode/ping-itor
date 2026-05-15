// Package notifier sends email alerts via SMTP with STARTTLS.
package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/maxlesscode/ping-itor/internal/store"
)

// Config holds SMTP connection parameters.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
	To   string
}

// Notifier sends up/down email alerts over SMTP with STARTTLS.
type Notifier struct {
	cfg Config
}

// New creates a Notifier with the provided configuration.
func New(cfg Config) *Notifier {
	return &Notifier{cfg: cfg}
}

// SendDown sends an alert email when a monitor has been down for two
// consecutive checks.
func (n *Notifier) SendDown(ctx context.Context, m store.Monitor) error {
	subject := fmt.Sprintf("[ping-itor] DOWN: %s", m.Name)
	body := fmt.Sprintf(
		"Monitor: %s\nURL: %s\nStatus: DOWN\nTime: %s\n",
		m.Name, m.URL, time.Now().UTC().Format(time.RFC1123),
	)
	return n.send(ctx, subject, body)
}

// SendRecovery sends a recovery email when a monitor comes back up.
func (n *Notifier) SendRecovery(ctx context.Context, m store.Monitor) error {
	subject := fmt.Sprintf("[ping-itor] UP: %s", m.Name)
	body := fmt.Sprintf(
		"Monitor: %s\nURL: %s\nStatus: UP (recovered)\nTime: %s\n",
		m.Name, m.URL, time.Now().UTC().Format(time.RFC1123),
	)
	return n.send(ctx, subject, body)
}

func (n *Notifier) send(_ context.Context, subject, body string) error {
	if n.cfg.To == "" {
		slog.Warn("notifier: To address empty, skipping email", "subject", subject)
		return nil
	}

	addr := net.JoinHostPort(n.cfg.Host, strconv.Itoa(n.cfg.Port))

	// Dial plain TCP first; STARTTLS upgrades the connection.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, n.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	tlsCfg := &tls.Config{ServerName: n.cfg.Host} //nolint:gosec // standard TLS defaults
	if err = client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}

	auth := smtp.PlainAuth("", n.cfg.User, n.cfg.Pass, n.cfg.Host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err = client.Mail(n.cfg.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err = client.Rcpt(n.cfg.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		n.cfg.From, n.cfg.To, subject, body,
	)
	if _, err = fmt.Fprint(wc, msg); err != nil {
		return fmt.Errorf("smtp write message: %w", err)
	}
	if err = wc.Close(); err != nil {
		return fmt.Errorf("smtp close data writer: %w", err)
	}

	return client.Quit()
}
