package main

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// ComposeEmail builds the message telling pair.Giver who they're buying for.
// SUBJECT and MESSAGE may use {santa}, {santee} and {year} (in TIMEZONE).
func ComposeEmail(cfg *Config, pair Pair, now time.Time) []byte {
	fill := strings.NewReplacer(
		"{santa}", pair.Giver.Name,
		"{santee}", pair.Receiver.Name,
		"{year}", strconv.Itoa(now.In(cfg.Location).Year()),
	).Replace
	to := mail.Address{Name: pair.Giver.Name, Address: pair.Giver.Email}

	var b strings.Builder
	fmt.Fprintf(&b, "Date: %s\r\n", now.In(cfg.Location).Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-Id: <%s@secret-santa>\r\n", rand.Text())
	fmt.Fprintf(&b, "From: %s\r\n", cfg.From.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", fill(cfg.Subject)))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(fill(cfg.Message) + "\n")
	return []byte(b.String())
}

type Mailer interface {
	Send(from, to string, msg []byte) error
	Close() error
}

// smtpMailer keeps one authenticated connection open for the whole batch.
type smtpMailer struct{ c *smtp.Client }

// DialSMTP connects with STARTTLS (port 587, as gmail uses) and logs in.
func DialSMTP(cfg *Config) (Mailer, error) {
	c, err := smtp.Dial(net.JoinHostPort(cfg.SMTPServer, strconv.Itoa(cfg.SMTPPort)))
	if err != nil {
		return nil, err
	}
	if err := c.StartTLS(&tls.Config{ServerName: cfg.SMTPServer}); err != nil {
		c.Close()
		return nil, err
	}
	if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPServer)); err != nil {
		c.Close()
		return nil, err
	}
	return smtpMailer{c}, nil
}

func (m smtpMailer) Send(from, to string, msg []byte) error {
	if err := m.c.Mail(from); err != nil {
		return err
	}
	if err := m.c.Rcpt(to); err != nil {
		return err
	}
	w, err := m.c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

func (m smtpMailer) Close() error { return m.c.Quit() }
