// Package sim joue des tournois fictifs avec le moteur tournoi : résultats tirés au sort selon
// les PR, durées de match aléatoires. Sert aux tests, à la validation des formats et à la
// prévision de fin de tournoi.
package sim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
)

// PGain : probabilité que A (PR a) batte B (PR b) en N points, formule Elo/FIBS (3 PR = 100 Elo).
func PGain(a, b float64, n int) float64 {
	d := (b - a) * 100 / 3
	return 1 / (1 + math.Pow(10, -d*math.Sqrt(float64(n))/2000))
}

// Duree : durée d'un match en minutes, loi gamma de forme 8 et de moyenne minPerPoint*n.
func Duree(n int, minPerPoint float64, rng *rand.Rand) time.Duration {
	sum := 0.0
	for i := 0; i < 8; i++ {
		sum += rng.ExpFloat64()
	}
	return time.Duration(sum / 8 * minPerPoint * float64(n) * float64(time.Minute))
}

// Result résume un tournoi simulé.
type Result struct {
	State    *tournoi.State
	Journal  tournoi.Journal
	Winner   tournoi.PlayerID
	Minutes  float64
	NMatches int
	Steps    int
	Err      error
}

// Options de simulation.
type Options struct {
	Seed        int64
	MinPerPoint float64
	Start       time.Time
	MaxSteps    int
	// Hook est appelé après chaque événement appliqué (tests d'invariants).
	Hook func(s *tournoi.State, ev tournoi.Event) error
}

type running struct {
	id   tournoi.MatchID
	end  time.Time
	a, b tournoi.PlayerID
	n    int
}

// Run joue un tournoi complet.
func Run(cfg tournoi.Config, players []tournoi.Player, opt Options) Result {
	rng := rand.New(rand.NewSource(opt.Seed))
	if opt.MinPerPoint <= 0 {
		opt.MinPerPoint = 8
	}
	if opt.Start.IsZero() {
		opt.Start = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	}
	if opt.MaxSteps <= 0 {
		opt.MaxSteps = 100000
	}
	now := opt.Start
	st, ev, err := tournoi.New(cfg, opt.Seed, now)
	if err != nil {
		return Result{Err: err}
	}
	journal := tournoi.Journal{ev}
	apply := func(e tournoi.Event) error {
		e.Seq = len(journal)
		if err := st.Apply(e); err != nil {
			return err
		}
		journal = append(journal, e)
		if opt.Hook != nil {
			return opt.Hook(st, e)
		}
		return nil
	}
	for i := range players {
		p := players[i]
		if err := apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			return Result{Err: err, Journal: journal, State: st}
		}
	}
	var enCours []running
	steps := 0
	for !st.Finished {
		steps++
		if steps > opt.MaxSteps {
			return Result{Err: fmt.Errorf("simulation sans fin (%d étapes)", steps), Journal: journal, State: st}
		}
		acts := st.ProposeAt(now)
		progressed := false
		for _, a := range acts {
			if a.Kind == tournoi.ActWait {
				continue
			}
			e, err := st.EventFromAction(a, now)
			if err != nil {
				return Result{Err: err, Journal: journal, State: st}
			}
			if err := apply(e); err != nil {
				return Result{Err: fmt.Errorf("%s : %w", a, err), Journal: journal, State: st}
			}
			progressed = true
			if a.Kind == tournoi.ActStartMatch {
				enCours = append(enCours, running{e.MatchID, now.Add(Duree(a.Length, opt.MinPerPoint, rng)), a.A, a.B, a.Length})
			}
			if a.Kind == tournoi.ActDraw || a.Kind == tournoi.ActNextPhase || a.Kind == tournoi.ActFinish {
				break // l'état a changé : re-proposer
			}
		}
		if progressed && len(acts) > 0 && acts[0].Kind != tournoi.ActWait {
			continue
		}
		// Une attente peut porter une échéance (micro-rondes) : c'est le rôle de l'hôte
		// d'avancer jusque-là, le moteur n'a pas d'horloge.
		var attente time.Time
		for _, a := range acts {
			if a.Kind == tournoi.ActWait && !a.Until.IsZero() && a.Until.After(now) {
				attente = a.Until
			}
		}
		if len(enCours) == 0 {
			if !attente.IsZero() {
				now = attente
				continue
			}
			if !progressed {
				return Result{Err: fmt.Errorf("blocage : %v", acts), Journal: journal, State: st}
			}
			continue
		}
		// le match qui finit le premier
		sort.Slice(enCours, func(i, j int) bool { return enCours[i].end.Before(enCours[j].end) })
		if !attente.IsZero() && attente.Before(enCours[0].end) {
			now = attente // l'échéance du prochain lot arrive avant la fin du premier match
			continue
		}
		m := enCours[0]
		enCours = enCours[1:]
		if m.end.After(now) {
			now = m.end
		}
		pa, pb := st.Players[m.a].Rating, st.Players[m.b].Rating
		w := m.a
		if rng.Float64() >= PGain(pa, pb, m.n) {
			w = m.b
		}
		sa, sb := m.n, rng.Intn(m.n)
		if w == m.b {
			sa, sb = sb, sa
		}
		if err := apply(tournoi.ResultEvent(m.id, w, sa, sb, now)); err != nil {
			return Result{Err: err, Journal: journal, State: st}
		}
	}
	winner := tournoi.PlayerID("")
	if len(st.Final) > 0 {
		winner = st.Final[0].Player
	}
	return Result{State: st, Journal: journal, Winner: winner, Minutes: now.Sub(opt.Start).Minutes(), NMatches: len(st.MatchOrder), Steps: steps}
}

// Champ crée P joueurs de PR tirés dans N(mu, sigma) tronquée à [min, max] ; le meilleur a l'ID "P1".
func Champ(P int, mu, sigma, min, max float64, rng *rand.Rand) []tournoi.Player {
	prs := make([]float64, 0, P)
	for len(prs) < P {
		x := rng.NormFloat64()*sigma + mu
		if x >= min && x <= max {
			prs = append(prs, x)
		}
	}
	sort.Float64s(prs)
	out := make([]tournoi.Player, P)
	for i, pr := range prs {
		out[i] = tournoi.Player{ID: tournoi.PlayerID(fmt.Sprintf("P%d", i+1)), Name: fmt.Sprintf("Joueur %d", i+1), Club: fmt.Sprintf("Club %d", i%5), Rating: pr}
	}
	return out
}

// Forecast rejoue un journal puis simule K fins de tournoi à partir de l'instant now ; renvoie
// les durées restantes (minutes) triées, pour la prévision de l'heure de fin. Les PR inconnus
// (0) sont remplacés par la moyenne des PR connus (ou 6).
func Forecast(j tournoi.Journal, now time.Time, K int, minPerPoint float64, seed int64) ([]float64, error) {
	base, err := tournoi.Replay(j)
	if err != nil {
		return nil, err
	}
	if base.Finished {
		return []float64{0}, nil
	}
	if minPerPoint <= 0 {
		minPerPoint = base.Config.MinPerPoint
	}
	mean, n := 0.0, 0
	for _, p := range base.Players {
		if p.Rating > 0 {
			mean += p.Rating
			n++
		}
	}
	if n > 0 {
		mean /= float64(n)
	} else {
		mean = 6
	}
	rating := func(st *tournoi.State, id tournoi.PlayerID) float64 {
		if p := st.Players[id]; p != nil && p.Rating > 0 {
			return p.Rating
		}
		return mean
	}
	var out []float64
	for k := 0; k < K; k++ {
		rng := rand.New(rand.NewSource(seed + int64(k)))
		st, _ := tournoi.Replay(j)
		t := now
		var enCours []running
		for _, m := range st.Running() {
			// durée restante : attendue moins écoulée, au moins 10 % de l'attendue
			rest := Duree(m.Length, minPerPoint, rng) - now.Sub(m.Start)
			if min := time.Duration(0.1 * minPerPoint * float64(m.Length) * float64(time.Minute)); rest < min {
				rest = min
			}
			enCours = append(enCours, running{m.ID, now.Add(rest), m.A, m.B, m.Length})
		}
		steps := 0
		for !st.Finished && steps < 20000 {
			steps++
			acts := st.ProposeAt(t)
			progressed := false
			for _, a := range acts {
				if a.Kind == tournoi.ActWait {
					continue
				}
				e, err := st.EventFromAction(a, t)
				if err != nil {
					return nil, err
				}
				if err := st.Apply(e); err != nil {
					return nil, err
				}
				progressed = true
				if a.Kind == tournoi.ActStartMatch {
					enCours = append(enCours, running{e.MatchID, t.Add(Duree(a.Length, minPerPoint, rng)), a.A, a.B, a.Length})
				}
				if a.Kind == tournoi.ActDraw || a.Kind == tournoi.ActNextPhase || a.Kind == tournoi.ActFinish {
					break
				}
			}
			if progressed && len(acts) > 0 && acts[0].Kind != tournoi.ActWait {
				continue
			}
			var attente time.Time
			for _, a := range acts {
				if a.Kind == tournoi.ActWait && !a.Until.IsZero() && a.Until.After(t) {
					attente = a.Until
				}
			}
			if len(enCours) == 0 {
				if !attente.IsZero() {
					t = attente
					continue
				}
				if !progressed {
					return nil, fmt.Errorf("prévision bloquée")
				}
				continue
			}
			sort.Slice(enCours, func(i, j int) bool { return enCours[i].end.Before(enCours[j].end) })
			if !attente.IsZero() && attente.Before(enCours[0].end) {
				t = attente
				continue
			}
			m := enCours[0]
			enCours = enCours[1:]
			if m.end.After(t) {
				t = m.end
			}
			w := m.a
			if rng.Float64() >= PGain(rating(st, m.a), rating(st, m.b), m.n) {
				w = m.b
			}
			if err := st.Apply(tournoi.ResultEvent(m.id, w, m.n, 0, t)); err != nil {
				return nil, err
			}
		}
		out = append(out, t.Sub(now).Minutes())
	}
	sort.Float64s(out)
	return out, nil
}
