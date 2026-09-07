package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// tire : confirme le tirage proposé (tableaux, poules, blocs).
func tire(t *testing.T, st *tournoi.State) {
	t.Helper()
	for _, a := range st.Propose() {
		if a.Kind != tournoi.ActDraw {
			continue
		}
		ev, err := st.EventFromAction(a, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("aucun tirage proposé")
}

// longueursParTour : la longueur de chaque tour de la section, du premier au dernier.
func longueursParTour(t *testing.T, sec *tournoi.Section) []int {
	t.Helper()
	var out []int
	for _, idx := range sec.Rounds {
		if len(idx) == 0 {
			continue
		}
		l := sec.Matches[idx[0]].Length
		for _, i := range idx {
			if sec.Matches[i].Length != l {
				t.Fatalf("section %s : deux longueurs dans le même tour (%d et %d)", sec.Name, l, sec.Matches[i].Length)
			}
		}
		out = append(out, l)
	}
	return out
}

// TestLongueursParTour : « 15, 13, 11, 9 » se lit du dernier tour vers le premier — c'est
// l'ordre dans lequel un organisateur annonce son tournoi.
func TestLongueursParTour(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 7, Lengths: []int{15, 13, 11, 9}}}}, 16)
	tire(t, st)
	got := longueursParTour(t, st.Phases[0].Sections[0])
	want := []int{9, 11, 13, 15}
	if len(got) != len(want) {
		t.Fatalf("tableau de 16 : %d tours, attendu %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tour %d : %d points, attendu %d (%v)", i+1, got[i], want[i], got)
		}
	}
}

// TestLongueursPlusCourtesQueLeTableau : une liste plus courte que le nombre de tours ne
// provoque aucune erreur — les tours qu'elle ne couvre pas gardent la longueur par défaut.
func TestLongueursPlusCourtesQueLeTableau(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 7, Lengths: []int{15, 13}}}}, 16)
	tire(t, st)
	got := longueursParTour(t, st.Phases[0].Sections[0])
	want := []int{7, 7, 13, 15}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tour %d : %d points, attendu %d (%v)", i+1, got[i], want[i], got)
		}
	}
}

// TestLongueursEtFinalLength : Lengths est plus précis que FinalLength et l'emporte sur lui.
func TestLongueursEtFinalLength(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 7, FinalLength: 11, Lengths: []int{15}}}}, 8)
	tire(t, st)
	got := longueursParTour(t, st.Phases[0].Sections[0])
	if got[len(got)-1] != 15 {
		t.Errorf("finale : %d points, attendu 15 (%v)", got[len(got)-1], got)
	}
	if got[0] != 7 {
		t.Errorf("premier tour : %d points, attendu 7 (%v)", got[0], got)
	}
}

// TestSuisseAllongeSousLeSeuil : sous le seuil, les matchs proposés sont les longs, et la durée
// attendue suit.
func TestSuisseAllongeSousLeSeuil(t *testing.T) {
	cfg := func(seuil int) tournoi.Config {
		return tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
			{Kind: tournoi.KindSwissLives, Length: 5, LengthLate: 11, LateThreshold: seuil}}}
	}
	// huit joueurs en vie, seuil 4 : on est encore au début, les matchs restent courts
	st := tournoiDeTest(t, cfg(4), 8)
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActStartMatch && a.Length != 5 {
			t.Errorf("au-dessus du seuil : match en %d points, attendu 5", a.Length)
		}
	}
	// huit joueurs en vie, seuil 8 : la fin de phase est là, les matchs s'allongent
	st = tournoiDeTest(t, cfg(8), 8)
	vu := false
	for _, a := range st.Propose() {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		vu = true
		if a.Length != 11 {
			t.Errorf("sous le seuil : match en %d points, attendu 11", a.Length)
		}
		if got, want := st.Expected(a.Length), st.Expected(11); got != want {
			t.Errorf("durée attendue %s, attendu %s", got, want)
		}
	}
	if !vu {
		t.Fatal("aucun match proposé")
	}
}

// TestLongueurChoisieParLeTDLEmporte : le TD dirige. S'il a changé la longueur de la phase à la
// main, la règle automatique de fin de suisse ne la lui reprend pas.
func TestLongueurChoisieParLeTDLEmporte(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 5, LengthLate: 11, LateThreshold: 8}}}, 8)
	if err := st.Apply(tournoi.LengthChangedEvent(0, 3, time.Now())); err != nil {
		t.Fatal(err)
	}
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActStartMatch && a.Length != 3 {
			t.Errorf("longueur imposée par le TD : match en %d points, attendu 3", a.Length)
		}
	}
}

// TestLongueursInvalidesRefusees : une longueur nulle ou négative est une faute de saisie, pas
// un défaut à combler en silence.
func TestLongueursInvalidesRefusees(t *testing.T) {
	c := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 7, Lengths: []int{15, 0}}}}
	if err := c.Validate(); err == nil {
		t.Error("une longueur nulle dans Lengths doit être refusée")
	}
	c = tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, LengthLate: 11}}}
	if err := c.Validate(); err == nil {
		t.Error("length_late sans seuil doit être refusé")
	}
}
