package tournoi_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

func configs() map[string]tournoi.Config {
	return map[string]tournoi.Config{
		"suisse2_continu": {Name: "Suisse 2 vies continu", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}},
		"suisse2_rondes":  {Name: "Suisse 2 vies rondes", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds", AvoidClubs: true}}},
		"suisse3_wins":    {Name: "Suisse 3 vies", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5, Lives: 3, Pairing: "wins"}}},
		"suisse_tableau16": {Name: "Suisse puis tableau à vies", Phases: []tournoi.PhaseConfig{
			{Kind: tournoi.KindSwissLives, Length: 7, Target: 16},
			{Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11}}},
		"gsl": {Name: "Blocs GSL", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindGSL, Length: 7}}},
		"gsl_tableau16": {Name: "GSL puis tableau", Phases: []tournoi.PhaseConfig{
			{Kind: tournoi.KindGSL, Length: 7, Target: 16},
			{Kind: tournoi.KindLivesBracket, Length: 9}}},
		"elim_simple":     {Name: "Élimination simple", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 11, FinalLength: 13}}},
		"elim_conso":      {Name: "Principal + consolante", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true}}},
		"elim_conso_last": {Name: "Principal + consolante + dernière chance", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true, LastChance: true}}},
		"double_elim":     {Name: "Double élimination", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7, Consolation: true, Reconciliation: true, Recharge: true}}},
		"poules_tableau": {Name: "Poules puis tableau", Phases: []tournoi.PhaseConfig{
			{Kind: tournoi.KindRoundRobin, Length: 5, GroupSize: 4, Qualifiers: 2},
			{Kind: tournoi.KindBracket, Length: 9, Entry: "survivors"}}},
	}
}

// hook vérifie les invariants après chaque événement.
func hook(t *testing.T, name string, seen map[string]int) func(s *tournoi.State, ev tournoi.Event) error {
	return func(s *tournoi.State, ev tournoi.Event) error {
		if ev.Kind != tournoi.EvMatchStarted {
			return nil
		}
		m := s.Matches[ev.MatchID]
		ph := s.Phases[m.Phase]
		// aucun match entre deux joueurs sans vie dans une phase à vies
		if ph.Cfg.Kind == tournoi.KindSwissLives || ph.Cfg.Kind == tournoi.KindGSL {
			for _, p := range []tournoi.PlayerID{m.A, m.B} {
				if ph.Losses[p] >= ph.Lives[p] {
					return fmt.Errorf("%s : %s joue alors qu'il est éliminé (%d défaites)", name, p, ph.Losses[p])
				}
			}
			// Les matchs de secours (finale, match croisé) apparient volontairement des groupes
			// différents ; ils se reconnaissent à leur CODE, jamais à leur libellé affiché.
			crossGroup := m.Label.Kind == tournoi.LabelFinal || m.Label.Kind == tournoi.LabelCrossed
			if ph.Losses[m.A] != ph.Losses[m.B] && !crossGroup {
				return fmt.Errorf("%s : match %s entre joueurs de groupes différents (%d vs %d défaites)", name, m.Label, ph.Losses[m.A], ph.Losses[m.B])
			}
		}
		// rematch : compté seulement en suisse (inhérent aux GSL et aux consolantes)
		if ph.Cfg.Kind != tournoi.KindSwissLives {
			return nil
		}
		n := 0
		for _, o := range ph.Opponents[m.A] {
			if o == m.B {
				n++
			}
		}
		if n > 1 {
			seen["rematch"]++
		}
		return nil
	}
}

func TestFormatsInvariants(t *testing.T) {
	for name, cfg := range configs() {
		for _, P := range []int{2, 3, 5, 8, 13, 32, 64, 100} {
			seen := map[string]int{}
			for k := 0; k < 8; k++ {
				rng := rand.New(rand.NewSource(int64(P*1000 + k)))
				players := sim.Champ(P, 6, 2, 2, 10, rng)
				r := sim.Run(cfg, players, sim.Options{Seed: int64(k), Hook: hook(t, name, seen)})
				if r.Err != nil {
					t.Fatalf("%s P=%d k=%d : %v", name, P, k, r.Err)
				}
				if r.Winner == "" {
					t.Fatalf("%s P=%d : pas de vainqueur", name, P)
				}
				if n := countRank1(r.State.Final); n != 1 {
					t.Fatalf("%s P=%d : %d joueurs classés premiers", name, P, n)
				}
				if len(r.State.Final) != P {
					t.Fatalf("%s P=%d : classement de %d joueurs", name, P, len(r.State.Final))
				}
				// rejeu déterministe
				st2, err := tournoi.Replay(r.Journal)
				if err != nil {
					t.Fatalf("%s : rejeu : %v", name, err)
				}
				a, _ := json.Marshal(r.State.Final)
				b, _ := json.Marshal(st2.Final)
				if string(a) != string(b) {
					t.Fatalf("%s P=%d : le rejeu donne un classement différent", name, P)
				}
				if len(st2.Warnings) > 0 {
					t.Fatalf("%s : avertissements après rejeu : %v", name, st2.Warnings)
				}
			}
			lives := cfg.Phases[0].Lives
			if lives == 0 {
				lives = 2
			}
			if P >= 8 && seen["rematch"] > 8*(lives+3) {
				t.Errorf("%s P=%d : trop de rematchs (%d)", name, P, seen["rematch"])
			}
		}
	}
}

func countRank1(r []tournoi.Rank) int {
	n := 0
	for _, x := range r {
		if x.Rank == 1 {
			n++
		}
	}
	return n
}

func TestMatchCounts(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	players := sim.Champ(64, 6, 2, 2, 10, rng)
	// 64 joueurs à 2 vies : 126 ou 127 matchs (127 avec recharge)
	r := sim.Run(configs()["suisse2_continu"], players, sim.Options{Seed: 1})
	if r.Err != nil || (r.NMatches != 126 && r.NMatches != 127) {
		t.Fatalf("suisse 2 vies 64 joueurs : %d matchs, err %v", r.NMatches, r.Err)
	}
	r = sim.Run(configs()["elim_simple"], players, sim.Options{Seed: 1})
	if r.Err != nil || r.NMatches != 63 {
		t.Fatalf("élimination simple 64 : %d matchs, err %v", r.NMatches, r.Err)
	}
	r = sim.Run(configs()["double_elim"], players, sim.Options{Seed: 1})
	if r.Err != nil || (r.NMatches != 126 && r.NMatches != 127) {
		t.Fatalf("double élimination 64 : %d matchs, err %v", r.NMatches, r.Err)
	}
	r = sim.Run(configs()["gsl"], players, sim.Options{Seed: 1})
	if r.Err != nil || (r.NMatches != 126 && r.NMatches != 127) {
		t.Fatalf("GSL 64 : %d matchs, err %v", r.NMatches, r.Err)
	}
}

func TestCorrection(t *testing.T) {
	// un résultat corrigé après coup : l'état est recalculé et cohérent
	rng := rand.New(rand.NewSource(3))
	players := sim.Champ(16, 6, 2, 2, 10, rng)
	r := sim.Run(configs()["suisse2_continu"], players, sim.Options{Seed: 3})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	j := r.Journal
	// on inverse le premier résultat
	for i := range j {
		if j[i].Kind == tournoi.EvResult {
			m := r.State.Matches[j[i].MatchID]
			c := j[i]
			c.Kind = tournoi.EvResultCorrected
			c.Winner = m.Loser()
			j = append(j, c)
			break
		}
	}
	st, err := tournoi.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	ph := st.Phases[0]
	total := 0
	for _, p := range ph.Entrants {
		total += ph.Losses[p]
	}
	finished := 0
	for _, m := range st.Matches {
		if m.Status == tournoi.Finished {
			finished++
		}
	}
	if total != finished {
		t.Fatalf("défaites %d ≠ matchs terminés %d après correction", total, finished)
	}
}

func TestWithdrawnAfterDraw(t *testing.T) {
	// un joueur retiré après le tirage d'un tableau ne doit plus être proposé : ses matchs non
	// lancés sont perdus par forfait et le tableau se termine quand même
	rng := rand.New(rand.NewSource(5))
	players := sim.Champ(16, 6, 2, 2, 10, rng)
	r := sim.Run(configs()["elim_simple"], players, sim.Options{Seed: 5})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	// journal coupé juste après le tirage
	var j tournoi.Journal
	for _, ev := range r.Journal {
		j = append(j, ev)
		if ev.Kind == tournoi.EvDraw {
			break
		}
	}
	st, err := tournoi.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	gone := st.Phases[0].Entrants[0]
	if err := st.Apply(tournoi.Event{Kind: tournoi.EvPlayerWithdrawn, Time: st.Last, ID: gone}); err != nil {
		t.Fatal(err)
	}
	for steps := 0; !st.Finished && steps < 1000; steps++ {
		acts := st.Propose()
		for _, a := range acts {
			if a.Kind == tournoi.ActStartMatch && (a.A == gone || a.B == gone) {
				t.Fatalf("le joueur retiré %s est proposé : %s", gone, a)
			}
			if a.Kind == tournoi.ActWait {
				continue
			}
			ev, err := st.EventFromAction(a, st.Last)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatal(err)
			}
			if a.Kind == tournoi.ActStartMatch {
				if err := st.Apply(tournoi.ResultEvent(ev.MatchID, a.A, ev.Length, 0, st.Last)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if !st.Finished {
		t.Fatal("le tableau ne se termine pas après un forfait")
	}
	if len(st.Warnings) > 0 {
		t.Fatal(st.Warnings)
	}
}
