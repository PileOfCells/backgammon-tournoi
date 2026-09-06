package tournoi_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// TestParity mesure P(meilleur gagne), P(top 4), matchs et durée sur 64 joueurs à 7 points,
// à comparer au simulateur de l'étude (modèle elo, champ N(6,2) borné [2,10]).
func TestParity(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	cfgs := map[string]tournoi.Config{
		"2 vies continu":        {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}},
		"2 vies rondes":         {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds"}}},
		"Élim. simple":          {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7}}},
		"Double élim. recharge": {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7, Consolation: true, Reconciliation: true, Recharge: true}}},
		"Blocs GSL":             {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindGSL, Length: 7}}},
		"Suisse → tableau 16":   {Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Target: 16}, {Kind: tournoi.KindLivesBracket, Length: 7}}},
	}
	it := 1500
	for name, cfg := range cfgs {
		best, top4, matchs, minutes := 0, 0, 0, 0.0
		for k := 0; k < it; k++ {
			rng := rand.New(rand.NewSource(int64(k)))
			players := sim.Champ(64, 6, 2, 2, 10, rng)
			r := sim.Run(cfg, players, sim.Options{Seed: int64(k)})
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			if r.Winner == "P1" {
				best++
			}
			for i := 1; i <= 4; i++ {
				if r.Winner == tournoi.PlayerID(fmt.Sprintf("P%d", i)) {
					top4++
				}
			}
			matchs += r.NMatches
			minutes += r.Minutes
		}
		fmt.Printf("%-24s P(best)=%.3f P(top4)=%.3f matchs=%.1f minutes=%.0f\n", name, float64(best)/float64(it), float64(top4)/float64(it), float64(matchs)/float64(it), minutes/float64(it))
	}
}
