package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

func rangDe(st *tournoi.State, p tournoi.PlayerID) tournoi.Rank {
	for _, r := range st.Ranking() {
		if r.Player == p {
			return r
		}
	}
	return tournoi.Rank{}
}

// TestRetireClasseSurSonParcoursAuSuisse (N20, S5) : le meilleur joueur part avec trois
// victoires. Il n'est pas dernier : il est classé sur son parcours, comme un joueur éliminé à cet
// instant, avec la note « retiré » — et non « forfait ».
func TestRetireClasseSurSonParcoursAuSuisse(t *testing.T) {
	x := nouveauSuisseMain(t, 6)
	for _, adv := range []tournoi.PlayerID{"b", "c", "d"} {
		x.gagne(x.lance("a", adv), "a")
	}
	x.gagne(x.lance("b", "c"), "b") // c : 2 défaites, 0 victoire → éliminé
	x.now = x.now.Add(time.Minute)
	if err := x.st.Apply(tournoi.PlayerWithdrawnEvent("a", x.now)); err != nil {
		t.Fatal(err)
	}
	ra, rc := rangDe(x.st, "a"), rangDe(x.st, "c")
	if ra.Note.Kind != tournoi.NoteWithdrawn {
		t.Errorf("note du retiré : %q, attendu %q", ra.Note.Kind, tournoi.NoteWithdrawn)
	}
	if ra.Note.Wins != 3 || ra.Note.Losses != 0 {
		t.Errorf("la note doit porter le parcours (3 victoires, 0 défaite) : %+v", ra.Note)
	}
	if ra.Rank >= rc.Rank {
		t.Errorf("le retiré à 3 victoires (rang %d) doit précéder l'éliminé sans victoire (rang %d)", ra.Rank, rc.Rank)
	}
	for _, r := range x.st.Ranking() {
		if r.Note.Kind == tournoi.NoteAlive && r.Rank >= ra.Rank {
			t.Errorf("un joueur en vie (%s, rang %d) doit précéder le retiré (rang %d)", r.Player, r.Rank, ra.Rank)
		}
	}
}

// TestRetireExAequoAvecLesElimines : un retiré est classé ex æquo avec les éliminés qui ont le
// même nombre de victoires — pas de départage, ici comme ailleurs.
func TestRetireExAequoAvecLesElimines(t *testing.T) {
	x := nouveauSuisseMain(t, 6)
	x.gagne(x.lance("a", "b"), "a")
	x.gagne(x.lance("c", "d"), "c")
	x.gagne(x.lance("c", "e"), "e") // c : 1 victoire, 1 défaite
	x.gagne(x.lance("c", "f"), "f") // c : 1 victoire, 2 défaites → éliminé
	x.now = x.now.Add(time.Minute)
	if err := x.st.Apply(tournoi.PlayerWithdrawnEvent("a", x.now)); err != nil { // a : 1 victoire
		t.Fatal(err)
	}
	if ra, rc := rangDe(x.st, "a"), rangDe(x.st, "c"); ra.Rank != rc.Rank {
		t.Errorf("retiré à 1 victoire (rang %d) et éliminé à 1 victoire (rang %d) : ex æquo attendus", ra.Rank, rc.Rank)
	}
}

// TestRetireEnDemiFinale (N20, S2) : une demi-finaliste part pendant sa demi-finale. Elle n'est
// pas dernière : elle est classée comme si elle perdait chacun de ses matchs restants — ici le
// match de consolante qui l'attendait —, ex æquo avec la perdante de ce match-là (#25). Avant
// #25, le forfait de consolante était ignoré et elle passait devant la gagnante de la
// consolante, qui a joué deux matchs de plus.
func TestRetireEnDemiFinale(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7, Consolation: true}}}, 16)
	var partie tournoi.PlayerID
	confirmer := func(a tournoi.Action) tournoi.Event {
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		return ev
	}
	for étape := 0; étape < 300 && !st.Finished; étape++ {
		for _, a := range st.ProposeAt(now) {
			if a.Kind == tournoi.ActWait {
				continue
			}
			ev := confirmer(a)
			if a.Kind != tournoi.ActStartMatch {
				break
			}
			demi := a.Section == "main" && a.Label.Sub != nil && a.Label.Sub.Kind == tournoi.LabelSemiFinal
			if demi && partie == "" {
				partie = a.A
				if err := st.Apply(tournoi.PlayerWithdrawnEvent(partie, now)); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := st.Apply(tournoi.ResultEvent(ev.MatchID, a.B, 3, 7, now)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !st.Finished || partie == "" {
		t.Fatalf("tournoi non terminé (%v) ou personne n'est parti (%q)", st.Finished, partie)
	}
	r := rangDe(st, partie)
	if r.Note.Kind != tournoi.NoteWithdrawn {
		t.Errorf("note de la partante : %+v, attendu %q", r.Note, tournoi.NoteWithdrawn)
	}
	if r.Note.Section == "" || r.Note.Sub == nil {
		t.Errorf("la note doit dire où elle est sortie : %+v", r.Note)
	}
	exæquo := false
	for _, x := range st.Ranking() {
		if x.Player != partie && x.Rank == r.Rank && x.Note.Kind == tournoi.NoteSectionExit &&
			x.Note.Section == r.Note.Section && x.Note.Sub != nil && *x.Note.Sub == *r.Note.Sub {
			exæquo = true
		}
		if x.Note.Kind == tournoi.NoteSectionWinner && x.Rank > r.Rank {
			t.Errorf("la retirée (rang %d) passe devant %s (%+v, rang %d)", r.Rank, x.Player, x.Note, x.Rank)
		}
	}
	if !exæquo || r.Rank >= 16 {
		t.Errorf("une demi-finaliste retirée est classée %d/16, sans ex æquo à son niveau : %+v", r.Rank, r.Note)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements : %v", st.Warnings)
	}
}
