package main

import (
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// history/<year>.yml records who gave to whom, so next year's draw can avoid
// repeats without digging through the Sent folder. It's gitignored.

func historyPath(dir string, year int) string {
	if dir == "" || dir == "." {
		return fmt.Sprintf("history/%d.yml", year)
	}
	return fmt.Sprintf("%s/history/%d.yml", dir, year)
}

func FormatHistory(pairs []Pair, sent time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# SPOILERS: who got whom in the draw sent %s.\n", sent.Format("2 Jan 2006"))
	b.WriteString("# Next year's draw reads this automatically so nobody gets the same person twice running.\n")
	b.WriteString("PAIRS:\n")
	for _, p := range pairs {
		fmt.Fprintf(&b, "  - %s, %s\n", p.Giver.Name, p.Receiver.Name)
	}
	return []byte(b.String())
}

func ParseHistory(data []byte) ([][2]string, error) {
	var h struct {
		Pairs []string `yaml:"PAIRS"`
	}
	if err := yaml.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	var pairs [][2]string
	for _, line := range h.Pairs {
		names := splitNames(line)
		if len(names) != 2 {
			return nil, fmt.Errorf("%q should be: Giver, Receiver", line)
		}
		pairs = append(pairs, [2]string{names[0], names[1]})
	}
	return pairs, nil
}
