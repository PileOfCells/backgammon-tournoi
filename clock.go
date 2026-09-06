package tournoi

import "time"

// Expected : durée attendue d'un match de n points.
func (s *State) Expected(n int) time.Duration {
	return time.Duration(s.Config.MinPerPoint * float64(n) * float64(time.Minute))
}

// SlowMatches : matchs en cours qui dépassent `facteur` fois leur durée attendue.
func (s *State) SlowMatches(now time.Time, facteur float64) []*Match {
	var out []*Match
	for _, m := range s.Running() {
		if now.Sub(m.Start) > time.Duration(facteur*float64(s.Expected(m.Length))) {
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

// ClockAt calcule les indicateurs à l'instant now.
func (s *State) ClockAt(now time.Time, start time.Time) Clock {
	c := Clock{Elapsed: now.Sub(start)}
	var total time.Duration
	pts := 0
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		switch m.Status {
		case Finished:
			if !m.Forfeit && !m.End.IsZero() {
				c.Played++
				total += m.End.Sub(m.Start)
				pts += m.Length
			}
		case Running:
			c.Running++
		}
	}
	if c.Played > 0 {
		c.AvgMatch = total / time.Duration(c.Played)
	}
	if pts > 0 {
		c.AvgPerPt = total / time.Duration(pts)
	}
	return c
}
