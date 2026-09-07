package tournoi_test

import (
	"encoding/json"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// tournoiDeTest : un tournoi suisse avec n joueurs, prêt à proposer.
func tournoiDeTest(t *testing.T, cfg tournoi.Config, n int) *tournoi.State {
	t.Helper()
	st, _, err := tournoi.New(cfg, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		id := tournoi.PlayerID(string(rune('a' + i)))
		if err := st.Apply(tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// TestTableIndisponibleJamaisAttribuee : une table hors service n'accueille aucun match.
func TestTableIndisponibleJamaisAttribuee(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Tables: tournoi.Tables{Count: 4, Unavailable: []int{2}},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 8)
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActStartMatch && a.Table == 2 {
			t.Errorf("la table 2 est indisponible, elle a pourtant été attribuée à %s-%s", a.A, a.B)
		}
	}
}

// TestTableReserveeALaSection : une table réservée à une section n'accueille que celle-là, et
// les autres tables restent ouvertes à tous.
func TestTableReserveeALaSection(t *testing.T) {
	tb := tournoi.Tables{Count: 3, Reserved: []tournoi.TableRule{{Table: 1, Section: "main", AllPhases: true}}}
	if tb.AvailableFor(1, "conso", 0) {
		t.Error("la table 1 est réservée au principal : elle ne doit pas accueillir la consolante")
	}
	if !tb.AvailableFor(1, "main", 0) {
		t.Error("la table 1 est réservée au principal : elle doit l'accueillir")
	}
	if !tb.AvailableFor(2, "conso", 0) {
		t.Error("la table 2 n'est réservée à rien : elle doit accueillir n'importe quoi")
	}
}

// TestTableReserveeAUnePhase : une réservation peut viser une phase précise (la finale).
func TestTableReserveeAUnePhase(t *testing.T) {
	tb := tournoi.Tables{Count: 3, Reserved: []tournoi.TableRule{{Table: 1, Phase: 2}}}
	if tb.AvailableFor(1, "", 1) {
		t.Error("la table 1 est réservée à la phase 2")
	}
	if !tb.AvailableFor(1, "", 2) {
		t.Error("la table 1 doit accueillir la phase 2")
	}
}

// TestSansTableLibreLaPropositionLeDit : quand toutes les tables sont prises, la proposition
// reste dans la file avec sa raison, au lieu de sortir sans table et sans explication.
func TestSansTableLibreLaPropositionLeDit(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Tables: tournoi.Tables{Count: 1},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 8)
	acts := st.Propose()
	var avecTable, sansTable int
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		if a.Table > 0 {
			avecTable++
			continue
		}
		sansTable++
		if a.Reason != tournoi.ReasonWaitingTable {
			t.Errorf("match %s-%s sans table : raison %q, attendu %q", a.A, a.B, a.Reason, tournoi.ReasonWaitingTable)
		}
	}
	if avecTable != 1 {
		t.Errorf("une seule table : %d match(s) placé(s)", avecTable)
	}
	if sansTable == 0 {
		t.Error("les matchs restants devaient être proposés en attente de table")
	}
}

// TestChangementDeTable : un match en cours se déplace, et le déplacement se rejoue.
func TestChangementDeTable(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Tables: tournoi.Tables{Count: 4},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 4)
	acts := st.Propose()
	if len(acts) == 0 {
		t.Fatal("aucune proposition")
	}
	ev, err := st.EventFromAction(acts[0], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(ev); err != nil {
		t.Fatal(err)
	}
	chg := tournoi.TableChangedEvent(ev.MatchID, 9, time.Now())
	if err := st.Apply(chg); err != nil {
		t.Fatal(err)
	}
	if got := st.Matches[ev.MatchID].Table; got != 9 {
		t.Errorf("table après déplacement : %d, attendu 9", got)
	}
	if chg.Version != tournoi.JournalVersion {
		t.Errorf("l'événement de déplacement doit porter la version du journal")
	}
	// Un match inconnu est refusé, pas ignoré en silence.
	if err := st.Apply(tournoi.TableChangedEvent("M999", 3, time.Now())); err == nil {
		t.Error("déplacer un match inconnu doit échouer")
	}
}

// TestTablesLitLesDeuxFormes : une configuration écrite « tables: 12 » avant les exceptions
// reste lisible.
func TestTablesLitLesDeuxFormes(t *testing.T) {
	var a tournoi.Tables
	if err := json.Unmarshal([]byte(`12`), &a); err != nil {
		t.Fatal(err)
	}
	if a.Count != 12 {
		t.Errorf("forme ancienne : Count = %d, attendu 12", a.Count)
	}
	var b tournoi.Tables
	if err := json.Unmarshal([]byte(`{"count":8,"unavailable":[3]}`), &b); err != nil {
		t.Fatal(err)
	}
	if b.Count != 8 || len(b.Unavailable) != 1 || b.Unavailable[0] != 3 {
		t.Errorf("forme structurée mal lue : %+v", b)
	}
}
