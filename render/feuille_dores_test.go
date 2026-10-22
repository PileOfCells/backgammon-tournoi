package render

import (
	"html"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
)

// Le fichier doré rules/testdata/feuille.json, rejoué ici contre les ENVELOPPES : Batches et
// PairingSheet. L'ordre des lignes est relu dans la feuille rendue elle-même, de sorte que le
// fichier fige ce que le TD imprime, et non ce qu'une fonction intermédiaire prétend.
// `go test ./render -update` le réécrit avec les témoins HTML.

const cheminFeuille = "../rules/testdata/feuille.json"

var ligneFeuille = regexp.MustCompile(`<tr><td>(-?\d+)</td><td>[^<]*</td><td>([^<]*)</td><td class="case"></td><td>([^<]*)</td><td class="case"></td></tr>`)

func casFeuille(t *testing.T) []dores.SheetCase {
	z := time.FixedZone("", 3*3600)
	at := func(h, m int) time.Time { return time.Date(2026, 9, 21, h, m, 0, 0, z) }
	main := []dores.SheetMatch{
		{ID: "M3", Phase: 1, Status: "finished", Start: at(9, 0), Table: 2, Length: 7, A: "a", B: "b"},
		{ID: "M1", Phase: 1, Status: "finished", Start: at(9, 0), Table: 2, Length: 7, A: "c", B: "d"},
		{ID: "M2", Phase: 1, Status: "running", Start: at(9, 0), Table: 1, Length: 7, A: "e", B: "f"},
		{ID: "M4", Phase: 0, Status: "finished", Start: at(8, 0), Table: 1, Length: 5, A: "a", B: "c"},
		{ID: "M5", Phase: 1, Status: "cancelled", Start: at(10, 0), Table: 3, Length: 7, A: "a", B: "d"},
		{ID: "M6", Phase: 1, Status: "running", Start: at(10, 30), Length: 9, A: "a", B: "d"},
		{ID: "M7", Phase: 1, Status: "running", Start: at(10, 30), Table: 4, Length: 9, A: "b", B: "c"},
		{ID: "M10", Phase: 1, Status: "running", Start: at(10, 30), Table: 4, Length: 9, A: "e", B: "g"},
		{ID: "M8", Phase: 1, Status: "running", Start: at(9, 45), Table: 5, Length: 7, A: "h", B: "i"},
	}
	cas := []dores.SheetCase{
		{Name: "avant toute phase", Current: -1, Round: 0, Matches: main},
		{Name: "aucun match dans la phase", Current: 2, Round: 1, Matches: main},
		{Name: "premier lot, tables égales départagées par identifiant", Current: 1, Round: 1, Matches: main},
		{Name: "lot du milieu", Current: 1, Round: 2, Matches: main},
		{Name: "dernier lot, table nulle en tête", Current: 1, Round: 3, Matches: main},
		{Name: "zéro : le plus récent", Current: 1, Round: 0, Matches: main},
		{Name: "négatif : le plus récent", Current: 1, Round: -2, Matches: main},
		{Name: "au-delà : le plus récent", Current: 1, Round: 9, Matches: main},
		{Name: "phase précédente", Current: 0, Round: 1, Matches: main},
	}
	for _, s := range []struct {
		nom  string
		st   *tournoi.State
		lots []int
	}{
		{"suisse par rondes de 32", mustÉtat(t, suisseRondes(), 32, 0.6), []int{1, 2, 0}},
		{"tableau avec consolante de 16", mustÉtat(t, conso(), 16, 0.55), []int{1, 2, 3, 0}},
	} {
		var ms []dores.SheetMatch
		for _, id := range s.st.MatchOrder {
			m := s.st.Matches[id]
			ms = append(ms, dores.SheetMatch{ID: string(m.ID), Phase: m.Phase, Status: string(m.Status), Start: m.Start,
				Table: m.Table, Length: m.Length, A: string(m.A), B: string(m.B)})
		}
		for _, l := range s.lots {
			cas = append(cas, dores.SheetCase{Name: s.nom + ", lot " + strconv.Itoa(l), Current: s.st.Current, Round: l, Matches: ms})
		}
	}
	return cas
}

func mustÉtat(t *testing.T, cfg tournoi.Config, n int, f float64) *tournoi.State {
	st, _ := état(t, cfg, n, f)
	return st
}

// TestDoresFeuille : les lots et l'ordre des lignes de la feuille imprimée.
func TestDoresFeuille(t *testing.T) {
	var cas []dores.SheetCase
	if *maj {
		cas = casFeuille(t)
	} else if err := dores.Read(cheminFeuille, &cas); err != nil {
		t.Fatal(err)
	}
	r := New(French())
	for i := range cas {
		c := &cas[i]
		// Le nom d'un joueur est son identifiant : la feuille rendue se relit sans table de noms.
		st := &tournoi.State{Current: c.Current, Players: map[tournoi.PlayerID]*tournoi.Player{},
			Matches: map[tournoi.MatchID]*tournoi.Match{}}
		for _, m := range c.Matches {
			id := tournoi.MatchID(m.ID)
			st.MatchOrder = append(st.MatchOrder, id)
			st.Matches[id] = &tournoi.Match{ID: id, Phase: m.Phase, Status: tournoi.MatchStatus(m.Status), Start: m.Start,
				Table: m.Table, Length: m.Length, A: tournoi.PlayerID(m.A), B: tournoi.PlayerID(m.B)}
			for _, p := range []string{m.A, m.B} {
				st.Players[tournoi.PlayerID(p)] = &tournoi.Player{ID: tournoi.PlayerID(p), Name: p}
			}
		}
		out := dores.SheetOut{Batches: [][]string{}, Rows: []string{}}
		for _, lot := range Batches(st) {
			ids := []string{}
			for _, m := range lot {
				ids = append(ids, string(m.ID))
			}
			out.Batches = append(out.Batches, ids)
		}
		feuille := r.PairingSheet(st, c.Round)
		if len(out.Batches) > 0 {
			out.Round = c.Round
			if out.Round < 1 || out.Round > len(out.Batches) {
				out.Round = len(out.Batches)
			}
			if !regexp.MustCompile(` ` + strconv.Itoa(out.Round) + `</h2>`).MatchString(feuille) {
				t.Fatalf("%s : la feuille ne porte pas le lot %d", c.Name, out.Round)
			}
			parPaire := map[[2]string]string{}
			for _, id := range out.Batches[out.Round-1] {
				m := st.Matches[tournoi.MatchID(id)]
				parPaire[[2]string{string(m.A), string(m.B)}] = id
			}
			for _, l := range ligneFeuille.FindAllStringSubmatch(feuille, -1) {
				id, ok := parPaire[[2]string{html.UnescapeString(l[2]), html.UnescapeString(l[3])}]
				if !ok {
					t.Fatalf("%s : ligne %v hors du lot", c.Name, l[1:])
				}
				if tab := strconv.Itoa(st.Matches[tournoi.MatchID(id)].Table); tab != l[1] {
					t.Fatalf("%s : table %s imprimée pour %s, qui joue table %s", c.Name, l[1], id, tab)
				}
				out.Rows = append(out.Rows, id)
			}
			if len(out.Rows) != len(out.Batches[out.Round-1]) {
				t.Fatalf("%s : %d lignes pour un lot de %d", c.Name, len(out.Rows), len(out.Batches[out.Round-1]))
			}
		}
		c.Out = out
	}
	if *maj {
		if err := dores.Write(cheminFeuille, cas); err != nil {
			t.Fatal(err)
		}
		return
	}
	ok, produit, err := dores.Same(cheminFeuille, cas)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("%s : la feuille a changé à entrée égale.\n--- produit ---\n%s", cheminFeuille, produit)
	}
}
