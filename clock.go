package tournoi

import (
	"time"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// Le rythme d'un tournoi : enveloppes de rules (rules/prevision.go) sur l'état.

// Expected : durée attendue d'un match de n points.
func (s *State) Expected(n int) time.Duration {
	return rules.ExpectedDuration(n, s.Config.MinPerPoint)
}

// SlowMatches : matchs en cours qui dépassent `facteur` fois leur durée attendue.
func (s *State) SlowMatches(now time.Time, facteur float64) []*Match {
	var out []*Match
	for _, m := range s.Running() {
		if rules.IsSlow(m.rulesMatch(), now, facteur, s.Config.MinPerPoint) {
			out = append(out, m)
		}
	}
	return out
}

// Clock résume l'avancement : matchs joués, en cours, temps écoulé.
type Clock struct {
	Played   int           `json:"played"`
	Running  int           `json:"running"`
	Elapsed  time.Duration `json:"elapsed"`
	AvgMatch time.Duration `json:"avg_match"`  // durée moyenne observée des matchs terminés
	AvgPerPt time.Duration `json:"avg_per_pt"` // durée moyenne observée par point
}

// ClockAt calcule les indicateurs à l'instant now (rules.PaceAt).
func (s *State) ClockAt(now time.Time, start time.Time) Clock {
	p := rules.PaceAt(s.RulesMatches(), now, start)
	return Clock{Played: p.Played, Running: p.Running, Elapsed: p.Elapsed, AvgMatch: p.AvgMatch, AvgPerPt: p.AvgPerPt}
}

// RulesMatches : les matchs de l'état, dans l'ordre de lancement, tels que les règles pures les
// lisent. C'est l'entrée de rules.PlayingTime, rules.PaceAt, rules.Batches et
// rules.PairingSheet pour un hôte qui tient un State ; chaque ligne porte son libellé (Label),
// celui qu'une feuille imprime.
func (s *State) RulesMatches() []rules.Match {
	out := make([]rules.Match, 0, len(s.MatchOrder))
	for _, id := range s.MatchOrder {
		if m := s.Matches[id]; m != nil {
			out = append(out, m.rulesMatch())
		}
	}
	return out
}

func (m *Match) rulesMatch() rules.Match {
	r := rules.Match{ID: string(m.ID), Phase: m.Phase, Section: m.Section, Status: rules.MatchStatus(m.Status), Start: m.Start,
		End: m.End, Table: m.Table, Length: m.Length, A: rules.Ref(m.A), B: rules.Ref(m.B), Winner: rules.Ref(m.Winner),
		Forfeit: m.Forfeit}
	if !m.Label.Empty() {
		l := toRulesLabel(m.Label)
		r.Label = &l
	}
	return r
}

// BreakIntervals : les pauses déclarées, telles que rules.AfterBreaks et rules.EstimatedEnd les
// lisent.
func (c Config) BreakIntervals() []rules.Interval {
	out := make([]rules.Interval, len(c.Breaks))
	for i, b := range c.Breaks {
		out[i] = rules.Interval{Start: b.Start, End: b.End}
	}
	return out
}
