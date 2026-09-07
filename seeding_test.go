package tournoi_test

import (
	"math/rand"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// tirage : les places du premier tour proposées pour une phase de tableau.
func tirage(t *testing.T, cfg tournoi.Config, joueurs []tournoi.Player, graine int64) []tournoi.PlayerID {
	t.Helper()
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(cfg, graine, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range joueurs {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActDraw {
			return a.Draw.Slots
		}
	}
	t.Fatal("aucun tirage proposé")
	return nil
}

func tableauSimple(seeding string) tournoi.Config {
	return tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 9, Seeding: seeding}}}
}

// TestSansOptionLeTirageEstCeluiDAvant : le défaut n'a pas bougé d'un cran. Les places ci-dessous
// ont été relevées AVANT l'ajout des têtes de série ; un journal écrit alors se rejoue à
// l'identique, et les tests d'invariants voient exactement le même tournoi.
func TestSansOptionLeTirageEstCeluiDAvant(t *testing.T) {
	attendu := map[int][]tournoi.PlayerID{
		13: {"P5", "BYE", "P12", "BYE", "P11", "BYE", "P1", "P7", "P10", "P2", "P9", "P6", "P4", "P3", "P13", "P8"},
		16: {"P3", "P6", "P9", "P8", "P14", "P12", "P1", "P10", "P4", "P15", "P16", "P7", "P2", "P5", "P13", "P11"},
	}
	for n, veut := range attendu {
		joueurs := sim.Champ(n, 6, 2, 2, 10, rand.New(rand.NewSource(4242)))
		got := tirage(t, tableauSimple(""), joueurs, 4242)
		if len(got) != len(veut) {
			t.Fatalf("n=%d : %d places, attendu %d", n, len(got), len(veut))
		}
		for i := range veut {
			if got[i] != veut[i] {
				t.Fatalf("n=%d : le tirage sans option a changé\n  %v\n  %v", n, got, veut)
			}
		}
	}
}

// TestTetesDeSerieEcartentLesMeilleurs : sur un tableau complet, les deux meilleurs ne peuvent
// pas se rencontrer avant la finale, ni les quatre meilleurs avant les demies.
func TestTetesDeSerieEcartentLesMeilleurs(t *testing.T) {
	// sim.Champ numérote les joueurs du meilleur au moins bon : P1 est la tête de série 1.
	joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(1)))
	slots := tirage(t, tableauSimple(tournoi.SeedingRating), joueurs, 1)
	place := map[tournoi.PlayerID]int{}
	for i, p := range slots {
		place[p] = i
	}
	// deux moitiés pour les têtes 1 et 2, quatre quarts pour les têtes 1 à 4
	if place["P1"]/8 == place["P2"]/8 {
		t.Errorf("P1 (place %d) et P2 (place %d) sont dans la même moitié", place["P1"], place["P2"])
	}
	quarts := map[int]tournoi.PlayerID{}
	for _, p := range []tournoi.PlayerID{"P1", "P2", "P3", "P4"} {
		q := place[p] / 4
		if autre, dup := quarts[q]; dup {
			t.Errorf("%s et %s partagent le quart %d", p, autre, q)
		}
		quarts[q] = p
	}
	if place["P1"] != 0 {
		t.Errorf("la tête de série 1 occupe la place %d, attendu 0", place["P1"])
	}
}

// TestTetesDeSerieFinaleDesDeuxMeilleurs : la vérification qui compte vraiment — on joue le
// tableau en faisant gagner le mieux coté à chaque fois, et la finale doit opposer P1 à P2.
func TestTetesDeSerieFinaleDesDeuxMeilleurs(t *testing.T) {
	for _, seeding := range []string{"", tournoi.SeedingRating} {
		joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(2)))
		now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
		st, _, err := tournoi.New(tableauSimple(seeding), 2, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range joueurs {
			if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
				t.Fatal(err)
			}
		}
		var finale *tournoi.Match
		for étape := 0; !st.Finished && étape < 1000; étape++ {
			for _, a := range st.Propose() {
				if a.Kind == tournoi.ActWait {
					continue
				}
				now = now.Add(time.Minute)
				ev, err := st.EventFromAction(a, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := st.Apply(ev); err != nil {
					t.Fatal(err)
				}
				if a.Kind != tournoi.ActStartMatch {
					break
				}
			}
			for _, m := range st.Running() {
				gagnant := m.A // les joueurs sont numérotés du meilleur au moins bon
				if st.Players[m.B].Rating < st.Players[m.A].Rating {
					gagnant = m.B
				}
				if m.Label.Kind == tournoi.LabelFinal && m.Section == "main" {
					c := *m
					finale = &c
				}
				now = now.Add(time.Minute)
				if err := st.Apply(tournoi.ResultEvent(m.ID, gagnant, m.Length, 0, now)); err != nil {
					t.Fatal(err)
				}
			}
		}
		if finale == nil {
			t.Fatalf("seeding %q : aucune finale jouée", seeding)
		}
		opposeLesDeuxMeilleurs := (finale.A == "P1" && finale.B == "P2") || (finale.A == "P2" && finale.B == "P1")
		if seeding == tournoi.SeedingRating && !opposeLesDeuxMeilleurs {
			t.Errorf("avec têtes de série, la finale devrait opposer P1 et P2 : %s contre %s", finale.A, finale.B)
		}
		if seeding == "" && opposeLesDeuxMeilleurs {
			t.Log("sans têtes de série, P1 et P2 se retrouvent en finale par hasard (possible)")
		}
	}
}

// TestCoteInconnuePlaceeDerriere : 0 est la cote d'un joueur parfait ; la prendre au pied de la
// lettre ferait du nouvel inscrit la tête de série numéro un. Le placement doit être déterministe.
func TestCoteInconnuePlaceeDerriere(t *testing.T) {
	joueurs := []tournoi.Player{
		{ID: "sans-cote-b", Name: "Sans cote B"},
		{ID: "fort", Name: "Fort", Rating: 3},
		{ID: "sans-cote-a", Name: "Sans cote A"},
		{ID: "moyen", Name: "Moyen", Rating: 8},
	}
	slots := tirage(t, tableauSimple(tournoi.SeedingRating), joueurs, 7)
	veut := []tournoi.PlayerID{"fort", "sans-cote-b", "moyen", "sans-cote-a"}
	// tableau de 4 : places 0-3 = têtes 1, 4, 2, 3
	for i := range veut {
		if slots[i] != veut[i] {
			t.Fatalf("placement %v, attendu %v", slots, veut)
		}
	}
	// et il ne dépend pas de la graine : rien n'est tiré au sort
	for _, graine := range []int64{1, 99, 12345} {
		autre := tirage(t, tableauSimple(tournoi.SeedingRating), joueurs, graine)
		for i := range autre {
			if autre[i] != slots[i] {
				t.Fatalf("graine %d : le placement par cote ne doit pas dépendre du hasard\n  %v\n  %v", graine, autre, slots)
			}
		}
	}
}

// TestExemptsDevantDansUnTableauAVies : dans un lives_bracket, les joueurs à deux vies passent
// devant tous les autres, quelle que soit leur cote — la structure du tableau l'exige, puisque
// les exemptions vont aux premières têtes de série.
func TestExemptsDevantDansUnTableauAVies(t *testing.T) {
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Target: 8},
		{Kind: tournoi.KindLivesBracket, Length: 9, Seeding: tournoi.SeedingRating}}}
	joueurs := sim.Champ(24, 6, 2, 2, 10, rand.New(rand.NewSource(13)))
	r := sim.Run(cfg, joueurs, sim.Options{Seed: 13})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	var slots []tournoi.PlayerID
	var vies map[tournoi.PlayerID]int
	for _, ev := range r.Journal {
		if ev.Kind == tournoi.EvDraw && ev.Phase == 1 {
			slots, vies = ev.Draw.Slots, ev.Draw.Lives
		}
	}
	if slots == nil {
		t.Fatal("pas de tirage de tableau dans le journal")
	}
	for i := 0; i+1 < len(slots); i += 2 {
		a, b := slots[i], slots[i+1]
		if a != tournoi.BYE && b == tournoi.BYE && vies[a] < 2 {
			// une exemption pour un joueur à une vie n'est admise que s'il ne reste plus assez
			// de joueurs à deux vies pour occuper toutes les paires exemptées
			continue
		}
		if a != tournoi.BYE && b != tournoi.BYE && (vies[a] >= 2 || vies[b] >= 2) {
			t.Errorf("le joueur à deux vies %s/%s n'est pas exempt (places %d, %d)", a, b, i, i+1)
		}
	}
}

// TestSeedingRefuseAilleurs : une option qui ne fait rien là où on la pose est un piège.
func TestSeedingRefuseAilleurs(t *testing.T) {
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Seeding: tournoi.SeedingRating}}}
	if err := cfg.Validate(); err == nil {
		t.Error("seeding sur un suisse doit être refusé : il n'a pas de tirage de tableau")
	}
	cfg = tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 7, Seeding: "elo"}}}
	if err := cfg.Validate(); err == nil {
		t.Error("une valeur de seeding inconnue doit être refusée")
	}
}
