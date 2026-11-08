// Secret santa: draws who gives to whom and emails each person their match.
package main

import (
	crand "crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// App holds everything that touches the outside world, so tests can fake it.
type App struct {
	Stdout, Stderr io.Writer
	ReadFile       func(name string) ([]byte, error)
	WriteFile      func(name string, data []byte) error
	Rand           *rand.Rand
	Dial           func(*Config) (Mailer, error)
	Now            func() time.Time
}

func main() {
	var seed [32]byte
	crand.Read(seed[:])
	app := App{
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		ReadFile: os.ReadFile,
		WriteFile: func(name string, data []byte) error {
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}
			return os.WriteFile(name, data, 0o644)
		},
		Rand: rand.New(rand.NewChaCha8(seed)),
		Dial: DialSMTP,
		Now:  time.Now,
	}
	os.Exit(app.Run(os.Args[1:]))
}

func (a App) Run(args []string) int {
	flags := flag.NewFlagSet("santa", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	send := flags.Bool("send", false, "email everyone their match (without it: dry run)")
	configPath := flags.String("config", "config.yml", "path to the config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if err := a.run(*configPath, *send); err != nil {
		fmt.Fprintln(a.Stderr, "error:", err)
		return 1
	}
	return 0
}

func (a App) run(configPath string, send bool) error {
	data, err := a.ReadFile(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s not found: copy config.yml.template to config.yml and fill it in", configPath)
	} else if err != nil {
		return err
	}
	cfg, warnings, err := ParseConfig(data)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(a.Stderr, "warning:", w)
	}

	now := a.Now().In(cfg.Location)
	dir := filepath.Dir(configPath)
	if err := a.avoidPastYears(cfg, dir, now.Year()); err != nil {
		return err
	}

	pairs, err := Draw(cfg.People, cfg.Rules.Allows, a.Rand)
	if errors.Is(err, ErrNoValidDraw) {
		return fmt.Errorf("%w.\nTry a lower AVOID_YEARS (currently %d), or check COUPLES and LAST_YEAR", err, cfg.AvoidYears)
	} else if err != nil {
		return err
	}
	if !send {
		a.showDryRun(cfg, pairs, now)
		return nil
	}

	// Save first: if that fails, nobody has been emailed yet.
	saveTo := historyPath(dir, now.Year())
	if err := a.WriteFile(saveTo, FormatHistory(pairs, now)); err != nil {
		return fmt.Errorf("couldn't save the draw to %s, so no emails were sent: %w", saveTo, err)
	}
	return a.sendAll(cfg, pairs, now, saveTo)
}

// avoidPastYears adds the saved pairs from the last AVOID_YEARS years as
// rules, on top of LAST_YEAR. Years with no saved draw are skipped.
func (a App) avoidPastYears(cfg *Config, dir string, thisYear int) error {
	var used []string
	for year := thisYear - 1; year >= thisYear-cfg.AvoidYears; year-- {
		path := historyPath(dir, year)
		data, err := a.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		pairs, err := ParseHistory(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, p := range pairs {
			cfg.Rules.Never(p[0], p[1])
		}
		used = append(used, path)
	}
	if len(used) > 0 {
		fmt.Fprintf(a.Stderr, "Avoiding pairs from %s\n", strings.Join(used, ", "))
	}
	return nil
}

func (a App) showDryRun(cfg *Config, pairs []Pair, now time.Time) {
	fmt.Fprintln(a.Stdout, "Sample draw (nothing sent; --send makes a fresh draw):")
	fmt.Fprintln(a.Stdout)
	for _, p := range pairs {
		fmt.Fprintf(a.Stdout, "  %s -> %s\n", p.Giver.Name, p.Receiver.Name)
	}
	fmt.Fprintln(a.Stdout)
	fmt.Fprintf(a.Stdout, "Example email to %s:\n\n", pairs[0].Giver.Name)
	for line := range strings.Lines(string(ComposeEmail(cfg, pairs[0], now))) {
		fmt.Fprint(a.Stdout, "  | "+strings.TrimRight(line, "\r\n")+"\n")
	}
	fmt.Fprintln(a.Stdout)
	fmt.Fprintln(a.Stdout, "To send for real:  docker compose run --rm --build santa --send")
}

// sendAll prints who was emailed, never who they got: the organiser is
// usually taking part too.
func (a App) sendAll(cfg *Config, pairs []Pair, now time.Time, savedTo string) error {
	mailer, err := a.Dial(cfg)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w (nobody was emailed)", cfg.SMTPServer, err)
	}
	defer mailer.Close()
	var done []string
	for _, p := range pairs {
		if err := mailer.Send(cfg.From.Address, p.Giver.Email, ComposeEmail(cfg, p, now)); err != nil {
			already := "nobody"
			if len(done) > 0 {
				already = strings.Join(done, ", ")
			}
			return fmt.Errorf("emailing %s <%s>: %w\nAlready emailed: %s\n"+
				"Run --send again for a fresh draw and tell those people to ignore the first email",
				p.Giver.Name, p.Giver.Email, err, already)
		}
		done = append(done, p.Giver.Name)
		fmt.Fprintf(a.Stdout, "Emailed %s <%s>\n", p.Giver.Name, p.Giver.Email)
	}
	fmt.Fprintf(a.Stdout, "\nAll %d emails sent. Draw saved to %s (spoilers!).\n", len(pairs), savedTo)
	return nil
}
