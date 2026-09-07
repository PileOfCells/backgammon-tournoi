package players_test

import (
	"reflect"
	"strings"
	"testing"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/players"
)

// TestAllerRetourSansPerte : l'annuaire de l'hôte s'exporte et se réimporte d'un tournoi à
// l'autre. Ce qui sort doit rentrer — accents et virgules dans les noms compris, puisque les
// noms français en sont pleins.
func TestAllerRetourSansPerte(t *testing.T) {
	in := []tournoi.Player{
		{ID: "kevin-unger", Name: "Kévin Unger", Club: "Paris", Rating: 4.25},
		{ID: "chloe-de-l-etang", Name: "Chloé de l'Étang", Club: "Aix-en-Provence", Rating: 7},
		{ID: "durand-jean-pierre", Name: "Durand, Jean-Pierre", Club: "", Rating: 12.5},
		{ID: "inconnu", Name: "Inconnu", Club: "Lyon"},
	}
	out, err := players.FromCSV(players.ToCSV(in))
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("aller-retour :\n  écrit %+v\n  relu  %+v", in, out)
	}
}

// TestCoteInconnueResteInconnue : une cote absente ne doit pas devenir 0 dans le fichier — 0
// est la cote d'un joueur parfait.
func TestCoteInconnueResteInconnue(t *testing.T) {
	b := players.ToCSV([]tournoi.Player{{ID: "sans-cote", Name: "Sans cote"}})
	lignes := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lignes) != 2 {
		t.Fatalf("une en-tête et une ligne attendues : %q", string(b))
	}
	if !strings.HasSuffix(strings.TrimRight(lignes[1], "\r"), ";") {
		t.Errorf("la cote inconnue doit sortir en cellule vide : %q", lignes[1])
	}
	out, err := players.FromCSV(b)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Rating != 0 {
		t.Errorf("cote relue %v, attendu 0", out[0].Rating)
	}
}

// TestColonnesDemandees : nom, club, cote — les trois colonnes qu'un organisateur ouvre dans un
// tableur, sans identifiant technique quand il n'en faut pas.
func TestColonnesDemandees(t *testing.T) {
	b := players.ToCSV([]tournoi.Player{{ID: "alice-martin", Name: "Alice Martin", Club: "Nice", Rating: 5}})
	tête := strings.SplitN(string(b), "\n", 2)[0]
	if strings.TrimRight(tête, "\r") != "nom;club;cote" {
		t.Errorf("en-tête %q, attendu \"nom;club;cote\"", tête)
	}
}

// TestIdentifiantExotiqueConserve : un identifiant venu d'ailleurs (autre logiciel, fédération)
// ne se déduit pas du nom ; il ne doit pas disparaître en silence.
func TestIdentifiantExotiqueConserve(t *testing.T) {
	in := []tournoi.Player{{ID: "FFBG-4172", Name: "Alice Martin", Club: "Nice", Rating: 5}}
	b := players.ToCSV(in)
	if !strings.HasPrefix(string(b), "id;") {
		t.Errorf("la colonne id doit apparaître quand elle est nécessaire : %q", string(b))
	}
	out, err := players.FromCSV(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("identifiant perdu : %+v", out)
	}
}

// TestHomonymes : deux joueurs du même nom reçoivent des identifiants distincts, et l'aller-
// retour les conserve.
func TestHomonymes(t *testing.T) {
	in := []tournoi.Player{
		{ID: "jean-martin", Name: "Jean Martin", Club: "Paris"},
		{ID: "jean-martin-2", Name: "Jean Martin", Club: "Lyon"},
	}
	out, err := players.FromCSV(players.ToCSV(in))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("homonymes :\n  écrit %+v\n  relu  %+v", in, out)
	}
}

// TestListeVide : un annuaire vide s'exporte et se relit sans erreur.
func TestListeVide(t *testing.T) {
	out, err := players.FromCSV(players.ToCSV(nil))
	if err != nil {
		t.Fatalf("une liste vide n'est pas une erreur : %v", err)
	}
	if len(out) != 0 {
		t.Errorf("%d joueurs relus, attendu 0", len(out))
	}
}

// TestSeparateursDetectes : les fichiers du monde réel viennent en point-virgule, en virgule ou
// en tabulation.
func TestSeparateursDetectes(t *testing.T) {
	for _, cas := range []struct{ nom, csv string }{
		{"point-virgule", "nom;club;cote\nAlice;Paris;4,2\n"},
		{"virgule", "nom,club,cote\nAlice,Paris,4.2\n"},
		{"tabulation", "nom\tclub\tcote\nAlice\tParis\t4.2\n"},
	} {
		ps, err := players.FromCSV([]byte(cas.csv))
		if err != nil {
			t.Fatalf("%s : %v", cas.nom, err)
		}
		if len(ps) != 1 || ps[0].Name != "Alice" || ps[0].Club != "Paris" || ps[0].Rating != 4.2 {
			t.Errorf("%s : %+v", cas.nom, ps)
		}
	}
}

// TestPrenomEtNomSepares : les exports HelloAsso donnent prénom et nom en deux colonnes.
func TestPrenomEtNomSepares(t *testing.T) {
	ps, err := players.FromCSV([]byte("prénom;nom de famille;club\nJean;Dupont;Nantes\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "Jean Dupont" {
		t.Fatalf("nom recomposé : %+v", ps)
	}
	if ps[0].ID != "jean-dupont" {
		t.Errorf("identifiant %q, attendu \"jean-dupont\"", ps[0].ID)
	}
}

// TestPrenomNestPasUneCote : « prénom » contient « pr ». Un fichier qui a par ailleurs une vraie
// colonne de cote ne doit pas voir ses prénoms lus comme des cotes.
func TestPrenomNestPasUneCote(t *testing.T) {
	ps, err := players.FromCSV([]byte("prénom;nom de famille;cote\nJean;Dupont;9,5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("%d joueurs", len(ps))
	}
	if ps[0].Rating != 9.5 {
		t.Errorf("cote %v, attendu 9.5 — la colonne « prénom » a été prise pour la cote", ps[0].Rating)
	}
	if ps[0].Name != "Jean Dupont" {
		t.Errorf("nom %q", ps[0].Name)
	}
}

// TestSansColonneDeNom : un fichier illisible se dit, il ne rend pas une liste vide.
func TestSansColonneDeNom(t *testing.T) {
	if _, err := players.FromCSV([]byte("colonne;autre\n1;2\n")); err == nil {
		t.Error("un CSV sans colonne de nom doit être refusé")
	}
	if _, err := players.FromCSV(nil); err == nil {
		t.Error("un CSV vide doit être refusé")
	}
}

// TestBOMEtLignesVides : les tableurs Windows écrivent un BOM et laissent des lignes vides.
func TestBOMEtLignesVides(t *testing.T) {
	ps, err := players.FromCSV([]byte("\xef\xbb\xbfnom;club;cote\nAlice;Paris;4\n;;\nBob;Lyon;6\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].Name != "Alice" || ps[1].Name != "Bob" {
		t.Errorf("%+v", ps)
	}
}
