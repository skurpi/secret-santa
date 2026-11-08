package main

import (
	"io"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func testConfig(t *testing.T) *Config {
	t.Helper()
	cfg, _ := parse(t, sampleConfig)
	return cfg
}

var december = time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)

func compose(t *testing.T, cfg *Config, pair Pair) *mail.Message {
	t.Helper()
	raw := ComposeEmail(cfg, pair, december)
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("not a valid email: %v\n%s", err, raw)
	}
	return msg
}

func TestEmailFillsInSantaAndSantee(t *testing.T) {
	cfg := testConfig(t)
	cfg.Subject = "{santa}, you have {santee}"
	msg := compose(t, cfg, Pair{Person{"Ann", "ann@example.com"}, Person{"Cat", "cat@example.com"}})

	if got := msg.Header.Get("Subject"); got != "Ann, you have Cat" {
		t.Errorf("subject = %q", got)
	}
	body, _ := io.ReadAll(msg.Body)
	if string(body) != "Dear Ann,\nThis year you have Cat\n" {
		t.Errorf("body = %q", body)
	}
}

func TestEmailHeaders(t *testing.T) {
	msg := compose(t, testConfig(t), Pair{Person{"Ann", "ann@example.com"}, Person{"Cat", "cat@example.com"}})
	want := map[string]string{
		"To":           `"Ann" <ann@example.com>`,
		"From":         `"Secret Santa" <me@example.com>`,
		"Date":         "Tue, 01 Dec 2026 12:00:00 -0800", // TIMEZONE: US/Pacific
		"Content-Type": `text/plain; charset="utf-8"`,
	}
	for k, v := range want {
		if got := msg.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if id := msg.Header.Get("Message-Id"); !strings.HasPrefix(id, "<") || !strings.Contains(id, "@") {
		t.Errorf("Message-Id = %q", id)
	}
}

func TestEmailHandlesNonEnglishNames(t *testing.T) {
	cfg := testConfig(t)
	cfg.Subject = "Your person is {santee}"
	msg := compose(t, cfg, Pair{Person{"Zoë", "z@example.com"}, Person{"José", "j@example.com"}})
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subject != "Your person is José" {
		t.Errorf("subject = %q (%v)", subject, err)
	}
	to, err := msg.Header.AddressList("To")
	if err != nil || to[0].Name != "Zoë" {
		t.Errorf("to = %v (%v)", to, err)
	}
}

func TestEmailFillsInYear(t *testing.T) {
	cfg := testConfig(t)
	cfg.Subject = "Your {year} person is..."
	cfg.Message = "Happy {year}, {santa}!"
	// 31 Dec 2026 14:00 UTC is already 1 Jan 2027 in Melbourne: the year
	// follows TIMEZONE, like the Date header does.
	cfg.Location, _ = time.LoadLocation("Australia/Melbourne")
	raw := ComposeEmail(cfg, Pair{Person{"Ann", "ann@example.com"}, Person{"Cat", "cat@example.com"}},
		time.Date(2026, 12, 31, 14, 0, 0, 0, time.UTC))
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if got := msg.Header.Get("Subject"); got != "Your 2027 person is..." {
		t.Errorf("subject = %q", got)
	}
	if body, _ := io.ReadAll(msg.Body); string(body) != "Happy 2027, Ann!\n" {
		t.Errorf("body = %q", body)
	}
}
