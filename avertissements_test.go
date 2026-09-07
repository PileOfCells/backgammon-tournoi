package tournoi_test

import (
	"math/rand"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// TestAvertissementDesLeResultatOrdinaire : un score au-delà de la longueur annoncée doit se
// voir TOUT DE SUITE. Avant ce test, check() n'était appelée que depuis recompute() — donc
// depuis les seules corrections, annulations et forfaits : un score faux saisi en cours de
// tournoi n'apparaissait qu'après une correction sans rapport, ou jamais.
func TestAvertissementDesLeResultatOrdinaire(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7}}}
	st, _, err := tournoi.New(cfg, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(4, 6, 2, 2, 10, rand.New(rand.NewSource(1))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	var id tournoi.MatchID
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
		id = ev.MatchID
		break
	}
	if id == "" {
		t.Fatal("aucun match lancé")
	}
	m := st.Matches[id]
	// Le score est enregistré : pendant un tournoi, c'est la parole du TD qui fait foi. Mais
	// l'incohérence doit être dite.
	if err := st.Apply(tournoi.ResultEvent(id, m.A, 99, 4, now.Add(time.Hour))); err != nil {
		t.Fatalf("un score incohérent est enregistré, pas refusé : %v", err)
	}
	if st.Matches[id].ScoreA != 99 {
		t.Errorf("le score n'a pas été enregistré : %d", st.Matches[id].ScoreA)
	}
	vu := false
	for _, w := range st.Warnings {
		if w.Code == tournoi.WarnScoreOverLength && w.Match == id {
			vu = true
			if w.Length != 7 || w.ScoreA != 99 || w.ScoreB != 4 {
				t.Errorf("avertissement mal renseigné : %+v", w)
			}
		}
	}
	if !vu {
		t.Fatalf("score 99-4 dans un match de 7 points : aucun avertissement (%v)", st.Warnings)
	}
	// et il disparaît dès que le score est corrigé
	if err := st.Apply(tournoi.CorrectionEvent(id, m.A, 7, 4, now.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	for _, w := range st.Warnings {
		if w.Code == tournoi.WarnScoreOverLength {
			t.Errorf("l'avertissement survit à la correction : %+v", w)
		}
	}
}

// TestAvertissementDesLeLancementDunMauvaisMatch : un match de tableau lancé par le mauvais
// joueur se signale au lancement, pas seulement au recalcul suivant.
func TestAvertissementDesLeLancementDunMauvaisMatch(t *testing.T) {
	joueurs := sim.Champ(8, 6, 2, 2, 10, rand.New(rand.NewSource(2)))
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 9}}}, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range joueurs {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range st.ProposeAt(now) { // le tirage
		if a.Kind == tournoi.ActDraw {
			ev, err := st.EventFromAction(a, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatal(err)
			}
		}
	}
	acts := st.ProposeAt(now)
	if len(acts) < 2 || acts[0].Kind != tournoi.ActStartMatch {
		t.Fatalf("attendu des matchs prêts : %v", acts)
	}
	// on lance le premier match de la place du premier… avec les joueurs du second
	a := acts[0]
	a.A, a.B = acts[1].A, acts[1].B
	ev, err := st.EventFromAction(a, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(ev); err != nil {
		t.Fatal(err)
	}
	vu := false
	for _, w := range st.Warnings {
		if w.Code == tournoi.WarnBracketWrongPlayers && w.Match == ev.MatchID {
			vu = true
		}
	}
	if !vu {
		t.Fatalf("match de tableau lancé par les mauvais joueurs : aucun avertissement (%v)", st.Warnings)
	}
}
