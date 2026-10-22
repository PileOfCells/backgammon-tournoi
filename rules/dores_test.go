package rules_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// Les fichiers dorés de testdata, rejoués contre les fonctions pures. Ils ont été produits par le
// moteur d'avant rules (dotation, horloge, feuille) et par l'arithmétique de direction/clock.go de
// blunderDB (fin) ; ce paquet ne les réécrit pas : `go test -run TestDores . -args -update` à la
// racine, et `go test ./render -update`, le font contre les enveloppes.

func identique(t *testing.T, chemin string, v any) {
	t.Helper()
	ok, produit, err := dores.Same(chemin, v)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("%s : la règle pure ne rend pas la sortie dorée.\n--- produit ---\n%s", chemin, produit)
	}
}

func matchs(ms []dores.TimedMatch) []rules.Match {
	var out []rules.Match
	for _, m := range ms {
		out = append(out, rules.Match{ID: m.ID, Status: rules.MatchStatus(m.Status), Start: m.Start, End: m.End,
			Length: m.Length, Forfeit: m.Forfeit})
	}
	return out
}

func créneaux(bs []dores.Interval) []rules.Interval {
	var out []rules.Interval
	for _, b := range bs {
		out = append(out, rules.Interval{Start: b.Start, End: b.End})
	}
	return out
}

func TestDoresDotation(t *testing.T) {
	const chemin = "testdata/dotation.json"
	var cas []dores.PrizeCase
	if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		pool := rules.Pool(c.EntryFee, c.Players)
		dist := rules.Distributable(pool, rules.Retention{Amount: c.RetAmount, Percent: c.RetPct})
		montants := rules.PlaceAmounts(rules.PrizeScale{Percents: c.Percents, Amounts: c.Amounts}, dist)
		var cl []rules.Placement
		for _, r := range c.Ranking {
			cl = append(cl, rules.Placement{Player: rules.Ref(r.Player), Rank: r.Rank})
		}
		parts := map[string]float64{}
		for p, v := range rules.SharePrizes(cl, montants) {
			parts[string(p)] = v
		}
		c.Out = dores.PrizeOut{Pool: pool, Distributable: dist, Amounts: montants, Shares: parts}
	}
	identique(t, chemin, cas)
}

func TestDoresHorloge(t *testing.T) {
	const chemin = "testdata/horloge.json"
	var cas []dores.ClockCase
	if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		ms := matchs(c.Matches)
		p := rules.PaceAt(ms, c.Now, c.Start)
		out := dores.ClockOut{Played: p.Played, Running: p.Running, Elapsed: p.Elapsed, AvgMatch: p.AvgMatch,
			AvgPerPt: p.AvgPerPt, Slow: []string{}, Expected: map[string]time.Duration{}}
		for _, m := range ms {
			if m.Status == rules.Running && rules.IsSlow(m, c.Now, c.Factor, c.MinPerPoint) {
				out.Slow = append(out.Slow, m.ID)
			}
		}
		for _, n := range []int{1, 3, 5, 7, 9, 11, 25} {
			out.Expected[strconv.Itoa(n)] = rules.ExpectedDuration(n, c.MinPerPoint)
		}
		c.Out = out
	}
	identique(t, chemin, cas)
}

func TestDoresFin(t *testing.T) {
	const chemin = "testdata/fin.json"
	var f dores.End
	if err := dores.Read(chemin, &f); err != nil {
		t.Fatal(err)
	}
	for i := range f.PlayingTime {
		c := &f.PlayingTime[i]
		c.Out = rules.PlayingTime(matchs(c.Matches), c.Now)
	}
	for i := range f.DayOfPlay {
		c := &f.DayOfPlay[i]
		c.Out = rules.DayOfPlay(c.Start, c.Now)
	}
	for i := range f.AfterBreaks {
		c := &f.AfterBreaks[i]
		c.Out = rules.AfterBreaks(c.Now, c.Play, créneaux(c.Breaks))
	}
	for i := range f.EstimatedEnd {
		c := &f.EstimatedEnd[i]
		c.Out, c.OK = rules.EstimatedEnd(c.Now, c.Remaining, créneaux(c.Breaks))
	}
	identique(t, chemin, f)
}

func TestDoresFeuille(t *testing.T) {
	const chemin = "testdata/feuille.json"
	var cas []dores.SheetCase
	if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		var ms []rules.Match
		for _, m := range c.Matches {
			ms = append(ms, rules.Match{ID: m.ID, Phase: m.Phase, Status: rules.MatchStatus(m.Status), Start: m.Start,
				Table: m.Table, Length: m.Length, A: rules.Ref(m.A), B: rules.Ref(m.B)})
		}
		out := dores.SheetOut{Batches: [][]string{}, Rows: []string{}}
		for _, lot := range rules.Batches(ms, c.Current) {
			ids := []string{}
			for _, m := range lot {
				ids = append(ids, m.ID)
			}
			out.Batches = append(out.Batches, ids)
		}
		f := rules.PairingSheet(ms, c.Current, c.Round)
		out.Round = f.Round
		for _, m := range f.Rows {
			out.Rows = append(out.Rows, m.ID)
		}
		c.Out = out
	}
	identique(t, chemin, cas)
}
