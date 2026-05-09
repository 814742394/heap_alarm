package alert

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"out_heap_alarm_go/config"
)

// AlertCooldown tracks the last alert time and prevents repeated alerts
// within a configurable cooldown period.
type AlertCooldown struct {
	mu       sync.Mutex
	cooldown time.Duration
	lastSent time.Time
}

// NewAlertCooldown creates a new cooldown manager.
// cooldownSec is the minimum interval (in seconds) between consecutive alerts.
func NewAlertCooldown(cooldownSec int) *AlertCooldown {
	return &AlertCooldown{
		cooldown: time.Duration(cooldownSec) * time.Second,
	}
}

// TrySend returns true if the alert may be sent (not in cooldown period).
func (a *AlertCooldown) TrySend() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if time.Since(a.lastSent) < a.cooldown {
		return false
	}
	a.lastSent = time.Now()
	return true
}

// Reset clears the cooldown state. Call when memory returns below threshold.
func (a *AlertCooldown) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastSent = time.Time{}
}

// PerProcessCooldown tracks cooldowns per PID, so multiple instances
// of the same process name are tracked independently.
type PerProcessCooldown struct {
	mu        sync.Mutex
	cooldown  time.Duration
	cooldowns map[int32]*AlertCooldown
}

// NewPerProcessCooldown creates a new per-PID cooldown manager.
func NewPerProcessCooldown(cooldownSec int) *PerProcessCooldown {
	return &PerProcessCooldown{
		cooldown:  time.Duration(cooldownSec) * time.Second,
		cooldowns: make(map[int32]*AlertCooldown),
	}
}

// TrySend returns true if an alert may be sent for the given PID.
func (p *PerProcessCooldown) TrySend(pid int32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	c, ok := p.cooldowns[pid]
	if !ok {
		c = &AlertCooldown{cooldown: p.cooldown}
		p.cooldowns[pid] = c
	}

	if time.Since(c.lastSent) < c.cooldown {
		return false
	}
	c.lastSent = time.Now()
	return true
}

// Reset clears the cooldown for a specific PID. Also removes the entry
// so the map doesn't grow indefinitely for dead processes.
func (p *PerProcessCooldown) Reset(pid int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cooldowns, pid)
}

// SendMail sends an email via the configured SMTP server.
// Port 465 uses SMTPS (implicit TLS) — requires a direct TLS connection.
// Port 587/25 uses STARTTLS — handled by smtp.SendMail.
func SendMail(cfg *config.SMTPConfig, subject, body string) error {
	msg := buildMessage(cfg.From, cfg.To, subject, body)

	if cfg.Port == 465 {
		return sendSMTPS(cfg, msg)
	}
	return sendSTARTTLS(cfg, msg)
}

// sendSMTPS connects directly over TLS (port 465 / SMTPS).
func sendSMTPS(cfg *config.SMTPConfig, msg string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	tlsConfig := &tls.Config{
		ServerName: cfg.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("EHLO: %w", err)
	}

	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("AUTH: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, recipient := range cfg.To {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return client.Quit()
}

// sendSTARTTLS uses smtp.SendMail which handles STARTTLS upgrade (port 587/25).
func sendSTARTTLS(cfg *config.SMTPConfig, msg string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(msg))
}

// buildMessage constructs the full RFC 822 email message.
func buildMessage(from string, to []string, subject, body string) string {
	headers := map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// DialFunc is exported so tests can inject a mock.
type DialFunc func(network, addr string) (net.Conn, error)
