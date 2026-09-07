package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// lanceUnMatch : lance la première proposition et renvoie l'événement correspondant.
func lanceUnMatch(t *testing.T, st *tournoi.State) tournoi.Event {
	t.Helper()
	for _, a := range st.Propose() {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		ev, err := st.EventFromAction(a, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		return ev
	}
	t.Fatal("aucun match à lancer")
	return tournoi.Event{}
}

// TestResultatSansScore : le vainqueur est la seule chose exigée. Un directeur note souvent
// « Alice gagne » et rien d'autre ; le classement et les durées ne doivent pas s'en trouver
// faussés.
func TestResultatSansScore(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 4)
	ev := lanceUnMatch(t, st)
	res := tournoi.ResultEvent(ev.MatchID, ev.A, 0, 0, time.Now())
	if err := st.Apply(res); err != nil {
		t.Fatalf("un résultat sans score doit être accepté : %v", err)
	}
	m := st.Matches[ev.MatchID]
	if m.Status != tournoi.Finished || m.Winner != ev.A {
		t.Fatalf("match non terminé correctement : %+v", m)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("un résultat sans score n'est pas une incohérence : %v", st.Warnings)
	}
	r := st.Ranking()
	if len(r) != 4 {
		t.Fatalf("classement incomplet : %d", len(r))
	}
	if r[0].Player != ev.A && r[1].Player != ev.A {
		t.Errorf("le vainqueur doit être devant le perdant")
	}
}

// TestForfaitDUnSeulMatch : le joueur perd ce match sans quitter le tournoi, et suit le chemin
// d'un perdant ordinaire — la consolante, par exemple.
func TestForfaitDUnSeulMatch(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 5, Consolation: true}}}, 8)
	// tirage
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActDraw {
			ev, err := st.EventFromAction(a, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatal(err)
			}
		}
	}
	ev := lanceUnMatch(t, st)
	absent := ev.B
	if err := st.Apply(tournoi.ForfeitEvent(ev.MatchID, ev.A, time.Now())); err != nil {
		t.Fatal(err)
	}
	if st.Withdrawn[absent] {
		t.Error("un forfait de match ne retire pas le joueur du tournoi")
	}
	if !st.Matches[ev.MatchID].Forfeit {
		t.Error("le match doit être marqué forfait")
	}
	// il reste apparaissable : la consolante l'attend
	trouve := false
	for i := 0; i < 40 && !trouve; i++ {
		for _, a := range st.Propose() {
			if a.Kind == tournoi.ActStartMatch && (a.A == absent || a.B == absent) {
				trouve = true
				break
			}
		}
		if trouve {
			break
		}
		// avancer le tournoi
		lancé := false
		for _, a := range st.Propose() {
			if a.Kind != tournoi.ActStartMatch {
				continue
			}
			e, err := st.EventFromAction(a, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(e); err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(tournoi.ResultEvent(e.MatchID, e.A, 5, 2, time.Now())); err != nil {
				t.Fatal(err)
			}
			lancé = true
		}
		if !lancé {
			break
		}
	}
	if !trouve {
		t.Error("le joueur forfait d'un match doit rester dans le tournoi et entrer en consolante")
	}
}

// TestRemarqueSurUnResultat : la remarque voyage dans le journal et ressort au rejeu.
func TestRemarqueSurUnResultat(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 4)
	ev := lanceUnMatch(t, st)
	res := tournoi.ResultEvent(ev.MatchID, ev.A, 5, 3, time.Now()).WithNote("tombé au temps")
	if res.Text != "tombé au temps" {
		t.Fatalf("remarque perdue : %q", res.Text)
	}
	if err := st.Apply(res); err != nil {
		t.Fatal(err)
	}
	j := tournoi.Journal{res}
	b, err := j.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := tournoi.ParseJournal(b)
	if err != nil {
		t.Fatal(err)
	}
	if j2[0].Text != "tombé au temps" {
		t.Errorf("la remarque doit survivre à l'aller-retour du journal : %q", j2[0].Text)
	}
}

// TestRetraitDiffere : le joueur n'est plus apparié mais finit son match en cours ; le retrait
// prend effet à la fin de ce match, et le résultat compte normalement.
func TestRetraitDiffere(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 4)
	ev := lanceUnMatch(t, st)
	partant := ev.A
	if err := st.Apply(tournoi.PlayerWithdrawnAfterCurrentEvent(partant, time.Now())); err != nil {
		t.Fatal(err)
	}
	if st.Withdrawn[partant] {
		t.Error("un retrait différé ne prend pas effet tant que le match est en cours")
	}
	if m := st.Matches[ev.MatchID]; m.Status != tournoi.Running {
		t.Errorf("le match en cours doit continuer, il est %q", m.Status)
	}
	// il gagne son dernier match : le résultat compte
	if err := st.Apply(tournoi.ResultEvent(ev.MatchID, partant, 5, 1, time.Now())); err != nil {
		t.Fatal(err)
	}
	if m := st.Matches[ev.MatchID]; m.Winner != partant || m.Forfeit {
		t.Errorf("le dernier match doit compter normalement : %+v", m)
	}
	if !st.Withdrawn[partant] {
		t.Error("le retrait doit prendre effet une fois le match terminé")
	}
	// et il n'est plus apparié
	for i := 0; i < 5; i++ {
		for _, a := range st.Propose() {
			if a.Kind == tournoi.ActStartMatch && (a.A == partant || a.B == partant) {
				t.Fatalf("le joueur retiré ne doit plus être apparié (%s-%s)", a.A, a.B)
			}
		}
		break
	}
}

// TestRetraitDiffereSansMatchEnCours : sans match en cours, un retrait différé est un retrait
// immédiat — il n'y a rien à finir.
func TestRetraitDiffereSansMatchEnCours(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, 4)
	if err := st.Apply(tournoi.PlayerWithdrawnAfterCurrentEvent("a", time.Now())); err != nil {
		t.Fatal(err)
	}
	if !st.Withdrawn["a"] {
		t.Error("sans match en cours, le retrait différé est immédiat")
	}
}

// TestRetraitDiffereSeRejoue : le rejeu du journal donne le même état.
func TestRetraitDiffereSeRejoue(t *testing.T) {
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}
	st, created, err := tournoi.New(cfg, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	j := tournoi.Journal{created}
	add := func(ev tournoi.Event) {
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		j = append(j, ev)
	}
	for i := 0; i < 4; i++ {
		id := tournoi.PlayerID(string(rune('a' + i)))
		add(tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, time.Now()))
	}
	a := st.Propose()[0]
	ev, err := st.EventFromAction(a, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	add(ev)
	add(tournoi.PlayerWithdrawnAfterCurrentEvent(ev.A, time.Now()))
	add(tournoi.ResultEvent(ev.MatchID, ev.A, 5, 1, time.Now()))

	st2, err := tournoi.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	if st2.Withdrawn[ev.A] != st.Withdrawn[ev.A] {
		t.Errorf("rejeu : retrait %v, attendu %v", st2.Withdrawn[ev.A], st.Withdrawn[ev.A])
	}
	if st2.NEvents != st.NEvents {
		t.Errorf("rejeu : %d événements, attendu %d", st2.NEvents, st.NEvents)
	}
	if len(st2.Warnings) != 0 {
		t.Errorf("rejeu : avertissements inattendus %v", st2.Warnings)
	}
}
