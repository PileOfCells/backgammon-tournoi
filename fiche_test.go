package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// TestFicheCorrigeeSansToucherAuRetrait (N23) : corriger le club d'une joueuse retirée corrige la
// fiche, et elle reste retirée. C'est ce que player_added sur un identifiant connu ne savait pas
// faire : il réinscrivait.
func TestFicheCorrigeeSansToucherAuRetrait(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 6)
	if err := st.Apply(tournoi.PlayerWithdrawnEvent("a", now)); err != nil {
		t.Fatal(err)
	}
	avant := st.Ranking()
	fiche := tournoi.Player{ID: "a", Name: "Valentine", Club: "Lyon", Rating: 5.5}
	if err := st.Apply(tournoi.PlayerUpdatedEvent(fiche, now)); err != nil {
		t.Fatal(err)
	}
	if !st.Withdrawn["a"] {
		t.Fatal("player_updated a réinscrit la joueuse retirée")
	}
	if got := *st.Players["a"]; got != fiche {
		t.Errorf("fiche %+v, attendu %+v", got, fiche)
	}
	après := st.Ranking()
	for i := range avant {
		if avant[i] != après[i] {
			t.Fatalf("le classement a changé : %v → %v", avant[i], après[i])
		}
	}
	for _, a := range st.Propose() {
		if a.A == "a" || a.B == "a" {
			t.Fatalf("la joueuse retirée est proposée : %s", a)
		}
	}
	// Un identifiant inconnu n'est pas une correction : c'est une inscription, et elle a son
	// propre événement.
	if err := st.Apply(tournoi.PlayerUpdatedEvent(tournoi.Player{ID: "zz", Name: "Z"}, now)); err == nil {
		t.Error("player_updated sur un joueur inconnu doit être refusé")
	}
}

// TestFicheCorrigeeRejouee : un journal qui porte player_updated se rejoue à l'identique.
func TestFicheCorrigeeRejouee(t *testing.T) {
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	_, created, err := tournoi.New(tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	j := tournoi.Journal{created,
		tournoi.PlayerAddedEvent(tournoi.Player{ID: "a", Name: "A"}, now),
		tournoi.PlayerAddedEvent(tournoi.Player{ID: "b", Name: "B"}, now),
		tournoi.PlayerWithdrawnEvent("a", now),
		tournoi.PlayerUpdatedEvent(tournoi.Player{ID: "a", Name: "A.", Club: "Nice"}, now),
	}
	b, err := j.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	relu, err := tournoi.ParseJournal(b)
	if err != nil {
		t.Fatal(err)
	}
	st, err := tournoi.Replay(relu)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Withdrawn["a"] || st.Players["a"].Club != "Nice" {
		t.Errorf("après rejeu : retiré=%v club=%q", st.Withdrawn["a"], st.Players["a"].Club)
	}
}
