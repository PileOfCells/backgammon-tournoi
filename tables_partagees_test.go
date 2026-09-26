package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// lancerTout confirme toutes les propositions de match et renvoie les événements appliqués.
func lancerTout(t *testing.T, st *tournoi.State, now time.Time) []tournoi.Event {
	t.Helper()
	var evs []tournoi.Event
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
		evs = append(evs, ev)
	}
	return evs
}

func avertissements(st *tournoi.State, code tournoi.WarningCode) []tournoi.Warning {
	var out []tournoi.Warning
	for _, w := range st.Warnings {
		if w.Code == code {
			out = append(out, w)
		}
	}
	return out
}

// TestTablePartageeSignalee (N15) : déplacer un match sur une table qui porte déjà un match en
// cours est accepté — le moteur avertit, il ne bloque pas — et l'avertissement disparaît dès que
// l'un des deux matchs se termine.
func TestTablePartageeSignalee(t *testing.T) {
	now := time.Date(2026, 9, 26, 14, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Tables: tournoi.Tables{Count: 8},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 8)
	evs := lancerTout(t, st, now)
	if len(evs) < 2 {
		t.Fatalf("il faut au moins deux matchs en cours, %d lancés", len(evs))
	}
	m1, m2 := evs[0], evs[1]
	if err := st.Apply(tournoi.TableChangedEvent(m1.MatchID, m2.Table, now)); err != nil {
		t.Fatalf("le déplacement sur une table occupée doit être accepté : %v", err)
	}
	ws := avertissements(st, tournoi.WarnTableShared)
	if len(ws) != 1 {
		t.Fatalf("un avertissement table_shared attendu, %d : %v", len(ws), st.Warnings)
	}
	w := ws[0]
	if w.Table != m2.Table {
		t.Errorf("table signalée %d, attendu %d", w.Table, m2.Table)
	}
	paire := map[tournoi.MatchID]bool{w.Match: true, w.Other: true}
	if !paire[m1.MatchID] || !paire[m2.MatchID] {
		t.Errorf("l'avertissement doit nommer %s et %s : %+v", m1.MatchID, m2.MatchID, w)
	}
	// Tant que les deux jouent, l'avertissement reste, même après un événement sans rapport.
	if err := st.Apply(tournoi.NoteEvent("rien", now)); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(tournoi.ResultEvent(evs[2].MatchID, evs[2].A, 5, 0, now)); err != nil {
		t.Fatal(err)
	}
	if len(avertissements(st, tournoi.WarnTableShared)) != 1 {
		t.Fatalf("l'avertissement doit persister tant que les deux matchs sont en cours : %v", st.Warnings)
	}
	if err := st.Apply(tournoi.ResultEvent(m2.MatchID, m2.A, 5, 0, now)); err != nil {
		t.Fatal(err)
	}
	if ws := avertissements(st, tournoi.WarnTableShared); len(ws) != 0 {
		t.Fatalf("un des deux matchs est fini : l'avertissement doit disparaître, reste %v", ws)
	}
}

// TestTablesOccupeesAilleurs (N22) : 14 tables dont 7 occupées par une autre épreuve de la
// salle ; aucune proposition ne les prend, et ce qui ne trouve pas de table attend.
func TestTablesOccupeesAilleurs(t *testing.T) {
	now := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "Speed", Tables: tournoi.Tables{Count: 14},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 3}}}, 32)
	// Le tirage d'abord.
	for _, a := range st.ProposeAt(now) {
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	ailleurs := []int{1, 2, 3, 4, 5, 6, 7}
	occupée := map[int]bool{}
	for _, n := range ailleurs {
		occupée[n] = true
	}
	acts := st.ProposeWith(now, tournoi.External{BusyTables: ailleurs})
	avecTable, enAttente := 0, 0
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		if occupée[a.Table] {
			t.Errorf("%s-%s proposé sur la table %d, occupée par une autre épreuve", a.A, a.B, a.Table)
		}
		if a.Table == 0 {
			if a.Reason != tournoi.ReasonWaitingTable {
				t.Errorf("sans table, la proposition doit attendre (waiting_table), raison %q", a.Reason)
			}
			enAttente++
		} else {
			avecTable++
		}
	}
	if avecTable != 7 {
		t.Errorf("7 tables libres : 7 propositions avec table attendues, %d", avecTable)
	}
	if enAttente == 0 {
		t.Error("16 matchs pour 7 tables libres : certains doivent attendre")
	}
	// L'extérieur rend ses tables : elles redeviennent attribuables.
	libres := 0
	for _, a := range st.ProposeWith(now, tournoi.External{}) {
		if a.Kind == tournoi.ActStartMatch && a.Table > 0 {
			libres++
		}
	}
	if libres != 14 {
		t.Errorf("les tables rendues : 14 propositions avec table attendues, %d", libres)
	}
	// ProposeAt est ProposeWith sans extérieur.
	if len(st.ProposeAt(now)) != len(st.ProposeWith(now, tournoi.External{})) {
		t.Error("ProposeAt et ProposeWith sans extérieur doivent proposer la même chose")
	}
}
