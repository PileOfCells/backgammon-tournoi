package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

func propositionAvec(acts []tournoi.Action, p tournoi.PlayerID) *tournoi.Action {
	for i := range acts {
		if acts[i].Kind == tournoi.ActStartMatch && (acts[i].A == p || acts[i].B == p) {
			return &acts[i]
		}
	}
	return nil
}

func attenteAbsent(acts []tournoi.Action, p tournoi.PlayerID) *tournoi.Action {
	for i := range acts {
		if acts[i].Kind == tournoi.ActWait && acts[i].Reason == tournoi.ReasonPlayerUnavailable && acts[i].A == p {
			return &acts[i]
		}
	}
	return nil
}

// TestIndisponibleJusquaUneHeure (N21, mode continu) : le joueur n'est pas apparié avant l'heure
// dite, la file dit pourquoi, et son classement ne bouge pas.
func TestIndisponibleJusquaUneHeure(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	retour := now.Add(3 * time.Hour)
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 6)
	avant := rangDe(st, "a")
	if err := st.Apply(tournoi.PlayerUnavailableEvent("a", retour, now)); err != nil {
		t.Fatal(err)
	}
	acts := st.ProposeAt(now)
	if m := propositionAvec(acts, "a"); m != nil {
		t.Fatalf("a est indisponible jusqu'à %s, il est pourtant proposé : %s", retour.Format("15:04"), m)
	}
	w := attenteAbsent(acts, "a")
	if w == nil {
		t.Fatalf("la file doit dire que a est indisponible : %v", acts)
	}
	if !w.Until.Equal(retour) {
		t.Errorf("heure de retour %v, attendu %v", w.Until, retour)
	}
	if après := rangDe(st, "a"); après != avant {
		t.Errorf("classement changé par l'absence : %+v → %+v", avant, après)
	}
	if st.Withdrawn["a"] {
		t.Error("une indisponibilité n'est pas un retrait")
	}
	if m := propositionAvec(st.ProposeAt(retour), "a"); m == nil {
		t.Error("à l'heure dite, a doit être de nouveau apparié")
	}
}

// TestIndisponibleJusquaUneRonde (N21, mode rondes) : absent de la ronde 1, apparié à la ronde 2 ;
// la ronde 1 se termine sans lui, sans bye.
func TestIndisponibleJusquaUneRonde(t *testing.T) {
	st, now, err := championnat(8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(tournoi.PlayerUnavailableUntilRoundEvent("j00", 2, now)); err != nil {
		t.Fatal(err)
	}
	e := newEngagement(t)
	acts := st.ProposeAt(now)
	if attenteAbsent(acts, "j00") == nil {
		t.Errorf("la file doit dire que j00 est absent : %v", acts)
	}
	var lancés []tournoi.Event
	for _, a := range acts {
		if a.A == "j00" || a.B == "j00" {
			if a.Kind != tournoi.ActWait {
				t.Fatalf("j00 est absent de la ronde 1 : %s", a)
			}
			continue
		}
		if a.Kind == tournoi.ActStartMatch || a.Kind == tournoi.ActBye {
			ev := appliquer(t, st, e, a, now)
			if a.Kind == tournoi.ActStartMatch {
				lancés = append(lancés, ev)
			}
		}
	}
	now = now.Add(time.Hour)
	for _, ev := range lancés {
		if err := st.Apply(tournoi.ResultEvent(ev.MatchID, ev.A, 7, 1, now)); err != nil {
			t.Fatal(err)
		}
	}
	m := propositionAvec(st.ProposeAt(now), "j00")
	if m == nil || m.Round != 2 || m.Reason != "" {
		t.Fatalf("à la ronde 2, j00 doit être apparié normalement : %v", st.ProposeAt(now))
	}
	if st.Phases[0].Byes["j00"] != 0 {
		t.Error("un absent ne reçoit pas de bye")
	}
}

// TestIndisponibleJusquaNouvelOrdre : sans échéance, l'indisponibilité dure jusqu'à l'événement
// inverse. Le journal se relit.
func TestIndisponibleJusquaNouvelOrdre(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC)
	_, created, err := tournoi.New(tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	j := tournoi.Journal{created}
	for _, id := range []tournoi.PlayerID{"a", "b", "c", "d"} {
		j = append(j, tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, now))
	}
	j = append(j, tournoi.PlayerUnavailableEvent("a", time.Time{}, now))
	b, _ := j.Bytes()
	relu, err := tournoi.ParseJournal(b)
	if err != nil {
		t.Fatal(err)
	}
	st, err := tournoi.Replay(relu)
	if err != nil {
		t.Fatal(err)
	}
	if propositionAvec(st.ProposeAt(now.Add(48*time.Hour)), "a") != nil {
		t.Fatal("sans échéance, a reste indisponible")
	}
	if err := st.Apply(tournoi.PlayerAvailableEvent("a", now)); err != nil {
		t.Fatal(err)
	}
	if propositionAvec(st.ProposeAt(now), "a") == nil {
		t.Error("player_available : a doit être de nouveau apparié")
	}
	if err := st.Apply(tournoi.PlayerUnavailableEvent("zz", time.Time{}, now)); err == nil {
		t.Error("un joueur inconnu ne peut pas être déclaré indisponible")
	}
}

// TestIndisponibleDansUnTableau : dans un tableau, le match d'un absent reste proposé — sa place
// est fixée par le graphe — mais sans table, avec la raison player_unavailable.
func TestIndisponibleDansUnTableau(t *testing.T) {
	now := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	st := tableauTire(t, tableauSeul())
	if err := st.Apply(tournoi.PlayerUnavailableEvent("a", now.Add(time.Hour), now)); err != nil {
		t.Fatal(err)
	}
	m := propositionAvec(st.ProposeAt(now), "a")
	if m == nil {
		t.Fatal("le match de a reste dans la file")
	}
	if m.Reason != tournoi.ReasonPlayerUnavailable || m.Table != 0 {
		t.Errorf("match d'un absent : raison %q, table %d ; attendu player_unavailable, sans table", m.Reason, m.Table)
	}
	if m := propositionAvec(st.ProposeAt(now.Add(time.Hour)), "a"); m == nil || m.Reason != "" || m.Table == 0 {
		t.Errorf("à son retour, le match de a se lance normalement : %v", m)
	}
}
