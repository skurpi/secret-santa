package main

import (
	"math"
	"math/bits"
	"math/rand/v2"
	"testing"
)

func names(ns ...string) []Person {
	ps := make([]Person, len(ns))
	for i, n := range ns {
		ps[i] = Person{Name: n, Email: n + "@example.com"}
	}
	return ps
}

func anyone(giver, receiver string) bool { return true }

func seeded() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

// Same shape as a real family draw: an odd number of people, five couples,
// one person whose partner sits this year out, everyone with a last-year match.
var family = names("Ann", "Bob", "Cat", "Dan", "Eve", "Fay", "Gus", "Hal", "Ivy", "Jon", "Kim")

var familyRules = mustRules(
	[][]string{{"Ann", "Bob"}, {"Cat", "Dan"}, {"Eve", "Fay"}, {"Gus", "Hal"}, {"Ivy", "Jon"}, {"Kim", "Zed"}},
	[][2]string{{"Ann", "Cat"}, {"Bob", "Eve"}, {"Cat", "Gus"}, {"Dan", "Ann"}, {"Eve", "Ivy"},
		{"Fay", "Bob"}, {"Gus", "Kim"}, {"Hal", "Dan"}, {"Ivy", "Fay"}, {"Jon", "Hal"}, {"Kim", "Jon"}},
)

func mustRules(groups [][]string, lastYear [][2]string) Rules {
	r := NewRules()
	for _, g := range groups {
		r.NeverWithin(g...)
	}
	for _, p := range lastYear {
		r.Never(p[0], p[1])
	}
	return r
}

func TestEveryoneGivesAndReceivesExactlyOnce(t *testing.T) {
	rng := seeded()
	for _, n := range []int{2, 3, 4, 5, 11} { // odd sizes included on purpose
		people := family[:n]
		for range 200 {
			pairs, err := Draw(people, anyone, rng)
			if err != nil {
				t.Fatalf("n=%d: %v", n, err)
			}
			gives, gets := map[string]int{}, map[string]int{}
			for _, p := range pairs {
				gives[p.Giver.Name]++
				gets[p.Receiver.Name]++
			}
			for _, p := range people {
				if gives[p.Name] != 1 || gets[p.Name] != 1 {
					t.Fatalf("n=%d: %s gives %d, receives %d", n, p.Name, gives[p.Name], gets[p.Name])
				}
			}
		}
	}
}

func TestNobodyDrawsThemselves(t *testing.T) {
	rng := seeded()
	for range 1000 {
		pairs, _ := Draw(family, anyone, rng)
		for _, p := range pairs {
			if p.Giver.Name == p.Receiver.Name {
				t.Fatalf("%s drew themselves", p.Giver.Name)
			}
		}
	}
}

func TestDrawRespectsRules(t *testing.T) {
	rng := seeded()
	for range 1000 {
		pairs, err := Draw(family, familyRules.Allows, rng)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range pairs {
			if !familyRules.Allows(p.Giver.Name, p.Receiver.Name) {
				t.Fatalf("%s -> %s breaks a rule", p.Giver.Name, p.Receiver.Name)
			}
		}
	}
}

func TestRules(t *testing.T) {
	r := mustRules([][]string{{"Ann", "Bob"}, {"Cat", "Dan", "Eve"}}, [][2]string{{"Ann", "Cat"}})
	cases := []struct {
		giver, receiver string
		want            bool
	}{
		{"Ann", "Ann", false}, // self
		{"Ann", "Bob", false}, // partner
		{"Bob", "Ann", false}, // partner, other way
		{"Ann", "Cat", false}, // last year
		{"Cat", "Ann", true},  // last year only blocks one direction
		{"Cat", "Eve", false}, // households can be bigger than two
		{"Ann", "Dan", true},
	}
	for _, c := range cases {
		if got := r.Allows(c.giver, c.receiver); got != c.want {
			t.Errorf("Allows(%s, %s) = %v, want %v", c.giver, c.receiver, got, c.want)
		}
	}
}

func TestImpossibleDrawIsAnErrorNotAHang(t *testing.T) {
	r := mustRules([][]string{{"Ann", "Bob"}}, nil)
	if _, err := Draw(names("Ann", "Bob"), r.Allows, seeded()); err == nil {
		t.Fatal("expected an error: the only two people are a couple")
	}
}

func TestNeedsAtLeastTwoPeople(t *testing.T) {
	if _, err := Draw(names("Ann"), anyone, seeded()); err == nil {
		t.Fatal("expected an error for one person")
	}
}

// The bug that started all this: the old script picked A->B->C->A one time in
// three and A->C->B->A two times in three. A fair draw is 50/50.
func TestThreePeopleIsFiftyFifty(t *testing.T) {
	rng := seeded()
	const runs = 40000
	aGivesB := 0
	for range runs {
		pairs, _ := Draw(names("A", "B", "C"), anyone, rng)
		if pairs[0].Receiver.Name == "B" {
			aGivesB++
		}
	}
	if got := float64(aGivesB) / runs; math.Abs(got-0.5) > 0.01 {
		t.Fatalf("A gives to B %.3f of the time, want 0.5", got)
	}
}

// Every valid draw equally likely, checked against exact counts. The old
// script was off by up to 9 percentage points on this family.
func TestFamilyDrawIsFair(t *testing.T) {
	rng := seeded()
	const runs = 100000
	n := len(family)
	counts := make([][]int, n)
	for i := range counts {
		counts[i] = make([]int, n)
	}
	index := map[string]int{}
	for i, p := range family {
		index[p.Name] = i
	}
	for range runs {
		pairs, _ := Draw(family, familyRules.Allows, rng)
		for _, p := range pairs {
			counts[index[p.Giver.Name]][index[p.Receiver.Name]]++
		}
	}
	want := exactFairOdds(family, familyRules.Allows)
	for i := range n {
		for j := range n {
			got := float64(counts[i][j]) / runs
			if math.Abs(got-want[i][j]) > 0.006 {
				t.Errorf("%s -> %s: %.4f, fair is %.4f", family[i].Name, family[j].Name, got, want[i][j])
			}
		}
	}
}

// exactFairOdds[i][j] is the probability i gives to j when every valid draw
// is equally likely, counted exactly over subsets of taken receivers.
func exactFairOdds(people []Person, allowed func(string, string) bool) [][]float64 {
	n := len(people)
	full := 1<<n - 1
	ok := func(i, j int) bool { return i != j && allowed(people[i].Name, people[j].Name) }
	toFinish := make([]float64, 1<<n)
	toFinish[full] = 1
	for mask := full - 1; mask >= 0; mask-- {
		i := popcount(mask)
		for j := range n {
			if mask&(1<<j) == 0 && ok(i, j) {
				toFinish[mask] += toFinish[mask|1<<j]
			}
		}
	}
	toReach := make([]float64, 1<<n)
	toReach[0] = 1
	odds := make([][]float64, n)
	for i := range odds {
		odds[i] = make([]float64, n)
	}
	for mask := range full {
		i := popcount(mask)
		for j := range n {
			if toReach[mask] > 0 && mask&(1<<j) == 0 && ok(i, j) {
				toReach[mask|1<<j] += toReach[mask]
				odds[i][j] += toReach[mask] * toFinish[mask|1<<j] / toFinish[0]
			}
		}
	}
	return odds
}

func popcount(x int) int { return bits.OnesCount(uint(x)) }
