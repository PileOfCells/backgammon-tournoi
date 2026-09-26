package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

func attenteOccupé(acts []tournoi.Action, p tournoi.PlayerID) *tournoi.Action {
	for i := range acts {
		if acts[i].Kind == tournoi.ActWait && acts[i].Reason == tournoi.ReasonPlayerBusy && acts[i].A == p {
			return &acts[i]
		}
	}
	return nil
}

// TestOccupéAilleursAuSuisse (N24) : un joueur qui joue dans une autre épreuve de la salle n'est
// pas apparié ; la file le dit ; il l'est de nouveau quand l'extérieur le rend. Rien n'est écrit.
func TestOccupéAilleursAuSuisse(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 6)
	avant := st.NEvents
	ext := tournoi.External{BusyPlayers: []tournoi.PlayerID{"a"}}
	acts := st.ProposeWith(now, ext)
	if m := propositionAvec(acts, "a"); m != nil {
		t.Fatalf("a joue ailleurs, il est pourtant proposé : %s", m)
	}
	if attenteOccupé(acts, "a") == nil {
		t.Fatalf("la file doit dire que a est occupé ailleurs : %v", acts)
	}
	if attenteAbsent(acts, "a") != nil {
		t.Error("occupé ailleurs n'est pas indisponible : raison player_busy, pas player_unavailable")
	}
	lancés := 0
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch {
			lancés++
		}
	}
	if lancés != 2 {
		t.Errorf("cinq joueurs libres : deux matchs attendus, %d", lancés)
	}
	if st.NEvents != avant || st.Unavailable["a"] != (tournoi.Absence{}) {
		t.Error("l'occupation extérieure ne s'écrit ni au journal ni dans l'état")
	}
	if m := propositionAvec(st.ProposeWith(now, tournoi.External{}), "a"); m == nil {
		t.Error("l'extérieur rend a : il doit être de nouveau apparié")
	}
	// Un joueur inconnu de ce tournoi (l'hôte se trompe de traduction) ne gêne rien.
	if acts := st.ProposeWith(now, tournoi.External{BusyPlayers: []tournoi.PlayerID{"zz"}}); attenteOccupé(acts, "zz") != nil {
		t.Error("un inconnu n'a pas d'attente dans la file")
	}
}

// TestOccupéAilleursAppariéÀLaMain : le directeur qui l'apparie quand même est obéi.
func TestOccupéAilleursAppariéÀLaMain(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 6)
	st.ProposeWith(now, tournoi.External{BusyPlayers: []tournoi.PlayerID{"a"}})
	ev, err := st.EventFromAction(tournoi.Action{Kind: tournoi.ActStartMatch, Phase: 0, A: "a", B: "b", Length: 5}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(ev); err != nil {
		t.Fatalf("un appariement à la main d'un joueur occupé ailleurs doit être accepté : %v", err)
	}
}

// TestOccupéAilleursDansUnTableau : sa place est fixée ; le match reste proposé, retenu, sans
// table, avec la raison player_busy.
func TestOccupéAilleursDansUnTableau(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := tableauTire(t, tableauSeul())
	m := propositionAvec(st.ProposeWith(now, tournoi.External{BusyPlayers: []tournoi.PlayerID{"a"}}), "a")
	if m == nil {
		t.Fatal("le match de a reste dans la file")
	}
	if m.Reason != tournoi.ReasonPlayerBusy || m.Table != 0 {
		t.Errorf("match d'un joueur occupé ailleurs : raison %q, table %d ; attendu player_busy, sans table", m.Reason, m.Table)
	}
	if m := propositionAvec(st.ProposeWith(now, tournoi.External{}), "a"); m == nil || m.Reason != "" || m.Table == 0 {
		t.Errorf("libéré, le match de a se lance normalement : %v", m)
	}
}

// TestOccupéAilleursEtAbsent : l'indisponibilité, écrite au journal, l'emporte dans la raison.
func TestOccupéAilleursEtAbsent(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := tableauTire(t, tableauSeul())
	if err := st.Apply(tournoi.PlayerUnavailableEvent("a", time.Time{}, now)); err != nil {
		t.Fatal(err)
	}
	m := propositionAvec(st.ProposeWith(now, tournoi.External{BusyPlayers: []tournoi.PlayerID{"a"}}), "a")
	if m == nil || m.Reason != tournoi.ReasonPlayerUnavailable {
		t.Errorf("absent et occupé ailleurs : raison player_unavailable attendue, %v", m)
	}
}
