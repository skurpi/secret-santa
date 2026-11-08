package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"
)

type sent struct {
	to  string
	msg string
}

type fakeMailer struct {
	sent   []sent
	failOn string
	closed bool
}

func (f *fakeMailer) Send(from, to string, msg []byte) error {
	if to == f.failOn {
		return errors.New("connection reset")
	}
	f.sent = append(f.sent, sent{to, string(msg)})
	return nil
}

func (f *fakeMailer) Close() error { f.closed = true; return nil }

type harness struct {
	app    App
	mailer *fakeMailer
	dialed bool
	files  map[string]string
	out    strings.Builder
	errs   strings.Builder
}

func newHarness(config string) *harness {
	h := &harness{mailer: &fakeMailer{}, files: map[string]string{}}
	if config != "" {
		h.files["config.yml"] = config
	}
	h.app = App{
		Stdout: &h.out,
		Stderr: &h.errs,
		ReadFile: func(name string) ([]byte, error) {
			if s, ok := h.files[name]; ok {
				return []byte(s), nil
			}
			return nil, fs.ErrNotExist
		},
		WriteFile: func(name string, data []byte) error {
			if h.files["fail-writes"] != "" {
				return errors.New("read-only file system")
			}
			h.files[name] = string(data)
			return nil
		},
		Rand: seeded(),
		Dial: func(*Config) (Mailer, error) { h.dialed = true; return h.mailer, nil },
		Now:  func() time.Time { return december },
	}
	return h
}

func TestDryRunShowsADrawAndSendsNothing(t *testing.T) {
	h := newHarness(sampleConfig)
	if code := h.app.Run(nil); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errs.String())
	}
	if h.dialed {
		t.Fatal("dry run connected to the mail server")
	}
	for _, name := range []string{"Ann", "Bob", "Cat", "Dan", "Eve"} {
		if strings.Count(h.out.String(), name+" -> ") != 1 {
			t.Errorf("%s should appear once as a giver:\n%s", name, h.out.String())
		}
	}
	if !strings.Contains(h.out.String(), "--send") {
		t.Error("dry run should explain how to send for real")
	}
}

func TestWarningsGoToStderr(t *testing.T) {
	h := newHarness(sampleConfig)
	h.app.Run(nil)
	if !strings.Contains(h.errs.String(), "Sal") {
		t.Errorf("stderr = %q", h.errs.String())
	}
}

func TestSendEmailsEveryGiverTheirOwnMatch(t *testing.T) {
	h := newHarness(sampleConfig)
	if code := h.app.Run([]string{"--send"}); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errs.String())
	}
	if len(h.mailer.sent) != 5 || !h.mailer.closed {
		t.Fatalf("sent %d emails, closed=%v", len(h.mailer.sent), h.mailer.closed)
	}
	receivers := map[string]int{}
	for _, s := range h.mailer.sent {
		if !strings.Contains(s.msg, "<"+s.to+">") {
			t.Errorf("email to %s isn't addressed to them", s.to)
		}
		for _, n := range []string{"Ann", "Bob", "Cat", "Dan", "Eve"} {
			if strings.Contains(s.msg, "This year you have "+n) {
				receivers[n]++
			}
		}
	}
	if len(receivers) != 5 {
		t.Errorf("each person should be someone's match exactly once: %v", receivers)
	}
}

func TestSendKeepsTheDrawSecretFromTheOrganiser(t *testing.T) {
	h := newHarness(sampleConfig)
	h.app.Run([]string{"--send"})
	if strings.Contains(h.out.String(), "->") {
		t.Fatalf("--send output reveals pairings:\n%s", h.out.String())
	}
	if !strings.Contains(h.out.String(), "ann@example.com") {
		t.Errorf("should list who was emailed:\n%s", h.out.String())
	}
}

func TestFailedSendSaysWhoAlreadyGotTheirEmail(t *testing.T) {
	h := newHarness(sampleConfig)
	h.mailer.failOn = "cat@example.com"
	if code := h.app.Run([]string{"--send"}); code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(h.errs.String(), "cat@example.com") || !strings.Contains(h.errs.String(), "connection reset") {
		t.Errorf("stderr = %q", h.errs.String())
	}
	if !h.mailer.closed {
		t.Error("connection left open")
	}
}

func TestMissingConfigPointsAtTheTemplate(t *testing.T) {
	h := newHarness("")
	if code := h.app.Run(nil); code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(h.errs.String(), "config.yml.template") {
		t.Errorf("stderr = %q", h.errs.String())
	}
}

func TestBadFlag(t *testing.T) {
	h := newHarness(sampleConfig)
	if code := h.app.Run([]string{"--sned"}); code != 2 {
		t.Fatalf("exit %d", code)
	}
}

// --- History: this year's pairs are saved so next year can avoid them ---

func TestSendSavesThisYearsPairs(t *testing.T) {
	h := newHarness(sampleConfig)
	if code := h.app.Run([]string{"--send"}); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errs.String())
	}
	saved, ok := h.files["history/2026.yml"]
	if !ok {
		t.Fatalf("nothing saved; files: %v", h.files)
	}
	// Every emailed pairing is in the file, as "Giver, Receiver".
	for _, s := range h.mailer.sent {
		giver := between(s.msg, "Dear ", ",")
		receiver := between(s.msg, "This year you have ", "\n")
		if !strings.Contains(saved, "- "+giver+", "+receiver+"\n") {
			t.Errorf("history is missing %s, %s:\n%s", giver, receiver, saved)
		}
	}
}

func TestDryRunSavesNothing(t *testing.T) {
	h := newHarness(sampleConfig)
	h.app.Run(nil)
	if _, ok := h.files["history/2026.yml"]; ok {
		t.Fatal("dry run wrote history")
	}
}

func TestNoEmailsIfHistoryCantBeSaved(t *testing.T) {
	h := newHarness(sampleConfig)
	h.files["fail-writes"] = "yes"
	if code := h.app.Run([]string{"--send"}); code == 0 {
		t.Fatal("expected failure")
	}
	if h.dialed || len(h.mailer.sent) > 0 {
		t.Fatal("emails went out without a saved record")
	}
}

func TestLastYearsSavedPairsAreAvoided(t *testing.T) {
	h := newHarness(sampleConfig)
	h.files["history/2025.yml"] = "PAIRS:\n  - Bob, Dan\n  - Eve, Ann\n"
	for range 300 {
		h.out.Reset()
		if code := h.app.Run(nil); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errs.String())
		}
		for _, banned := range []string{"Bob -> Dan", "Eve -> Ann"} {
			if strings.Contains(h.out.String(), banned) {
				t.Fatalf("drew %s, which happened last year", banned)
			}
		}
	}
}

func TestSavedHistoryRoundTrips(t *testing.T) {
	h := newHarness(sampleConfig)
	h.app.Run([]string{"--send"})
	pairs, err := ParseHistory([]byte(h.files["history/2026.yml"]))
	if err != nil || len(pairs) != 5 {
		t.Fatalf("got %v, %v", pairs, err)
	}
}

func between(s, start, end string) string {
	_, after, _ := strings.Cut(s, start)
	before, _, _ := strings.Cut(after, end)
	return before
}

// --- AVOID_YEARS: how many past years of saved draws to avoid ---

func withAvoidYears(n string) string {
	return sampleConfig + "AVOID_YEARS: " + n + "\n"
}

// Feasible for the sample's five people: Ann can still give to Dan or Eve.
func seedHistory(h *harness) {
	h.files["history/2025.yml"] = "PAIRS:\n  - Bob, Dan\n  - Eve, Ann\n"
	h.files["history/2024.yml"] = "PAIRS:\n  - Cat, Eve\n  - Dan, Bob\n"
	h.files["history/2023.yml"] = "PAIRS:\n  - Bob, Cat\n"
}

func TestAvoidsTheLastTwoYearsByDefault(t *testing.T) {
	h := newHarness(sampleConfig)
	seedHistory(h)
	for range 300 {
		h.out.Reset()
		h.errs.Reset()
		if code := h.app.Run(nil); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errs.String())
		}
		for _, banned := range []string{"Bob -> Dan", "Eve -> Ann", "Cat -> Eve", "Dan -> Bob"} {
			if strings.Contains(h.out.String(), banned) {
				t.Fatalf("drew %s, which happened in the last two years", banned)
			}
		}
	}
	if strings.Contains(h.errs.String(), "2023") {
		t.Errorf("default should only look back two years: %s", h.errs.String())
	}
}

func TestAvoidYearsSetting(t *testing.T) {
	cases := map[string]struct{ reads, skips []string }{
		"0": {nil, []string{"2025", "2024", "2023"}},
		"1": {[]string{"2025"}, []string{"2024", "2023"}},
		"3": {[]string{"2025", "2024", "2023"}, nil},
	}
	for n, c := range cases {
		t.Run("AVOID_YEARS="+n, func(t *testing.T) {
			h := newHarness(withAvoidYears(n))
			seedHistory(h)
			if code := h.app.Run(nil); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errs.String())
			}
			for _, y := range c.reads {
				if !strings.Contains(h.errs.String(), "history/"+y+".yml") {
					t.Errorf("didn't avoid %s: %s", y, h.errs.String())
				}
			}
			for _, y := range c.skips {
				if strings.Contains(h.errs.String(), y) {
					t.Errorf("shouldn't look at %s: %s", y, h.errs.String())
				}
			}
		})
	}
}

func TestMissingYearsAreFine(t *testing.T) {
	h := newHarness(withAvoidYears("5"))
	h.files["history/2024.yml"] = "PAIRS:\n  - Cat, Eve\n"
	if code := h.app.Run(nil); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errs.String())
	}
}

func TestTooManyYearsSaysWhichSettingToLoosen(t *testing.T) {
	h := newHarness(withAvoidYears("3"))
	// Ann can't have Bob (partner) or Cat (LAST_YEAR); history rules out the rest.
	h.files["history/2025.yml"] = "PAIRS:\n  - Ann, Dan\n"
	h.files["history/2024.yml"] = "PAIRS:\n  - Ann, Eve\n"
	if code := h.app.Run(nil); code == 0 {
		t.Fatal("expected an impossible draw")
	}
	if !strings.Contains(h.errs.String(), "AVOID_YEARS") {
		t.Errorf("error should mention AVOID_YEARS: %s", h.errs.String())
	}
}

func TestAvoidYearsCantBeNegative(t *testing.T) {
	if _, _, err := ParseConfig([]byte(withAvoidYears("-1"))); err == nil || !strings.Contains(err.Error(), "AVOID_YEARS") {
		t.Fatalf("error = %v", err)
	}
}

// The whole draw happens before anything is saved or sent, so a draw that
// can't be completed never emails anyone.
func TestFailedDrawSendsNothingAndSavesNothing(t *testing.T) {
	h := newHarness(withAvoidYears("3"))
	// Ann can't have Bob (partner) or Cat (LAST_YEAR); history rules out the rest.
	h.files["history/2025.yml"] = "PAIRS:\n  - Ann, Dan\n"
	h.files["history/2024.yml"] = "PAIRS:\n  - Ann, Eve\n"
	if code := h.app.Run([]string{"--send"}); code == 0 {
		t.Fatal("expected the draw to fail")
	}
	if h.dialed || len(h.mailer.sent) > 0 {
		t.Fatal("connected to the mail server even though the draw failed")
	}
	if _, ok := h.files["history/2026.yml"]; ok {
		t.Fatal("saved a draw that was never completed")
	}
}
