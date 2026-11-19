// Package mail sends the panel's emails (invitations).
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// ErrDisabled reports that no way to send email is configured.
var ErrDisabled = errors.New("email sending is not configured (EQ_SMTP_ADDR)")

// Message is a plain-text email.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender sends messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Disabled refuses to send: invitations are created but marked as not sent.
type Disabled struct{}

// Send implements Sender.
func (Disabled) Send(context.Context, Message) error { return ErrDisabled }

// Log writes messages to the log instead of sending them. Development only:
// the message contains the invitation link.
type Log struct{ Logger *slog.Logger }

// Send implements Sender.
func (l Log) Send(ctx context.Context, m Message) error {
	l.Logger.WarnContext(ctx, "email not sent (EQ_MAIL_MODE=log, development only)",
		slog.Group("detail", slog.String("to", m.To), slog.String("subject", m.Subject), slog.String("body", m.Body)))
	return nil
}

// SMTP sends through a mail server with STARTTLS (port 587) or implicit TLS (465).
type SMTP struct {
	Addr     string // host:port
	User     string
	Password string
	From     string
}

// Send implements Sender.
func (s SMTP) Send(ctx context.Context, m Message) error {
	if strings.ContainsAny(m.To+m.Subject, "\r\n") {
		return errors.New("invalid header value")
	}
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if port == "465" {
		conn, err = tls.DialWithDialer(&d, "tcp", s.Addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", s.Addr)
	}
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, host)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.From); err != nil {
		return err
	}
	if err := c.Rcpt(m.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n"+
		"Date: %s\r\n\r\n%s\r\n", s.From, m.To, m.Subject, time.Now().UTC().Format(time.RFC1123Z), strings.ReplaceAll(m.Body, "\n", "\r\n"))
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// Recorder keeps messages in memory (tests).
type Recorder struct {
	mu   sync.Mutex
	Sent []Message
}

// Send implements Sender.
func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Sent = append(r.Sent, m)
	return nil
}

// Messages returns a copy of what was sent so far.
func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.Sent...)
}
