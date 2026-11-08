package main

import (
	"errors"
	"math/rand/v2"
)

type Person struct {
	Name  string
	Email string
}

type Pair struct {
	Giver, Receiver Person
}

// Rules says who may not give to whom. Nobody may give to themselves.
type Rules struct {
	never map[[2]string]bool
}

func NewRules() Rules { return Rules{never: map[[2]string]bool{}} }

// Never forbids giver -> receiver (one direction only).
func (r Rules) Never(giver, receiver string) { r.never[[2]string{giver, receiver}] = true }

// NeverWithin forbids everyone in a household from drawing each other.
func (r Rules) NeverWithin(names ...string) {
	for _, a := range names {
		for _, b := range names {
			r.Never(a, b)
		}
	}
}

func (r Rules) Allows(giver, receiver string) bool {
	return giver != receiver && !r.never[[2]string{giver, receiver}]
}

// Plenty for any family: with 11 people and typical rules about 1 shuffle in
// 30 is valid, and this many attempts takes well under a second.
const maxAttempts = 1_000_000

// Draw returns a secret santa assignment. Every valid assignment is equally
// likely: shuffle everyone, keep the shuffle only if it breaks no rule.
// Works for any number of people >= 2, odd or even.
func Draw(people []Person, allowed func(giver, receiver string) bool, rng *rand.Rand) ([]Pair, error) {
	if len(people) < 2 {
		return nil, errors.New("need at least two participants")
	}
	receivers := make([]Person, len(people))
	copy(receivers, people)
	for range maxAttempts {
		rng.Shuffle(len(receivers), func(i, j int) { receivers[i], receivers[j] = receivers[j], receivers[i] })
		if valid(people, receivers, allowed) {
			pairs := make([]Pair, len(people))
			for i := range people {
				pairs[i] = Pair{people[i], receivers[i]}
			}
			return pairs, nil
		}
	}
	return nil, ErrNoValidDraw
}

// Returned after maxAttempts shuffles. Usually the rules are impossible, but
// rules so tight that only a few hundred draws exist can also end up here.
var ErrNoValidDraw = errors.New("couldn't find a valid draw in a million tries: the rules are too tight")

func valid(givers, receivers []Person, allowed func(string, string) bool) bool {
	for i := range givers {
		if givers[i].Name == receivers[i].Name || !allowed(givers[i].Name, receivers[i].Name) {
			return false
		}
	}
	return true
}
