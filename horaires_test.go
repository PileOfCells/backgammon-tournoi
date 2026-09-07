package tournoi_test

import (
	"math/rand"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

func suisseAvecLots(t *testing.T, minutes, n int) (*tournoi.State, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, BatchMinutes: minutes}}}
	st, _, err := tournoi.New(cfg, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(n, 6, 2, 2, 10, rand.New(rand.NewSource(1))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	return st, now
}

// TestPremierLotSansAttente : sans lot précédent, il n'y a rien à attendre.
func TestPremierLotSansAttente(t *testing.T) {
	st, now := suisseAvecLots(t, 20, 8)
	acts := st.ProposeAt(now)
	n := 0
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("%d matchs proposés au premier lot, attendu 4", n)
	}
}

// TestAvantEcheanceRienNEstApparie : deux appels successifs avant l'échéance ne produisent aucun
// appariement, et l'échéance est lisible dans l'action d'attente — c'est ce qui permet à l'hôte
// d'afficher un compte à rebours.
func TestAvantEcheanceRienNEstApparie(t *testing.T) {
	st, now := suisseAvecLots(t, 20, 8)
	// premier lot : on lance tout, puis on rend deux résultats
	var lancés []tournoi.MatchID
	for _, a := range st.ProposeAt(now) {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		lancés = append(lancés, ev.MatchID)
	}
	fin := now.Add(30 * time.Minute)
	for _, id := range lancés[:2] {
		m := st.Matches[id]
		if err := st.Apply(tournoi.ResultEvent(id, m.A, 7, 3, fin)); err != nil {
			t.Fatal(err)
		}
	}
	échéance := now.Add(20 * time.Minute)
	// quatre joueurs sont libres, mais l'échéance du lot suivant n'est pas passée
	avant := now.Add(19 * time.Minute)
	acts := st.ProposeAt(avant)
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch {
			t.Fatalf("un match a été apparié avant l'échéance : %s", a)
		}
	}
	if len(acts) != 1 || acts[0].Kind != tournoi.ActWait || acts[0].Reason != tournoi.ReasonWaitingBatch {
		t.Fatalf("attendu une attente waiting_batch, reçu %v", acts)
	}
	if !acts[0].Until.Equal(échéance) {
		t.Errorf("échéance %s, attendue %s", acts[0].Until, échéance)
	}
	// un second appel avant l'échéance dit la même chose et n'apparie toujours rien
	acts2 := st.ProposeAt(avant.Add(30 * time.Second))
	if len(acts2) != 1 || acts2[0].Kind != tournoi.ActWait || !acts2[0].Until.Equal(échéance) {
		t.Fatalf("second appel : %v", acts2)
	}
}

// TestALEcheanceToutLeGroupeEstApparieDUnCoup : à l'échéance, tous les joueurs libres d'un même
// groupe de défaites partent ensemble — c'est ce qui ferme la porte à la manipulation par
// l'heure d'annonce d'un résultat.
func TestALEcheanceToutLeGroupeEstApparieDUnCoup(t *testing.T) {
	st, now := suisseAvecLots(t, 20, 8)
	var lancés []tournoi.MatchID
	for _, a := range st.ProposeAt(now) {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		ev, _ := st.EventFromAction(a, now)
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		lancés = append(lancés, ev.MatchID)
	}
	// les quatre matchs finissent à des instants différents, tous avant l'échéance
	for i, id := range lancés {
		m := st.Matches[id]
		if err := st.Apply(tournoi.ResultEvent(id, m.A, 7, 3, now.Add(time.Duration(i+1)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}
	acts := st.ProposeAt(now.Add(20 * time.Minute))
	gagnants, perdants := 0, 0
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			t.Fatalf("action inattendue : %s", a)
		}
		if a.Label.Losses == 0 {
			gagnants++
		} else {
			perdants++
		}
	}
	if gagnants != 2 || perdants != 2 {
		t.Errorf("lot : %d matchs de vainqueurs et %d de perdants, attendu 2 et 2 (%v)", gagnants, perdants, acts)
	}
}

// TestMicroRondesRejeuIdentique : le rejeu d'un journal avec micro-rondes redonne exactement les
// mêmes appariements — l'heure n'entre pas dans le journal autrement que par les horodatages.
func TestMicroRondesRejeuIdentique(t *testing.T) {
	joueurs := sim.Champ(24, 6, 2, 2, 10, rand.New(rand.NewSource(3)))
	r := sim.Run(configs()["suisse_micro_rondes"], joueurs, sim.Options{Seed: 3})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	st, err := tournoi.Replay(r.Journal)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.MatchOrder) != len(r.State.MatchOrder) {
		t.Fatalf("rejeu : %d matchs au lieu de %d", len(st.MatchOrder), len(r.State.MatchOrder))
	}
	for _, id := range r.State.MatchOrder {
		a, b := r.State.Matches[id], st.Matches[id]
		if b == nil || a.A != b.A || a.B != b.B || a.Winner != b.Winner {
			t.Fatalf("match %s diffère au rejeu", id)
		}
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements au rejeu : %v", st.Warnings)
	}
}

// TestLesLotsGroupentLesDeparts : la mesure qui compte. Sans lots, les matchs partent un par un ;
// avec, ils partent par paquets.
func TestLesLotsGroupentLesDeparts(t *testing.T) {
	joueurs := sim.Champ(32, 6, 2, 2, 10, rand.New(rand.NewSource(8)))
	départs := func(cfg tournoi.Config) float64 {
		r := sim.Run(cfg, joueurs, sim.Options{Seed: 8})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		instants := map[time.Time]int{}
		for _, id := range r.State.MatchOrder {
			instants[r.State.Matches[id].Start]++
		}
		return float64(len(r.State.MatchOrder)) / float64(len(instants))
	}
	sans := départs(configs()["suisse2_continu"])
	avec := départs(configs()["suisse_micro_rondes"])
	t.Logf("matchs par instant de départ : au fil de l'eau %.2f, en micro-rondes %.2f", sans, avec)
	if avec <= sans {
		t.Errorf("les micro-rondes devraient grouper les départs : %.2f contre %.2f", avec, sans)
	}
}

// TestFinPendantUnePauseEstSignaleeSansEtreBloquee : le moteur le dit, le directeur décide.
func TestFinPendantUnePauseEstSignaleeSansEtreBloquee(t *testing.T) {
	jour := func(h, m int) time.Time { return time.Date(2026, 9, 7, h, m, 0, 0, time.UTC) }
	cfg := tournoi.Config{Name: "T", MinPerPoint: 8,
		Breaks: []tournoi.TimeRange{{Start: jour(12, 0), End: jour(13, 0)}},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}}
	st, _, err := tournoi.New(cfg, 1, jour(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(8, 6, 2, 2, 10, rand.New(rand.NewSource(1))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, jour(10, 0))); err != nil {
			t.Fatal(err)
		}
	}
	// un match de 7 points dure 56 minutes : lancé à 11 h 30 il finit à 12 h 26, en pleine pause
	acts := st.ProposeAt(jour(11, 30))
	if len(acts) == 0 {
		t.Fatal("aucune proposition")
	}
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		if a.Warn != tournoi.WarnEndsInBreak {
			t.Errorf("la proposition devrait porter ends_in_break : %+v", a)
		}
	}
	// et elle reste PROPOSÉE : rien n'est bloqué
	n := 0
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch {
			n++
		}
	}
	if n != 4 {
		t.Errorf("%d matchs proposés, attendu 4 — un avertissement ne bloque rien", n)
	}
	// lancé à 9 h, le même match finit avant la pause : aucun avertissement
	for _, a := range st.ProposeAt(jour(9, 0)) {
		if a.Kind == tournoi.ActStartMatch && a.Warn != "" {
			t.Errorf("avertissement injustifié à 9 h : %+v", a)
		}
	}
}

// TestPauseIncoherenteRefusee : une pause qui finit avant de commencer est une faute de saisie.
func TestPauseIncoherenteRefusee(t *testing.T) {
	jour := func(h int) time.Time { return time.Date(2026, 9, 7, h, 0, 0, 0, time.UTC) }
	cfg := tournoi.Config{Name: "T",
		Breaks: []tournoi.TimeRange{{Start: jour(13), End: jour(12)}},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}}
	if err := cfg.Validate(); err == nil {
		t.Error("une pause qui finit avant de commencer doit être refusée")
	}
}

// TestBatchMinutesRefuseAilleurs : une option qui ne fait rien là où on la pose est un piège.
func TestBatchMinutesRefuseAilleurs(t *testing.T) {
	for nom, ph := range map[string]tournoi.PhaseConfig{
		"tableau": {Kind: tournoi.KindBracket, Length: 7, BatchMinutes: 20},
		"rondes":  {Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds", BatchMinutes: 20},
		"négatif": {Kind: tournoi.KindSwissLives, Length: 7, BatchMinutes: -1},
	} {
		cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{ph}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s : batch_minutes devrait être refusé", nom)
		}
	}
}
