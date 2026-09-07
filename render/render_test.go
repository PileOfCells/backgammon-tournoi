package render

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

var maj = flag.Bool("update", false, "réécrire les fichiers témoins de testdata/")

// Les fichiers témoins (« golden files ») figent le rendu à l'octet près. Toute modification du
// rendu les fait échouer : c'est le but — on relit alors le diff, et on régénère avec
// `go test ./render -update` si le changement est voulu.
func témoin(t *testing.T, nom, produit string) {
	t.Helper()
	chemin := filepath.Join("testdata", nom)
	if *maj {
		if err := os.WriteFile(chemin, []byte(produit), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	attendu, err := os.ReadFile(chemin)
	if err != nil {
		t.Fatalf("%s : %v (lancer `go test ./render -update` pour le créer)", nom, err)
	}
	if string(attendu) != produit {
		t.Errorf("%s : le rendu a changé (%d octets au lieu de %d). Relire le diff, puis "+
			"`go test ./render -update` si c'est voulu.\n--- attendu ---\n%s\n--- produit ---\n%s",
			nom, len(produit), len(attendu), extrait(string(attendu), produit), extrait(produit, string(attendu)))
	}
}

// extrait : les 200 octets autour de la première différence, pour un message lisible.
func extrait(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	deb, fin := i-60, i+140
	if deb < 0 {
		deb = 0
	}
	if fin > len(a) {
		fin = len(a)
	}
	return a[deb:fin]
}

// joueurs : des noms sans mot français, pour que le test « aucun libellé en dur » ne confonde
// pas une donnée saisie avec un libellé du rendu.
func joueurs(n int) []tournoi.Player {
	out := make([]tournoi.Player, n)
	for i := range out {
		out[i] = tournoi.Player{
			ID:     tournoi.PlayerID(fmt.Sprintf("p%02d", i+1)),
			Name:   fmt.Sprintf("Nom%02d", i+1),
			Club:   fmt.Sprintf("C%d", i%4),
			Rating: 3 + float64(i)/2,
		}
	}
	return out
}

// état : un tournoi joué puis rejoué jusqu'à `fraction` de son journal — un instantané de milieu
// de tournoi, où il y a des matchs en cours, des tables occupées et un arbre à moitié rempli.
func état(t *testing.T, cfg tournoi.Config, n int, fraction float64) (*tournoi.State, time.Time) {
	t.Helper()
	r := sim.Run(cfg, joueurs(n), sim.Options{Seed: 42})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	k := int(float64(len(r.Journal)) * fraction)
	if k < 1 {
		k = 1
	}
	st, err := tournoi.Replay(r.Journal[:k])
	if err != nil {
		t.Fatal(err)
	}
	return st, st.Last
}

// étatAvantLot : le journal coupé juste avant le départ du lot numéro `lot` (1 = le premier),
// c'est-à-dire au moment où le moteur va le proposer.
func étatAvantLot(t *testing.T, cfg tournoi.Config, n, lot int) (*tournoi.State, time.Time) {
	t.Helper()
	r := sim.Run(cfg, joueurs(n), sim.Options{Seed: 42})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	vus, précédent := 0, time.Time{}
	for i, ev := range r.Journal {
		if ev.Kind != tournoi.EvMatchStarted {
			continue
		}
		if ev.Time.Equal(précédent) {
			continue
		}
		précédent = ev.Time
		vus++
		if vus == lot {
			st, err := tournoi.Replay(r.Journal[:i])
			if err != nil {
				t.Fatal(err)
			}
			return st, st.Last
		}
	}
	t.Fatalf("le lot %d n'existe pas", lot)
	return nil, time.Time{}
}

// étatRéparation : un tableau dont un quart a été corrigé après coup, si bien que la demi-finale
// a été jouée par le mauvais joueur. Le moteur propose alors des annulations.
func étatRéparation(t *testing.T) (*tournoi.State, time.Time) {
	t.Helper()
	st, now := état(t, conso(), 16, 1.0)
	for _, id := range st.MatchOrder {
		m := st.Matches[id]
		if m.Label.Kind != tournoi.LabelMainDraw || m.Label.Sub == nil ||
			m.Label.Sub.Kind != tournoi.LabelQuarterFinal || m.Status != tournoi.Finished {
			continue
		}
		now = now.Add(time.Hour)
		if st.Finished {
			if err := st.Apply(tournoi.ReopenedEvent(now)); err != nil {
				t.Fatal(err)
			}
		}
		if err := st.Apply(tournoi.CorrectionEvent(id, m.Loser(), 0, m.Length, now)); err != nil {
			t.Fatal(err)
		}
		return st, now
	}
	t.Fatal("aucun quart de finale joué")
	return nil, time.Time{}
}

func conso() tournoi.Config {
	return tournoi.Config{Name: "Open temoin", MinPerPoint: 8,
		Tables: tournoi.Tables{Count: 10, Unavailable: []int{4},
			Reserved: []tournoi.TableRule{{Table: 1, Section: "main", AllPhases: true}}},
		Prizes: tournoi.PrizePool{EntryFee: 20, Retention: tournoi.Retention{Percent: 10},
			Sections: map[string]tournoi.PrizeScale{tournoi.PrizeSectionAll: {Percents: []float64{50, 25}}}},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true}}}
}

func suisseRondes() tournoi.Config {
	return tournoi.Config{Name: "Suisse temoin", MinPerPoint: 8,
		Tables: tournoi.Tables{Count: 16},
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds"}}}
}

func TestTemoinsDesComposants(t *testing.T) {
	r := New(French())
	st, now := état(t, conso(), 16, 0.55)
	acts := st.ProposeAt(now)
	témoin(t, "arbre_conso.svg", r.BracketBoardSVG(st, st.Phases[st.Current]))
	témoin(t, "tables.html", r.TableGrid(st, now))
	témoin(t, "en_cours.html", r.RunningTable(st, now))
	témoin(t, "classement.html", r.StandingsTable(st))
	témoin(t, "page.html", r.Page(st, acts, now))

	// Une liste d'actions VRAIMENT peuplée : le journal coupé juste avant le départ de la
	// deuxième ronde, quand le moteur propose seize matchs d'un coup.
	av, avNow := étatAvantLot(t, suisseRondes(), 32, 2)
	témoin(t, "actions.html", r.ActionsList(av, av.ProposeAt(avNow)))

	// Et la réparation d'un tableau désaccordé, qui a ses propres actions.
	rep, repNow := étatRéparation(t)
	témoin(t, "actions_reparation.html", r.ActionsList(rep, rep.ProposeAt(repNow)))

	sw, swNow := état(t, suisseRondes(), 32, 0.35)
	// Vingt minutes plus tard : c'est l'heure à laquelle le TD lève la tête vers l'écran, et
	// celle où la colonne « attente » dit quelque chose.
	témoin(t, "vies.html", r.LivesBoard(sw, sw.Phases[sw.Current], swNow.Add(20*time.Minute)))
	témoin(t, "appariements.html", r.PairingSheet(sw, 1))
}

// Une proposition qui attend une table libre porte Table = 0. « Table 0 » sur l'affichage d'une
// salle est un numéro que les joueurs vont chercher entre la 1 et la 2 : la table ne se nomme
// que lorsqu'elle est attribuée.
func TestPasDeTableZeroDansLesActions(t *testing.T) {
	r := New(French())
	// Huit tables pour trente-deux joueurs : seize matchs proposés, huit lancés, huit en
	// attente d'une table.
	cfg := suisseRondes()
	cfg.Tables = tournoi.Tables{Count: 8}
	st, now := étatAvantLot(t, cfg, 32, 1)
	acts := st.ProposeAt(now)

	attente := 0
	for _, a := range acts {
		if a.Kind == tournoi.ActStartMatch && a.Table == 0 {
			attente++
		}
	}
	if attente == 0 {
		t.Fatal("le cas ne se produit pas : le test ne prouve rien")
	}
	html := r.ActionsList(st, acts)
	if strings.Contains(html, "Table 0") {
		t.Errorf("une action nomme la table 0 :\n%s", html)
	}
	// Et la table attribuée, elle, est toujours nommée.
	if !strings.Contains(html, "Table 1") {
		t.Errorf("une action dont la table est attribuée ne la nomme plus :\n%s", html)
	}
}

// TestPageAutonome : la page doit s'ouvrir hors ligne, depuis une clé USB, sur l'ordinateur de
// la salle. Aucune ressource externe, aucun script.
func TestPageAutonome(t *testing.T) {
	r := New(French())
	st, now := état(t, conso(), 16, 0.55)
	for nom, page := range map[string]string{
		"page":         r.Page(st, st.ProposeAt(now), now),
		"appariements": r.PairingSheet(st, 1),
	} {
		for _, interdit := range []string{"<script", "<link", "@import", "src=", "url(http", "//cdn", "http://", "https://"} {
			// xmlns porte une URI de namespace, qui n'est pas une ressource chargée.
			nettoyée := strings.ReplaceAll(page, `xmlns="http://www.w3.org/2000/svg"`, "")
			if strings.Contains(nettoyée, interdit) {
				t.Errorf("%s : contient %q — la page ne serait plus autonome", nom, interdit)
			}
		}
		if !strings.Contains(page, "<style>") {
			t.Errorf("%s : la feuille de style doit être embarquée", nom)
		}
		if !strings.Contains(page, DefaultCredit) {
			t.Errorf("%s : le crédit doit figurer en pied de page", nom)
		}
	}
}

// sentinelles : un Labeler qui ne renvoie que des marqueurs. Ce qui subsiste de français dans
// la sortie est donc écrit EN DUR dans le paquet — ce que l'issue interdit.
type sentinelles struct{}

func (sentinelles) Label(l tournoi.Label) string           { return "[L:" + string(l.Kind) + "]" }
func (sentinelles) Note(n tournoi.Note) string             { return "[N:" + string(n.Kind) + "]" }
func (sentinelles) Reason(r tournoi.ReasonCode) string     { return "[R:" + string(r) + "]" }
func (sentinelles) Warn(w tournoi.WarningCode) string      { return "[W:" + string(w) + "]" }
func (sentinelles) SectionName(s string) string            { return "[S:" + s + "]" }
func (sentinelles) PhaseName(p tournoi.PhaseConfig) string { return "[P:" + p.Kind + "]" }
func (sentinelles) Term(t Term, n int) string              { return "[T:" + string(t) + "]" }

// TestAucunLibelleEnDur : tout passe par le Labeler. Un mot français qui survivrait à un
// Labeler muet serait un mot que l'hôte ne peut pas traduire.
func TestAucunLibelleEnDur(t *testing.T) {
	r := &Renderer{L: sentinelles{}, Lang: "xx", Credit: "CREDIT"}
	st, now := état(t, conso(), 16, 0.55)
	sw, swNow := état(t, suisseRondes(), 32, 0.35)
	pages := map[string]string{
		"page":         r.Page(st, st.ProposeAt(now), now),
		"appariements": r.PairingSheet(sw, 1),
		"vies":         r.LivesBoard(sw, sw.Phases[sw.Current], swNow),
		"tables":       r.TableGrid(st, now),
		"arbre":        r.BracketBoardSVG(st, st.Phases[st.Current]),
		"classement":   r.StandingsTable(st),
	}
	// On regarde le TEXTE, pas le balisage : un nom de classe CSS est un identifiant, pas un
	// libellé, et l'hôte s'en sert pour accrocher sa feuille de style.
	for nom, page := range pages {
		texte := texteSeul(page)
		for terme, mot := range motsFrançais {
			if len([]rune(mot)) < 3 {
				continue // « # », « V » : trop courts pour être cherchés sans faux positifs
			}
			if strings.Contains(texte, mot) {
				t.Errorf("%s : le mot %q (terme %s) est écrit en dur", nom, mot, terme)
			}
		}
		// Aucune lettre accentuée ne subsiste : les données du test n'en portent pas, donc
		// tout accent viendrait d'une phrase écrite en dur. La ponctuation typographique que
		// le rendu utilise comme séparateur n'est pas de la langue.
		for _, ru := range texte {
			if ru < 128 || strings.ContainsRune("—·…’'", ru) {
				continue
			}
			t.Errorf("%s : lettre non ASCII %q dans le texte — une phrase écrite en dur ?", nom, ru)
			break
		}
	}
}

// texteSeul : le contenu textuel d'un fragment HTML ou SVG, sans balises, sans attributs et
// sans feuille de style.
func texteSeul(page string) string {
	if i := strings.Index(page, "<style>"); i >= 0 {
		if j := strings.Index(page[i:], "</style>"); j >= 0 {
			page = page[:i] + page[i+j+len("</style>"):]
		}
	}
	var b strings.Builder
	dansBalise := false
	for _, ru := range page {
		switch {
		case ru == '<':
			dansBalise = true
		case ru == '>':
			dansBalise = false
			b.WriteRune(' ')
		case !dansBalise:
			b.WriteRune(ru)
		}
	}
	return b.String()
}

// TestArbresNeSeChevauchentPas : le principal et la consolante occupent des bandes de colonnes
// disjointes, et les descentes de l'un vers l'autre sont tracées.
func TestArbresNeSeChevauchentPas(t *testing.T) {
	r := New(French())
	st, _ := état(t, conso(), 16, 1.0)
	ph := st.Phases[st.Current]
	svg := r.BracketBoardSVG(st, ph)
	if !strings.Contains(svg, "stroke-dasharray") {
		t.Error("aucune descente tracée entre le principal et la consolante")
	}
	// abscisses occupées par chaque section : deux sections ne doivent pas se recouvrir
	boîtes := regexp.MustCompile(`<rect x="([0-9.]+)"`).FindAllStringSubmatch(svg, -1)
	if len(boîtes) < 10 {
		t.Fatalf("%d boîtes dessinées", len(boîtes))
	}
	// le SVG est assez large pour contenir toutes les boîtes
	vb := regexp.MustCompile(`viewBox="0 0 ([0-9.]+) ([0-9.]+)"`).FindStringSubmatch(svg)
	if vb == nil {
		t.Fatal("pas de viewBox")
	}
	var largeur float64
	fmt.Sscanf(vb[1], "%f", &largeur)
	for _, m := range boîtes {
		var x float64
		fmt.Sscanf(m[1], "%f", &x)
		if x+boxW > largeur {
			t.Errorf("une boîte déborde du SVG : x=%.0f, largeur=%.0f", x, largeur)
		}
	}
	// deux boîtes ne se superposent jamais exactement
	vus := map[string]bool{}
	for _, m := range regexp.MustCompile(`<rect x="([0-9.]+)" y="([0-9.]+)"`).FindAllString(svg, -1) {
		if vus[m] {
			t.Errorf("deux boîtes au même endroit : %s", m)
		}
		vus[m] = true
	}
}

// TestFeuilleDappariementsTientSurA4 : une ronde de 32 joueurs fait 16 lignes, et la feuille
// porte les règles d'impression A4. La pagination réelle ne se teste pas ici — ce qui se teste,
// c'est qu'on n'imprime pas une page sans mise en page.
func TestFeuilleDappariementsTientSurA4(t *testing.T) {
	r := New(French())
	sw, _ := état(t, suisseRondes(), 32, 0.35)
	lots := Batches(sw)
	if len(lots) == 0 {
		t.Fatal("aucun lot")
	}
	if n := len(lots[0]); n != 16 {
		t.Errorf("la première ronde de 32 joueurs compte %d matchs, attendu 16", n)
	}
	feuille := r.PairingSheet(sw, 1)
	lignes := strings.Count(feuille, "<tr>") - 1 // moins l'en-tête
	if lignes != 16 {
		t.Errorf("%d lignes de match, attendu 16", lignes)
	}
	if lignes > 24 {
		t.Errorf("%d lignes ne tiennent pas sur une A4", lignes)
	}
	for _, règle := range []string{"@page", "size:A4", "@media print"} {
		if !strings.Contains(feuille, règle) {
			t.Errorf("la feuille n'a pas la règle d'impression %q", règle)
		}
	}
	if strings.Count(feuille, `class="case"`) != 2*lignes {
		t.Error("chaque match doit avoir deux cases de score à remplir à la main")
	}
}

// TestFeuilleParDefaut : sans numéro, on imprime le dernier lot — celui qu'on a sous les yeux.
func TestFeuilleParDefaut(t *testing.T) {
	r := New(French())
	sw, _ := état(t, suisseRondes(), 32, 0.35)
	n := len(Batches(sw))
	if r.PairingSheet(sw, 0) != r.PairingSheet(sw, n) {
		t.Error("le numéro 0 doit donner le dernier lot")
	}
	if r.PairingSheet(sw, 999) != r.PairingSheet(sw, n) {
		t.Error("un numéro au-delà du dernier doit donner le dernier lot")
	}
}

// TestStyleEtCreditInjectes : l'hôte retrouve sa charte et sa mention.
func TestStyleEtCreditInjectes(t *testing.T) {
	r := &Renderer{L: French(), Style: "body{color:red}", Credit: "Chez Machin", Lang: "de"}
	st, now := état(t, conso(), 16, 0.5)
	page := r.Page(st, nil, now)
	if !strings.Contains(page, "body{color:red}") || strings.Contains(page, "--trait") {
		t.Error("la feuille de style injectée n'a pas remplacé celle par défaut")
	}
	if !strings.Contains(page, "Chez Machin") || strings.Contains(page, DefaultCredit) {
		t.Error("le crédit injecté n'a pas remplacé celui par défaut")
	}
	if !strings.Contains(page, `<html lang="de">`) {
		t.Error("la langue de la page doit être injectable")
	}
}

// TestVuesTemporelles : les colonnes ajoutées au tableau des vies disent bien quelque chose.
func TestVuesTemporelles(t *testing.T) {
	r := New(French())
	sw, now := état(t, suisseRondes(), 32, 0.5)
	vues := r.LivesBoard(sw, sw.Phases[sw.Current], now.Add(20*time.Minute))
	if !strings.Contains(vues, "Adversaires rencontrés") || !strings.Contains(vues, "Attente") {
		t.Fatal("les colonnes adversaires et attente manquent")
	}
	// l'apostrophe des minutes est échappée en &#39; par le rendu HTML
	if !regexp.MustCompile(`<td>[0-9]+(&#39;|h[0-9]{2})</td>`).MatchString(vues) {
		t.Error("aucun temps d'attente affiché")
	}
	if !strings.Contains(vues, "Nom") {
		t.Error("aucun adversaire nommé")
	}
}
