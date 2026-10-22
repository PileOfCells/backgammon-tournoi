package tournoi_test

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// Les fichiers dorés de rules/testdata, rejoués ici contre les ENVELOPPES du moteur : la
// dotation de State, ClockAt, SlowMatches, Expected. Les tests de rules rejouent les mêmes
// fichiers contre les fonctions pures ; les deux ensemble sont la parité.
//
// `go test -run TestDores . -args -update` les réécrit, ce qui n'a de sens qu'après un
// changement de règle VOULU (une mineure au changelog des règles, ADR-0105 de blunderDB §5).

var majDores = flag.Bool("update", false, "réécrire les fichiers dorés de rules/testdata")

func comparerDore(t *testing.T, chemin string, v any) {
	t.Helper()
	if *majDores {
		if err := dores.Write(chemin, v); err != nil {
			t.Fatal(err)
		}
		return
	}
	ok, produit, err := dores.Same(chemin, v)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("%s : la sortie a changé ; une règle a bougé à entrée égale.\n--- produit ---\n%s", chemin, produit)
	}
}

func casDotation() []dores.PrizeCase {
	cl := func(ranks ...int) []dores.Placed {
		var out []dores.Placed
		for i, r := range ranks {
			out = append(out, dores.Placed{Player: fmt.Sprintf("p%02d", i+1), Rank: r})
		}
		return out
	}
	return []dores.PrizeCase{
		{Name: "affiche 50/30/20 sur 432", EntryFee: 18, Players: 24, Percents: []float64{50, 30, 20}, Ranking: cl(1, 2, 3, 4, 5)},
		{Name: "retenue en pourcentage et en montant", EntryFee: 20, Players: 37, RetAmount: 50, RetPct: 10, Percents: []float64{40, 25, 15, 10, 10}, Ranking: cl(1, 2, 3, 3, 5, 5, 5, 8)},
		{Name: "retenue plus grande que le pool", EntryFee: 5, Players: 4, RetAmount: 100, Percents: []float64{60, 40}, Ranking: cl(1, 2)},
		{Name: "montants fixes", EntryFee: 10, Players: 16, Amounts: []float64{100, 60, 40.5}, Ranking: cl(1, 2, 3, 4)},
		{Name: "tiers", EntryFee: 10, Players: 10, Percents: []float64{33.333, 33.333, 33.334}, Ranking: cl(1, 2, 3)},
		{Name: "4 × 15 % de 10 : restes égaux aux mieux classés", EntryFee: 1, Players: 10, Percents: []float64{15, 15, 15, 15}, Ranking: cl(1, 2, 3, 4)},
		{Name: "reste positif au premier", EntryFee: 7, Players: 13, RetPct: 5, Percents: []float64{45, 35, 20}, Ranking: cl(1, 2, 3)},
		{Name: "ex aequo au-delà des places dotées", EntryFee: 10, Players: 8, Percents: []float64{50, 30, 20}, Ranking: cl(1, 2, 3, 3, 3, 3)},
		{Name: "premier ex aequo", EntryFee: 10, Players: 8, Percents: []float64{60, 40}, Ranking: cl(1, 1, 3)},
		{Name: "pourcentages sous 100", EntryFee: 15, Players: 11, Percents: []float64{30, 20}, Ranking: cl(1, 2, 3)},
		{Name: "aucune dotation", EntryFee: 10, Players: 8, Ranking: cl(1, 2)},
		{Name: "aucun inscrit", EntryFee: 10, Players: 0, Percents: []float64{100}, Ranking: nil},
		{Name: "droit nul, montants fixes", Players: 6, Amounts: []float64{20, 10}, Ranking: cl(1, 1, 1)},
	}
}

// TestDoresDotation : Pool, Distributable, PrizeAmounts et Prizes.
func TestDoresDotation(t *testing.T) {
	const chemin = "rules/testdata/dotation.json"
	var cas []dores.PrizeCase
	if *majDores {
		cas = casDotation()
	} else if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		st := &tournoi.State{Config: tournoi.Config{Prizes: tournoi.PrizePool{
			EntryFee:  c.EntryFee,
			Retention: tournoi.Retention{Amount: c.RetAmount, Percent: c.RetPct},
			Sections:  map[string]tournoi.PrizeScale{tournoi.PrizeSectionAll: {Percents: c.Percents, Amounts: c.Amounts}},
		}}}
		for k := 0; k < c.Players; k++ {
			st.Order = append(st.Order, tournoi.PlayerID(fmt.Sprintf("i%d", k)))
		}
		var ranking []tournoi.Rank
		for _, r := range c.Ranking {
			ranking = append(ranking, tournoi.Rank{Player: tournoi.PlayerID(r.Player), Rank: r.Rank})
		}
		montants := st.PrizeAmounts(tournoi.PrizeSectionAll)
		parts := map[string]float64{}
		for p, v := range tournoi.Prizes(ranking, montants) {
			parts[string(p)] = v
		}
		c.Out = dores.PrizeOut{Pool: st.Pool(), Distributable: st.Distributable(), Amounts: montants, Shares: parts}
	}
	comparerDore(t, chemin, cas)
}

// casHorloge : des matchs écrits à la main pour chaque branche, et l'instantané d'un suisse
// simulé à mi-parcours.
func casHorloge(t *testing.T) []dores.ClockCase {
	z := time.FixedZone("", 3*3600)
	at := func(h, m int) time.Time { return time.Date(2026, 9, 21, h, m, 0, 0, z) }
	cas := []dores.ClockCase{
		{Name: "aucun match", Now: at(10, 0), Start: at(9, 0), MinPerPoint: 8, Factor: 1.5},
		{Name: "chaque branche", Now: at(12, 0), Start: at(9, 0), MinPerPoint: 7.3, Factor: 1.5, Matches: []dores.TimedMatch{
			{ID: "fini", Status: "finished", Start: at(9, 0), End: at(9, 50), Length: 7},
			{ID: "forfait", Status: "finished", Start: at(9, 0), End: at(9, 1), Length: 7, Forfeit: true},
			{ID: "fin_nulle", Status: "finished", Start: at(9, 0), Length: 5},
			{ID: "annule", Status: "cancelled", Start: at(9, 10), Length: 7},
			{ID: "lent", Status: "running", Start: at(10, 0), Length: 9},
			{ID: "juste", Status: "running", Start: at(10, 21), Length: 9},
			{ID: "rapide", Status: "running", Start: at(11, 30), Length: 9},
			{ID: "long", Status: "finished", Start: at(9, 55), End: at(11, 40), Length: 11},
		}},
		{Name: "facteur 1, durées non entières", Now: at(15, 7), Start: at(8, 59), MinPerPoint: 6.25, Factor: 1, Matches: []dores.TimedMatch{
			{ID: "a", Status: "finished", Start: at(9, 0), End: at(9, 33), Length: 3},
			{ID: "b", Status: "finished", Start: at(9, 1), End: at(10, 2), Length: 7},
			{ID: "c", Status: "running", Start: at(14, 20), Length: 7},
		}},
	}
	r := sim.Run(tournoi.Config{Name: "Suisse dore", MinPerPoint: 8, Tables: tournoi.Tables{Count: 12},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Lives: 2}}},
		sim.Champ(24, 6, 2, 1, 15, rand.New(rand.NewSource(7))), sim.Options{Seed: 7})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	st, err := tournoi.Replay(r.Journal[:len(r.Journal)/2])
	if err != nil {
		t.Fatal(err)
	}
	c := dores.ClockCase{Name: "suisse simulé à mi-parcours", Now: st.Last.Add(37 * time.Minute), Start: r.Journal[0].Time,
		MinPerPoint: 8, Factor: 1.2}
	for _, id := range st.MatchOrder {
		m := st.Matches[id]
		c.Matches = append(c.Matches, dores.TimedMatch{ID: string(m.ID), Status: string(m.Status), Start: m.Start,
			End: m.End, Length: m.Length, Forfeit: m.Forfeit})
	}
	return append(cas, c)
}

// TestDoresHorloge : ClockAt, SlowMatches et Expected.
func TestDoresHorloge(t *testing.T) {
	const chemin = "rules/testdata/horloge.json"
	var cas []dores.ClockCase
	if *majDores {
		cas = casHorloge(t)
	} else if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		st := &tournoi.State{Config: tournoi.Config{MinPerPoint: c.MinPerPoint}, Matches: map[tournoi.MatchID]*tournoi.Match{}}
		for _, m := range c.Matches {
			id := tournoi.MatchID(m.ID)
			st.MatchOrder = append(st.MatchOrder, id)
			st.Matches[id] = &tournoi.Match{ID: id, Status: tournoi.MatchStatus(m.Status), Start: m.Start, End: m.End,
				Length: m.Length, Forfeit: m.Forfeit}
		}
		k := st.ClockAt(c.Now, c.Start)
		out := dores.ClockOut{Played: k.Played, Running: k.Running, Elapsed: k.Elapsed, AvgMatch: k.AvgMatch,
			AvgPerPt: k.AvgPerPt, Slow: []string{}, Expected: map[string]time.Duration{}}
		for _, m := range st.SlowMatches(c.Now, c.Factor) {
			out.Slow = append(out.Slow, string(m.ID))
		}
		for _, n := range []int{1, 3, 5, 7, 9, 11, 25} {
			out.Expected[strconv.Itoa(n)] = st.Expected(n)
		}
		c.Out = out
	}
	comparerDore(t, chemin, cas)
}
