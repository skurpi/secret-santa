package main

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	_ "time/tzdata" // the container image has no zoneinfo of its own

	"go.yaml.in/yaml/v3"
)

type Config struct {
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       *mail.Address
	Subject    string
	Message    string
	Location   *time.Location
	People     []Person
	Rules      Rules
	AvoidYears int // how many past years of history/ to avoid repeating
}

const defaultAvoidYears = 2

// The on-disk format, kept compatible with the original Python version.
// Unknown keys (e.g. the old, never-implemented RANDOMSEED) are ignored.
type rawConfig struct {
	SMTPServer   string   `yaml:"SMTP_SERVER"`
	SMTPPort     int      `yaml:"SMTP_PORT"`
	Username     string   `yaml:"USERNAME"`
	Password     string   `yaml:"PASSWORD"`
	Timezone     string   `yaml:"TIMEZONE"`
	Participants []string `yaml:"PARTICIPANTS"`
	Couples      []string `yaml:"COUPLES"`
	LastYear     []string `yaml:"LAST_YEAR"`
	AvoidYears   *int     `yaml:"AVOID_YEARS"` // nil = not set
	From         string   `yaml:"FROM"`
	Subject      string   `yaml:"SUBJECT"`
	Message      string   `yaml:"MESSAGE"`
}

// ParseConfig returns the config plus warnings that shouldn't stop the draw,
// e.g. a couple whose other half is sitting this year out.
func ParseConfig(data []byte) (*Config, []string, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("config is not valid yaml: %w", err)
	}
	if err := raw.checkRequired(); err != nil {
		return nil, nil, err
	}

	cfg := &Config{
		SMTPServer: raw.SMTPServer,
		SMTPPort:   raw.SMTPPort,
		Username:   raw.Username,
		Password:   raw.Password,
		Subject:    raw.Subject,
		Message:    strings.TrimSpace(raw.Message),
		Rules:      NewRules(),
		AvoidYears: defaultAvoidYears,
	}
	if raw.AvoidYears != nil {
		if *raw.AvoidYears < 0 {
			return nil, nil, fmt.Errorf("AVOID_YEARS can't be negative (use 0 to ignore history)")
		}
		cfg.AvoidYears = *raw.AvoidYears
	}
	var err error
	if cfg.From, err = mail.ParseAddress(raw.From); err != nil {
		return nil, nil, fmt.Errorf("FROM %q: %w", raw.From, err)
	}
	if raw.Timezone == "" {
		raw.Timezone = "UTC"
	}
	if cfg.Location, err = time.LoadLocation(raw.Timezone); err != nil {
		return nil, nil, fmt.Errorf("TIMEZONE %q: %w", raw.Timezone, err)
	}

	taking := map[string]bool{}
	for _, entry := range raw.Participants {
		addr, err := mail.ParseAddress(entry)
		if err != nil || strings.TrimSpace(addr.Name) == "" {
			return nil, nil, fmt.Errorf("PARTICIPANTS: %q should look like: Name <email@example.com>", entry)
		}
		name := strings.TrimSpace(addr.Name)
		if taking[name] {
			return nil, nil, fmt.Errorf("PARTICIPANTS: %s appears twice; names must be unique", name)
		}
		taking[name] = true
		cfg.People = append(cfg.People, Person{Name: name, Email: addr.Address})
	}

	var warnings []string
	absent := func(section string, names ...string) {
		for _, n := range names {
			if !taking[n] {
				warnings = append(warnings, fmt.Sprintf("%s mentions %s, who isn't in PARTICIPANTS this year (ignored)", section, n))
			}
		}
	}
	for _, line := range raw.Couples {
		names := splitNames(line)
		absent("COUPLES", names...)
		cfg.Rules.NeverWithin(names...)
	}
	for _, line := range raw.LastYear {
		names := splitNames(line)
		if len(names) != 2 {
			return nil, nil, fmt.Errorf("LAST_YEAR: %q should be: Giver, Receiver", line)
		}
		absent("LAST_YEAR", names...)
		cfg.Rules.Never(names[0], names[1])
	}
	return cfg, warnings, nil
}

func (r rawConfig) checkRequired() error {
	var missing []string
	for _, f := range []struct {
		key   string
		empty bool
	}{
		{"SMTP_SERVER", r.SMTPServer == ""},
		{"SMTP_PORT", r.SMTPPort == 0},
		{"USERNAME", r.Username == ""},
		{"PASSWORD", r.Password == ""},
		{"PARTICIPANTS", len(r.Participants) == 0},
		{"FROM", r.From == ""},
		{"SUBJECT", r.Subject == ""},
		{"MESSAGE", r.Message == ""},
	} {
		if f.empty {
			missing = append(missing, f.key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("config is missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func splitNames(line string) []string {
	var names []string
	for _, n := range strings.Split(line, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}
