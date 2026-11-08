package main

import (
	"strings"
	"testing"
)

// Mirrors the real config.yml layout, fictional people.
const sampleConfig = `
SMTP_SERVER: smtp.example.com
SMTP_PORT: 587
USERNAME: me@example.com
PASSWORD: "secret"
RANDOMSEED: 10045
TIMEZONE: 'US/Pacific'
PARTICIPANTS:
  - Ann <ann@example.com>
  - Bob <bob@example.com>
  #- Sal <sal@example.com>
  - Cat <cat@example.com>
  - Dan <dan@example.com>
  - Eve <eve@example.com>
COUPLES:
  - Ann, Bob
  - Cat, Sal
LAST_YEAR:
  - Ann, Cat
  - Sal, Bob
FROM: Secret Santa <me@example.com>
SUBJECT: Your person is...
MESSAGE:
  Dear {santa},

  This year you have {santee}
`

func parse(t *testing.T, yaml string) (*Config, []string) {
	t.Helper()
	cfg, warnings, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return cfg, warnings
}

func TestCommentedOutPersonIsNotInTheDraw(t *testing.T) {
	cfg, _ := parse(t, sampleConfig)
	var got []string
	for _, p := range cfg.People {
		got = append(got, p.Name)
	}
	if strings.Join(got, ",") != "Ann,Bob,Cat,Dan,Eve" {
		t.Fatalf("people = %v, want Ann,Bob,Cat,Dan,Eve", got)
	}
}

func TestReadsNamesAndEmails(t *testing.T) {
	cfg, _ := parse(t, sampleConfig)
	if cfg.People[1] != (Person{"Bob", "bob@example.com"}) {
		t.Fatalf("got %+v", cfg.People[1])
	}
}

func TestCouplesAndLastYearBecomeRules(t *testing.T) {
	cfg, _ := parse(t, sampleConfig)
	if cfg.Rules.Allows("Ann", "Bob") || cfg.Rules.Allows("Bob", "Ann") {
		t.Error("couple can draw each other")
	}
	if cfg.Rules.Allows("Ann", "Cat") {
		t.Error("Ann can draw last year's person")
	}
	if !cfg.Rules.Allows("Cat", "Ann") {
		t.Error("Cat should be able to draw Ann")
	}
}

func TestWarnsAboutNamesNotTakingPart(t *testing.T) {
	_, warnings := parse(t, sampleConfig)
	all := strings.Join(warnings, "\n")
	if !strings.Contains(all, "Sal") {
		t.Fatalf("expected a warning about Sal, got %q", all)
	}
}

func TestOddNumberOfPeopleCanBeDrawn(t *testing.T) {
	cfg, _ := parse(t, sampleConfig) // 5 people, one of a couple sitting out
	if _, err := Draw(cfg.People, cfg.Rules.Allows, seeded()); err != nil {
		t.Fatal(err)
	}
}

func TestMessageAndTimezone(t *testing.T) {
	cfg, _ := parse(t, sampleConfig)
	if cfg.Message != "Dear {santa},\nThis year you have {santee}" {
		t.Errorf("message = %q", cfg.Message)
	}
	if cfg.Location.String() != "US/Pacific" {
		t.Errorf("location = %v", cfg.Location)
	}
	if cfg.SMTPPort != 587 || cfg.From.Address != "me@example.com" {
		t.Errorf("smtp = %d, from = %v", cfg.SMTPPort, cfg.From)
	}
}

func TestOptionalSectionsMayBeLeftOut(t *testing.T) {
	cfg, _ := parse(t, `
SMTP_SERVER: s
SMTP_PORT: 587
USERNAME: u
PASSWORD: p
PARTICIPANTS: [A <a@x.com>, B <b@x.com>]
FROM: me@x.com
SUBJECT: s
MESSAGE: m
`)
	if cfg.Location.String() != "UTC" || len(cfg.People) != 2 {
		t.Fatalf("got %v, %d people", cfg.Location, len(cfg.People))
	}
}

func TestConfigErrors(t *testing.T) {
	cases := map[string]struct{ yaml, mentions string }{
		"missing keys listed together": {"PARTICIPANTS: [A <a@x.com>]", "SMTP_SERVER, SMTP_PORT"},
		"participant without email":    {strings.Replace(sampleConfig, "Bob <bob@example.com>", "Bob", 1), "Bob"},
		"participant without name":     {strings.Replace(sampleConfig, "Bob <bob@example.com>", "bob@example.com", 1), "bob@example.com"},
		"same name twice":              {strings.Replace(sampleConfig, "Bob <bob", "Ann <bob", 1), "Ann"},
		"last year needs two names":    {strings.Replace(sampleConfig, "Ann, Cat", "Ann", 1), "LAST_YEAR"},
		"unknown timezone":             {strings.Replace(sampleConfig, "US/Pacific", "Mars/Olympus", 1), "Mars/Olympus"},
		"not yaml":                     {"PARTICIPANTS: [", "yaml"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := ParseConfig([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.mentions) {
				t.Fatalf("error = %v, want it to mention %q", err, c.mentions)
			}
		})
	}
}
