package tournoi_test

import (
	"math/rand"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// tableauDe8 : un tableau de 8 joueurs, tiré, dont on joue `tours` tours. Le vainqueur est
// toujours le premier joueur de la proposition, si bien que le tournoi est reproductible.
func tableauDe8(t *testing.T, tours int) (*tournoi.State, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 9}}}, 5, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(8, 6, 2, 2, 10, rand.New(rand.NewSource(5))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	for joués := 0; joués < tours; {
		var départs []tournoi.Action
		tiré := false
		for _, a := range st.ProposeAt(now) {
			switch a.Kind {
			case tournoi.ActDraw:
				now = now.Add(time.Minute)
				ev, err := st.EventFromAction(a, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := st.Apply(ev); err != nil {
					t.Fatal(err)
				}
				tiré = true
			case tournoi.ActStartMatch:
				départs = append(départs, a)
			}
			if tiré {
				break
			}
		}
		if tiré {
			continue
		}
		if len(départs) == 0 {
			break
		}
		for _, a := range départs {
			now = now.Add(time.Minute)
			ev, err := st.EventFromAction(a, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatal(err)
			}
		}
		for _, m := range st.Running() {
			now = now.Add(time.Minute)
			if err := st.Apply(tournoi.ResultEvent(m.ID, m.A, m.Length, 3, now)); err != nil {
				t.Fatal(err)
			}
		}
		joués++
	}
	return st, now
}

// matchsPar : les matchs joués d'un libellé donné, dans l'ordre du journal.
func matchsPar(st *tournoi.State, kind tournoi.LabelKind) []*tournoi.Match {
	var out []*tournoi.Match
	for _, id := range st.MatchOrder {
		if m := st.Matches[id]; m.Label.Kind == kind && m.Status != tournoi.Cancelled {
			out = append(out, m)
		}
	}
	return out
}

func annulations(acts []tournoi.Action) []tournoi.Action {
	var out []tournoi.Action
	for _, a := range acts {
		if a.Kind == tournoi.ActCancelMatch {
			out = append(out, a)
		}
	}
	return out
}

// TestReparationAUnTour : corriger le vainqueur d'un quart déjà suivi d'une demi-finale produit
// la proposition d'annuler la demi-finale, puis celle du bon match.
func TestReparationAUnTour(t *testing.T) {
	st, now := tableauDe8(t, 2) // quarts + demies
	quarts := matchsPar(st, tournoi.LabelQuarterFinal)
	demies := matchsPar(st, tournoi.LabelSemiFinal)
	if len(quarts) != 4 || len(demies) != 2 {
		t.Fatalf("%d quarts et %d demies", len(quarts), len(demies))
	}
	q := quarts[0]
	ancien, nouveau := q.Winner, q.Loser()
	now = now.Add(time.Hour)
	if err := st.Apply(tournoi.CorrectionEvent(q.ID, nouveau, 3, q.Length, now)); err != nil {
		t.Fatal(err)
	}
	if len(st.Warnings) == 0 {
		t.Fatal("la correction devrait désaccorder le tableau")
	}
	acts := st.ProposeAt(now)
	ann := annulations(acts)
	if len(ann) != 1 {
		t.Fatalf("attendu une annulation (la demi-finale), reçu %d : %v", len(ann), acts)
	}
	// c'est bien la demi-finale que le quart alimentait
	demie := st.Matches[ann[0].Match]
	if demie == nil || !demie.Has(ancien) {
		t.Fatalf("l'annulation ne porte pas sur la demi-finale jouée par %s : %v", ancien, ann[0])
	}
	if ann[0].Label.Kind != tournoi.LabelSemiFinal {
		t.Errorf("libellé de l'annulation : %+v", ann[0].Label)
	}
	// on confirme la réparation
	ev, err := st.EventFromAction(ann[0], now)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != tournoi.EvMatchCancelled || ev.MatchID != demie.ID {
		t.Fatalf("événement produit : %+v", ev)
	}
	if err := st.Apply(ev); err != nil {
		t.Fatal(err)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("l'avertissement devrait avoir disparu : %v", st.Warnings)
	}
	// et le bon match est proposé
	acts = st.ProposeAt(now)
	if len(annulations(acts)) != 0 {
		t.Errorf("plus rien à annuler : %v", acts)
	}
	vu := false
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch && a.Label.Kind == tournoi.LabelSemiFinal &&
			(a.A == nouveau || a.B == nouveau) {
			vu = true
		}
	}
	if !vu {
		t.Errorf("la demi-finale devrait être reproposée avec %s : %v", nouveau, acts)
	}
}

// TestReparationADeuxTours : la correction remonte de deux tours. La finale aussi est fausse —
// elle opposait bien les deux joueurs que le graphe attendait, mais pour de mauvaises raisons.
func TestReparationADeuxTours(t *testing.T) {
	st, now := tableauDe8(t, 3) // quarts + demies + finale
	quarts := matchsPar(st, tournoi.LabelQuarterFinal)
	if len(matchsPar(st, tournoi.LabelFinal)) != 1 {
		t.Fatalf("la finale devrait être jouée")
	}
	q := quarts[0]
	now = now.Add(time.Hour)
	if err := st.Apply(tournoi.CorrectionEvent(q.ID, q.Loser(), 3, q.Length, now)); err != nil {
		t.Fatal(err)
	}
	ann := annulations(st.ProposeAt(now))
	if len(ann) != 2 {
		t.Fatalf("attendu deux annulations (demi-finale et finale), reçu %d : %v", len(ann), ann)
	}
	// du plus profond au moins profond : la finale d'abord
	if ann[0].Label.Kind != tournoi.LabelFinal || ann[1].Label.Kind != tournoi.LabelSemiFinal {
		t.Errorf("ordre des annulations : %s puis %s", ann[0].Label.Kind, ann[1].Label.Kind)
	}
	for _, a := range ann {
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements après réparation complète : %v", st.Warnings)
	}
	acts := st.ProposeAt(now)
	if len(annulations(acts)) != 0 {
		t.Errorf("plus rien à annuler : %v", acts)
	}
	if len(acts) == 0 || acts[0].Kind != tournoi.ActStartMatch {
		t.Fatalf("la demi-finale corrigée devrait être proposée : %v", acts)
	}
	// et le tableau se termine
	for étape := 0; !st.Finished && étape < 100; étape++ {
		avancé := false
		for _, a := range st.ProposeAt(now) {
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
			avancé = true
		}
		for _, m := range st.Running() {
			now = now.Add(time.Minute)
			if err := st.Apply(tournoi.ResultEvent(m.ID, m.A, m.Length, 3, now)); err != nil {
				t.Fatal(err)
			}
			avancé = true
		}
		if !avancé {
			break
		}
	}
	if !st.Finished {
		t.Fatal("le tableau réparé ne se termine pas")
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements à la fin : %v", st.Warnings)
	}
}

// TestRienNEstAppliqueDOffice : ne rien confirmer laisse l'état tel quel, avertissement compris.
func TestRienNEstAppliqueDOffice(t *testing.T) {
	st, now := tableauDe8(t, 2)
	q := matchsPar(st, tournoi.LabelQuarterFinal)[0]
	now = now.Add(time.Hour)
	if err := st.Apply(tournoi.CorrectionEvent(q.ID, q.Loser(), 3, q.Length, now)); err != nil {
		t.Fatal(err)
	}
	avant := map[tournoi.MatchID]tournoi.MatchStatus{}
	for _, id := range st.MatchOrder {
		avant[id] = st.Matches[id].Status
	}
	nWarn := len(st.Warnings)
	// on propose trois fois sans rien confirmer
	for i := 0; i < 3; i++ {
		if len(annulations(st.ProposeAt(now))) == 0 {
			t.Fatalf("appel %d : la réparation devrait rester proposée", i)
		}
	}
	for id, statut := range avant {
		if st.Matches[id].Status != statut {
			t.Errorf("match %s : statut %s au lieu de %s — rien ne doit être appliqué d'office",
				id, st.Matches[id].Status, statut)
		}
	}
	if len(st.Warnings) != nWarn {
		t.Errorf("%d avertissements au lieu de %d", len(st.Warnings), nWarn)
	}
}

// TestAnnulationLibereLaPlace : un match de tableau annulé doit pouvoir être rejoué. La place
// gardait l'identifiant du match annulé et n'était plus jamais proposée.
func TestAnnulationLibereLaPlace(t *testing.T) {
	st, now := tableauDe8(t, 1) // quarts joués
	q := matchsPar(st, tournoi.LabelQuarterFinal)[0]
	a, b := q.A, q.B
	if err := st.Apply(tournoi.CancelEvent(q.ID, now)); err != nil {
		t.Fatal(err)
	}
	vu := false
	for _, act := range st.ProposeAt(now) {
		if act.Kind == tournoi.ActStartMatch && ((act.A == a && act.B == b) || (act.A == b && act.B == a)) {
			vu = true
		}
	}
	if !vu {
		t.Errorf("le quart annulé devrait être reproposé (%s contre %s) : %v", a, b, st.ProposeAt(now))
	}
}
