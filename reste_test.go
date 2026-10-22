package tournoi_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/rules"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// TestRemainingMatchesEncadre : à chaque événement d'un tournoi simulé, rules.RemainingMatches
// encadre le nombre de matchs que la phase en cours terminera encore, compté à la fin du
// tournoi. Exact (Min = Max) dans un tableau sans recharge.
func TestRemainingMatchesEncadre(t *testing.T) {
	effectifs, graines := []int{2, 3, 5, 8, 13, 32}, 3
	if testing.Short() {
		effectifs, graines = []int{2, 3, 5, 8, 13}, 2
	}
	cfgs := configs()
	var noms []string
	for n := range cfgs {
		noms = append(noms, n)
	}
	sort.Strings(noms)
	for _, nom := range noms {
		cfg := cfgs[nom]
		for _, P := range effectifs {
			for k := 0; k < graines; k++ {
				type relevé struct {
					phase, finis int
					r            rules.Remaining
					exact        bool
				}
				var rel []relevé
				hook := func(s *tournoi.State, ev tournoi.Event) error {
					// Pendant les inscriptions, les entrants de la phase ne sont pas encore connus.
					if s.Current < 0 || s.Finished || ev.Kind == tournoi.EvPlayerAdded {
						return nil
					}
					p := s.RulesPhase(s.Current)
					finis := 0
					for _, m := range s.Matches {
						if m.Phase == s.Current && m.Status == tournoi.Finished {
							finis++
						}
					}
					exact := (p.Format == rules.FormatBracket || p.Format == rules.FormatLivesBracket) && !p.Rules.Recharge
					rel = append(rel, relevé{s.Current, finis, rules.RemainingMatches(*p), exact})
					return nil
				}
				rng := rand.New(rand.NewSource(int64(P*1000 + k)))
				r := sim.Run(cfg, sim.Champ(P, 6, 2, 2, 10, rng), sim.Options{Seed: int64(k), Hook: hook})
				if r.Err != nil {
					t.Fatalf("%s P=%d k=%d : %v", nom, P, k, r.Err)
				}
				total := map[int]int{}
				for _, m := range r.State.Matches {
					if m.Status == tournoi.Finished {
						total[m.Phase]++
					}
				}
				for i, x := range rel {
					reste := total[x.phase] - x.finis
					if reste < x.r.Min || reste > x.r.Max || (x.exact && x.r.Min != x.r.Max) {
						t.Fatalf("%s P=%d k=%d, relevé %d (phase %d) : %d matchs restaient, encadrés par %+v", nom, P, k, i, x.phase, reste, x.r)
					}
				}
			}
		}
	}
}
