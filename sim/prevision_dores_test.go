package sim_test

import (
	"encoding/json"
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
	"github.com/PileOfCells/backgammon-tournoi/rules"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// testdata/prevision.json fige Forecast : un journal coupé en cours de tournoi, l'instant de la
// prévision, et les K durées restantes. La prévision est la seule règle de fin qui rejoue le
// moteur ; ses pièces arithmétiques (rating moyen, reste d'un match en cours, médiane, pauses)
// vivent dans rules, et c'est ce fichier qui prouve que l'enveloppe les assemble comme avant.
// `go test ./sim -update` le réécrit.

var maj = flag.Bool("update", false, "réécrire testdata/prevision.json")

type casPrevision struct {
	Name        string          `json:"name"`
	Journal     tournoi.Journal `json:"journal"`
	Now         time.Time       `json:"now"`
	K           int             `json:"k"`
	MinPerPoint float64         `json:"min_per_point"`
	Seed        int64           `json:"seed"`
	Out         []float64       `json:"out"`
}

func casPrevisions(t *testing.T) []casPrevision {
	suisse := tournoi.Config{Name: "Suisse prevision", MinPerPoint: 8, Tables: tournoi.Tables{Count: 10},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Lives: 2}}}
	tableau := tournoi.Config{Name: "Tableau prevision", MinPerPoint: 7,
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true}}}
	rng := rand.New(rand.NewSource(11))
	vingt := sim.Champ(20, 6, 2, 1, 15, rng)
	seize := sim.Champ(16, 6, 2, 1, 15, rng)
	inconnus := sim.Champ(16, 6, 2, 1, 15, rng)
	for i := range inconnus {
		if i%3 == 0 {
			inconnus[i].Rating = 0
		}
	}
	var out []casPrevision
	for _, s := range []struct {
		nom      string
		cfg      tournoi.Config
		joueurs  []tournoi.Player
		fraction float64
		après    time.Duration
		k        int
		mpp      float64
		graine   int64
	}{
		{"suisse au tiers", suisse, vingt, 0.3, 5 * time.Minute, 9, 0, 3},
		{"suisse aux deux tiers, rythme imposé", suisse, vingt, 0.7, 0, 7, 9.5, 4},
		{"tableau à mi-parcours, match en cours au-delà de l'attendu", tableau, seize, 0.5, 3 * time.Hour, 6, 0, 5},
		{"cotes inconnues", tableau, inconnus, 0.4, 20 * time.Minute, 5, 0, 6},
		{"tournoi terminé", tableau, seize, 1, 0, 4, 0, 7},
	} {
		r := sim.Run(s.cfg, s.joueurs, sim.Options{Seed: s.graine})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		j := r.Journal[:int(float64(len(r.Journal))*s.fraction)]
		out = append(out, casPrevision{Name: s.nom, Journal: j, Now: j[len(j)-1].Time.Add(s.après), K: s.k,
			MinPerPoint: s.mpp, Seed: s.graine * 100})
	}
	return out
}

// TestDoresPrevision : sim.Forecast à entrée égale.
func TestDoresPrevision(t *testing.T) {
	const chemin = "testdata/prevision.json"
	var cas []casPrevision
	if *maj {
		cas = casPrevisions(t)
	} else if err := dores.Read(chemin, &cas); err != nil {
		t.Fatal(err)
	}
	for i := range cas {
		c := &cas[i]
		runs, err := sim.Forecast(c.Journal, c.Now, c.K, c.MinPerPoint, c.Seed)
		if err != nil {
			t.Fatalf("%s : %v", c.Name, err)
		}
		c.Out = runs
	}
	if *maj {
		if err := dores.Write(chemin, cas); err != nil {
			t.Fatal(err)
		}
		return
	}
	ok, produit, err := dores.Same(chemin, cas)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		var court []any
		for _, c := range cas {
			court = append(court, map[string]any{"name": c.Name, "out": c.Out})
		}
		b, _ := json.MarshalIndent(court, "", " ")
		t.Errorf("%s : la prévision a changé à entrée égale (%d octets).\n%s", chemin, len(produit), b)
	}
}

// TestEstimatedEnd : la fin estimée est la médiane de Forecast reportée après les pauses, et un
// tournoi terminé n'en a pas.
func TestEstimatedEnd(t *testing.T) {
	var cas []casPrevision
	if err := dores.Read("testdata/prevision.json", &cas); err != nil {
		t.Fatal(err)
	}
	for _, c := range cas {
		fin, ok, err := sim.EstimatedEnd(c.Journal, c.Now, c.K, c.MinPerPoint, c.Seed)
		if err != nil {
			t.Fatalf("%s : %v", c.Name, err)
		}
		st, _ := tournoi.Replay(c.Journal)
		if st.Finished {
			if ok {
				t.Errorf("%s : terminé, et pourtant une fin estimée", c.Name)
			}
			continue
		}
		want, _ := rules.EstimatedEnd(c.Now, sim.Durations(c.Out), st.Config.BreakIntervals())
		if !ok || !fin.Equal(want) {
			t.Errorf("%s : %v (%v), attendu %v", c.Name, fin, ok, want)
		}
	}
}
