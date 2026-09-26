package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// suisseMain : un suisse 2 vies continu où les matchs sont lancés à la main, pour fixer qui joue
// qui ; lance(a, b) renvoie l'identifiant du match.
type suisseMain struct {
	t   *testing.T
	st  *tournoi.State
	now time.Time
}

func nouveauSuisseMain(t *testing.T, n int) *suisseMain {
	st := tournoiDeTest(t, tournoi.Config{Name: "T",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 5}}}, n)
	return &suisseMain{t: t, st: st, now: time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)}
}

func (x *suisseMain) lance(a, b tournoi.PlayerID) tournoi.MatchID {
	x.t.Helper()
	x.now = x.now.Add(time.Minute)
	id := tournoi.MatchID("M" + string(rune('0'+len(x.st.MatchOrder)+1)))
	ev := tournoi.Event{Version: tournoi.JournalVersion, Kind: tournoi.EvMatchStarted, Time: x.now,
		MatchID: id, A: a, B: b, Length: 5}
	if err := x.st.Apply(ev); err != nil {
		x.t.Fatal(err)
	}
	return id
}

func (x *suisseMain) gagne(id tournoi.MatchID, w tournoi.PlayerID) {
	x.t.Helper()
	x.now = x.now.Add(time.Minute)
	if err := x.st.Apply(tournoi.ResultEvent(id, w, 5, 1, x.now)); err != nil {
		x.t.Fatal(err)
	}
}

func (x *suisseMain) corrige(id tournoi.MatchID, w tournoi.PlayerID) {
	x.t.Helper()
	x.now = x.now.Add(time.Minute)
	if err := x.st.Apply(tournoi.CorrectionEvent(id, w, 5, 2, x.now)); err != nil {
		x.t.Fatal(err)
	}
}

// TestCorrectionEliminantUnJoueurEnMatch (N19, S5) : une correction fait tomber à 0 vie une
// joueuse qui joue en ce moment. Le moteur le signale sur le match et propose de l'annuler.
func TestCorrectionEliminantUnJoueurEnMatch(t *testing.T) {
	x := nouveauSuisseMain(t, 6)
	m1 := x.lance("b", "c")
	x.gagne(m1, "c") // b : 1 défaite
	m2 := x.lance("b", "d")
	x.gagne(m2, "b") // b gagne… à tort
	m3 := x.lance("b", "e")
	x.corrige(m2, "d") // b : 2 défaites, alors qu'elle joue m3
	var w *tournoi.Warning
	for i := range x.st.Warnings {
		if x.st.Warnings[i].Code == tournoi.WarnCorrectionEliminatesRunning {
			w = &x.st.Warnings[i]
		}
	}
	if w == nil {
		t.Fatalf("avertissement correction_eliminates_running attendu : %v", x.st.Warnings)
	}
	if w.Match != m3 || w.Player != "b" {
		t.Errorf("avertissement %+v : attendu match %s, joueuse b", *w, m3)
	}
	annule := false
	for _, a := range x.st.ProposeAt(x.now) {
		if a.Kind == tournoi.ActCancelMatch && a.Match == m3 {
			annule = true
		}
	}
	if !annule {
		t.Fatalf("l'annulation de %s doit être proposée : %v", m3, x.st.ProposeAt(x.now))
	}
	// Le TD confirme l'annulation : l'avertissement tombe, e redevient libre.
	x.now = x.now.Add(time.Minute)
	if err := x.st.Apply(tournoi.CancelEvent(m3, x.now)); err != nil {
		t.Fatal(err)
	}
	for _, w := range x.st.Warnings {
		if w.Code == tournoi.WarnCorrectionEliminatesRunning {
			t.Errorf("après l'annulation, l'avertissement doit disparaître : %v", w)
		}
	}
}

// TestCorrectionRessuscite (N19, S1) : la vraie gagnante, éliminée à tort, revient à une vie.
// Le moteur le dit, et le dit jusqu'à ce qu'elle rejoue.
func TestCorrectionRessuscite(t *testing.T) {
	x := nouveauSuisseMain(t, 6)
	m1 := x.lance("a", "b")
	x.gagne(m1, "b") // a : 1 défaite
	m2 := x.lance("a", "c")
	x.gagne(m2, "c") // a : 2 défaites, éliminée… à tort
	x.corrige(m2, "a")
	revit := func() bool {
		for _, w := range x.st.Warnings {
			if w.Code == tournoi.WarnCorrectionRevives && w.Player == "a" {
				return true
			}
		}
		return false
	}
	if !revit() {
		t.Fatalf("avertissement correction_revives attendu pour a : %v", x.st.Warnings)
	}
	m3 := x.lance("d", "e")
	x.gagne(m3, "d")
	if !revit() {
		t.Fatal("l'avertissement doit persister après un résultat sans rapport")
	}
	x.lance("a", "f")
	if revit() {
		t.Error("a rejoue : l'avertissement a servi, il tombe")
	}
}

// TestSansCorrectionAucunAvertissement : un suisse mené sans correction ne produit aucun des
// deux avertissements (les simulations de sim_test le vérifient à grande échelle).
func TestSansCorrectionAucunAvertissement(t *testing.T) {
	x := nouveauSuisseMain(t, 4)
	m1 := x.lance("a", "b")
	x.gagne(m1, "a")
	m2 := x.lance("b", "c")
	x.gagne(m2, "c")
	if len(x.st.Warnings) != 0 {
		t.Fatal(x.st.Warnings)
	}
}
