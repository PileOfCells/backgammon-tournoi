package tournoi_test

import (
	"fmt"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

func championnat(n, tables int) (*tournoi.State, time.Time, error) {
	now := time.Date(2026, 8, 17, 20, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(tournoi.Config{Name: "Club", Tables: tournoi.Tables{Count: tables},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Lives: 6, Mode: "rounds"}}}, 3, now)
	if err != nil {
		return nil, now, err
	}
	for i := 0; i < n; i++ {
		id := tournoi.PlayerID(fmt.Sprintf("j%02d", i))
		if err := st.Apply(tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, now)); err != nil {
			return nil, now, err
		}
	}
	return st, now, nil
}

// engagement : qui a été engagé dans quelle ronde (match lancé ou bye), relevé sur les
// événements confirmés.
type engagement struct {
	t      *testing.T
	ronde  map[tournoi.PlayerID]map[int]int
	ordre  []int // ronde de chaque match lancé, dans l'ordre
	nMatch map[int]int
}

func newEngagement(t *testing.T) *engagement {
	return &engagement{t: t, ronde: map[tournoi.PlayerID]map[int]int{}, nMatch: map[int]int{}}
}

func (e *engagement) note(ev tournoi.Event) {
	add := func(p tournoi.PlayerID) {
		if e.ronde[p] == nil {
			e.ronde[p] = map[int]int{}
		}
		e.ronde[p][ev.Round]++
		if e.ronde[p][ev.Round] > 1 {
			e.t.Errorf("%s engagé deux fois dans la ronde %d", p, ev.Round)
		}
	}
	switch ev.Kind {
	case tournoi.EvMatchStarted:
		add(ev.A)
		add(ev.B)
		e.ordre = append(e.ordre, ev.Round)
		e.nMatch[ev.Round]++
	case tournoi.EvBye:
		add(ev.ID)
	}
}

func appliquer(t *testing.T, st *tournoi.State, e *engagement, a tournoi.Action, now time.Time) tournoi.Event {
	t.Helper()
	ev, err := st.EventFromAction(a, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(ev); err != nil {
		t.Fatalf("%s : %v", a, err)
	}
	e.note(ev)
	return ev
}

// TestRondePlusGrandeQueLaSalle (N17) : 24 joueurs, 6 tables. Le TD lance ce qui a une table,
// saisit les résultats, relance : les 12 matchs de la ronde 1 sont tous joués avant le premier
// de la ronde 2, et personne ne joue deux fois la même ronde.
func TestRondePlusGrandeQueLaSalle(t *testing.T) {
	st, now, err := championnat(24, 6)
	if err != nil {
		t.Fatal(err)
	}
	e := newEngagement(t)
	for vague := 0; vague < 12 && e.nMatch[3] == 0; vague++ {
		acts := st.ProposeAt(now)
		enAttente := 0
		for _, a := range acts {
			if a.Kind == tournoi.ActStartMatch && a.Reason == tournoi.ReasonWaitingTable {
				enAttente++
			}
		}
		var lancés []tournoi.Event
		for _, a := range acts {
			switch {
			case a.Kind == tournoi.ActStartMatch && a.Table > 0:
				lancés = append(lancés, appliquer(t, st, e, a, now))
			case a.Kind == tournoi.ActBye:
				appliquer(t, st, e, a, now)
			}
		}
		// Pendant que la première vague joue, les appariements sans table restent proposés.
		if enAttente > 0 {
			rest := 0
			for _, a := range st.ProposeAt(now) {
				if a.Kind == tournoi.ActStartMatch {
					rest++
				}
			}
			if rest != enAttente {
				t.Fatalf("vague %d : %d appariements attendaient une table, %d restent proposés après le lancement",
					vague, enAttente, rest)
			}
		}
		now = now.Add(time.Hour)
		for _, ev := range lancés {
			if err := st.Apply(tournoi.ResultEvent(ev.MatchID, ev.A, 7, 3, now)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if e.nMatch[1] != 12 || e.nMatch[2] != 12 {
		t.Fatalf("matchs par ronde : %v, attendu 12 en ronde 1 et 12 en ronde 2", e.nMatch)
	}
	for i := 1; i < len(e.ordre); i++ {
		if e.ordre[i] < e.ordre[i-1] {
			t.Fatalf("un match de ronde %d lancé après un match de ronde %d : %v", e.ordre[i], e.ordre[i-1], e.ordre)
		}
	}
}

// TestByeConfirmeSeul (N18) : 25 joueurs, une ronde de 12 matchs et un bye. Confirmer le bye
// seul ne clôt pas la ronde : les 12 matchs restent proposés, les mêmes, et l'exempté n'est pas
// réapparié.
func TestByeConfirmeSeul(t *testing.T) {
	st, now, err := championnat(25, 0)
	if err != nil {
		t.Fatal(err)
	}
	e := newEngagement(t)
	avant := st.ProposeAt(now)
	var bye tournoi.Action
	paires := map[string]bool{}
	for _, a := range avant {
		switch a.Kind {
		case tournoi.ActBye:
			bye = a
		case tournoi.ActStartMatch:
			paires[string(a.A)+"-"+string(a.B)] = true
		}
	}
	if bye.Kind == "" || len(paires) != 12 {
		t.Fatalf("ronde 1 : 12 matchs et un bye attendus, %v", avant)
	}
	appliquer(t, st, e, bye, now)
	après := st.ProposeAt(now)
	n := 0
	for _, a := range après {
		if a.A == bye.A || a.B == bye.A {
			t.Errorf("l'exempté %s est réapparié dans la même ronde : %s", bye.A, a)
		}
		if a.Kind == tournoi.ActBye {
			t.Errorf("second bye dans la ronde : %s", a)
		}
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		n++
		if a.Round != 1 {
			t.Errorf("proposition de ronde %d, attendu 1 : %s", a.Round, a)
		}
		if !paires[string(a.A)+"-"+string(a.B)] {
			t.Errorf("appariement changé après le bye : %s-%s", a.A, a.B)
		}
	}
	if n != 12 {
		t.Fatalf("après le bye, les 12 matchs de la ronde doivent rester proposés : %d", n)
	}
}

// TestRondeReprendApresResultats : la moitié de la ronde est lancée ET jouée avant que l'autre
// moitié ne commence ; les appariements restants ne changent pas pour autant — la feuille
// imprimée reste juste.
func TestRondeReprendApresResultats(t *testing.T) {
	st, now, err := championnat(16, 4)
	if err != nil {
		t.Fatal(err)
	}
	e := newEngagement(t)
	acts := st.ProposeAt(now)
	restants := map[string]bool{}
	var lancés []tournoi.Event
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		if a.Table > 0 {
			lancés = append(lancés, appliquer(t, st, e, a, now))
		} else {
			restants[string(a.A)+"-"+string(a.B)] = true
		}
	}
	for _, ev := range lancés {
		if err := st.Apply(tournoi.ResultEvent(ev.MatchID, ev.B, 3, 7, now)); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, a := range st.ProposeAt(now) {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		n++
		if !restants[string(a.A)+"-"+string(a.B)] || a.Round != 1 {
			t.Errorf("après les résultats de la première vague, proposition inattendue : %s (ronde %d)", a, a.Round)
		}
	}
	if n != len(restants) {
		t.Errorf("%d appariements restaient, %d proposés", len(restants), n)
	}
}
