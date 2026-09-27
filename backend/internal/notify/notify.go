// Package notify sends incident emails asynchronously (PRD §13, FR-11).
// In-app notifications are stored by the repository; this package only
// handles the email channel. When no SMTP host is configured the sender is
// disabled and emails are recorded as skipped.
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/khairulazzam0001/incident-management/backend/internal/repository"
)

// Config holds SMTP settings (empty Host disables sending).
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// FromEnv builds a Config from the standard SMTP_* variables.
func FromEnv(get func(string) string) Config {
	port := 25
	if p := strings.TrimSpace(get("SMTP_PORT")); p != "" {
		var v int
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil && v > 0 {
			port = v
		}
	}
	return Config{
		Host: strings.TrimSpace(get("SMTP_HOST")),
		Port: port,
		User: strings.TrimSpace(get("SMTP_USER")),
		Pass: get("SMTP_PASS"),
		From: strings.TrimSpace(get("SMTP_FROM")),
	}
}

type job struct {
	notifID string
	to      string
	subject string
	body    string
}

// Sender delivers emails on a background goroutine with per-mail timeout.
type Sender struct {
	repo *repository.Repository
	cfg  Config
	log  *slog.Logger
	jobs chan job
}

// NewSender creates the sender and starts its loop. A nil repo disables
// status updates (used in tests).
func NewSender(repo *repository.Repository, cfg Config, log *slog.Logger) *Sender {
	s := &Sender{repo: repo, cfg: cfg, log: log, jobs: make(chan job, 100)}
	go s.loop()
	return s
}

// Enabled reports whether SMTP sending is configured.
func (s *Sender) Enabled() bool { return s != nil && s.cfg.Host != "" }

// Enqueue schedules delivery; it never blocks the request path.
func (s *Sender) Enqueue(notifID, to, subject, body string) {
	if !s.Enabled() {
		return
	}
	select {
	case s.jobs <- job{notifID: notifID, to: to, subject: subject, body: body}:
	default:
		s.log.Warn("email queue penuh, tandai gagal", "notif_id", notifID)
		if s.repo != nil {
			_ = s.repo.UpdateEmailStatus(context.Background(), notifID, "failed", "antrean email penuh")
		}
	}
}

func (s *Sender) loop() {
	for j := range s.jobs {
		if err := s.send(j); err != nil {
			s.log.Warn("gagal kirim email", "notif_id", j.notifID, "error", err)
			if s.repo != nil {
				_ = s.repo.UpdateEmailStatus(context.Background(), j.notifID, "failed", err.Error())
			}
			continue
		}
		if s.repo != nil {
			_ = s.repo.UpdateEmailStatus(context.Background(), j.notifID, "sent", "")
		}
	}
}

func (s *Sender) send(j job) error {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if s.cfg.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Pass, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	from := s.cfg.From
	if from == "" {
		from = s.cfg.User
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := client.Rcpt(j.to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, j.to, j.subject, j.body)
	if _, err := fmt.Fprint(w, msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp commit: %w", err)
	}
	return client.Quit()
}
