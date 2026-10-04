package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// Repêchage d'un qualifié de poule retiré avant le tirage (règle N26, repechage.go).

// banc : un tournoi mené comme l'hôte le mène — chaque événement confirmé est ajouté au journal,
// que les tests rejouent ensuite.
type banc struct {
	t       *testing.T
	st      *tournoi.State
	journal tournoi.Journal
	now     time.Time
	// vainqueur : qui gagne un match de poule ou de barrage (section, joueurs).
	vainqueur func(section string, a, b tournoi.PlayerID) tournoi.PlayerID
}

func nouveauBanc(t *testing.T, cfg tournoi.Config, ids []tournoi.PlayerID) *banc {
	t.Helper()
	now := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
	st, ev, err := tournoi.New(cfg, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	b := &banc{t: t, st: st, journal: tournoi.Journal{ev}, now: now}
	for _, id := range ids {
		b.applique(tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, b.now))
	}
	return b
}

func (b *banc) applique(ev tournoi.Event) {
	b.t.Helper()
	b.now = b.now.Add(time.Minute)
	ev.Time = b.now
	if err := b.st.Apply(ev); err != nil {
		b.t.Fatalf("%s : %v", ev.Kind, err)
	}
	b.journal = append(b.journal, ev)
}

func (b *banc) confirme(a tournoi.Action) tournoi.Event {
	b.t.Helper()
	ev, err := b.st.EventFromAction(a, b.now)
	if err != nil {
		b.t.Fatal(err)
	}
	b.applique(ev)
	return ev
}

// poules : tire les poules telles quelles (groupes imposés, comme le relevé de la simulation),
// puis joue tous les matchs de poule et de barrage jusqu'à ce que le moteur propose autre chose.
func (b *banc) poules(groupes [][]tournoi.PlayerID) {
	b.t.Helper()
	acts := b.st.Propose()
	if len(acts) == 0 || acts[0].Kind != tournoi.ActDraw {
		b.t.Fatalf("tirage des poules attendu : %v", acts)
	}
	a := acts[0]
	a.Draw = &tournoi.Draw{Groups: groupes}
	b.confirme(a)
	for i := 0; i < 200; i++ {
		var lancés []tournoi.Event
		for _, a := range b.st.Propose() {
			if a.Kind != tournoi.ActStartMatch {
				continue
			}
			lancés = append(lancés, b.confirme(a))
		}
		if len(lancés) == 0 {
			// Un tirage de barrage, s'il y en a un, puis on rejoue.
			acts := b.st.Propose()
			if len(acts) > 0 && acts[0].Kind == tournoi.ActDraw {
				b.confirme(acts[0])
				continue
			}
			return
		}
		for _, ev := range lancés {
			b.applique(tournoi.ResultEvent(ev.MatchID, b.vainqueur(ev.Section, ev.A, ev.B), 5, 0, b.now))
		}
	}
	b.t.Fatal("les poules ne se terminent pas")
}

func (b *banc) retire(p tournoi.PlayerID) {
	b.t.Helper()
	b.applique(tournoi.PlayerWithdrawnEvent(p, b.now))
}

func actionsDe(acts []tournoi.Action, k tournoi.ActionKind) []tournoi.Action {
	var out []tournoi.Action
	for _, a := range acts {
		if a.Kind == k {
			out = append(out, a)
		}
	}
	return out
}

// ---- Scénario T3 de la simulation blunderDB 2026-10 (épreuve A, incident T3-E3) ----

var (
	t3Groupes = [][]tournoi.PlayerID{
		{"karim-delorme", "benoît-carrel", "laure-gauthier", "pauline-brunet"},
		{"chloé-ferrand", "david-pujol", "olivier-caron", "jules-perrin"},
		{"gaëlle-tessier", "elsa-marchand", "nadia-lefort", "fabien-roux"},
		{"inès-vautrin", "agathe-lemoine", "hugo-bastide", "mathis-aubry"},
	}
	// Résultats des poules relevés à l'écran : le vainqueur de chaque paire.
	t3Poules = map[[2]tournoi.PlayerID]tournoi.PlayerID{}
	// Barrage de la poule B (Chloé, David, Olivier à 2 victoires pour 2 places) : David gagne
	// ses deux matchs, Chloé bat Olivier — l'ordre des matchs, tiré par le moteur, n'y change rien.
	t3Barrage = map[tournoi.PlayerID]int{"david-pujol": 3, "chloé-ferrand": 2, "olivier-caron": 1}
	// Le tableau tiré dans la variante repêchage (feuilles/T3.json).
	t3Tableau = []tournoi.PlayerID{"chloé-ferrand", "pauline-brunet", "elsa-marchand", "mathis-aubry",
		"fabien-roux", "david-pujol", "karim-delorme", "agathe-lemoine"}
)

func init() {
	for _, r := range [][3]tournoi.PlayerID{
		{"karim-delorme", "benoît-carrel", "benoît-carrel"}, {"laure-gauthier", "pauline-brunet", "pauline-brunet"},
		{"karim-delorme", "pauline-brunet", "karim-delorme"}, {"benoît-carrel", "laure-gauthier", "benoît-carrel"},
		{"karim-delorme", "laure-gauthier", "karim-delorme"}, {"pauline-brunet", "benoît-carrel", "benoît-carrel"},
		{"chloé-ferrand", "david-pujol", "chloé-ferrand"}, {"olivier-caron", "jules-perrin", "olivier-caron"},
		{"chloé-ferrand", "jules-perrin", "chloé-ferrand"}, {"david-pujol", "olivier-caron", "david-pujol"},
		{"chloé-ferrand", "olivier-caron", "olivier-caron"}, {"jules-perrin", "david-pujol", "david-pujol"},
		{"gaëlle-tessier", "elsa-marchand", "elsa-marchand"}, {"nadia-lefort", "fabien-roux", "fabien-roux"},
		{"gaëlle-tessier", "fabien-roux", "fabien-roux"}, {"elsa-marchand", "nadia-lefort", "elsa-marchand"},
		{"gaëlle-tessier", "nadia-lefort", "gaëlle-tessier"}, {"fabien-roux", "elsa-marchand", "fabien-roux"},
		{"inès-vautrin", "agathe-lemoine", "agathe-lemoine"}, {"hugo-bastide", "mathis-aubry", "mathis-aubry"},
		{"inès-vautrin", "mathis-aubry", "mathis-aubry"}, {"agathe-lemoine", "hugo-bastide", "agathe-lemoine"},
		{"inès-vautrin", "hugo-bastide", "hugo-bastide"}, {"mathis-aubry", "agathe-lemoine", "agathe-lemoine"},
	} {
		t3Poules[[2]tournoi.PlayerID{r[0], r[1]}] = r[2]
		t3Poules[[2]tournoi.PlayerID{r[1], r[0]}] = r[2]
	}
}

// bancT3 : l'épreuve A de T3 (16 joueurs, poules de 4, 2 qualifiés, puis tableau), poules et
// barrage joués comme dans la simulation.
func bancT3(t *testing.T) *banc {
	t.Helper()
	var ids []tournoi.PlayerID
	for _, g := range t3Groupes {
		ids = append(ids, g...)
	}
	b := nouveauBanc(t, tournoi.Config{Name: "Open A", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindRoundRobin, GroupSize: 4, Qualifiers: 2, Length: 5},
		{Kind: tournoi.KindBracket, Length: 7},
	}}, ids)
	b.vainqueur = func(section string, a, c tournoi.PlayerID) tournoi.PlayerID {
		if section == "barrage:B" {
			if t3Barrage[a] > t3Barrage[c] {
				return a
			}
			return c
		}
		w, ok := t3Poules[[2]tournoi.PlayerID{a, c}]
		if !ok {
			t.Fatalf("match de poule %s : %s contre %s absent du relevé", section, a, c)
		}
		return w
	}
	b.poules(t3Groupes)
	return b
}

func contient(ids []tournoi.PlayerID, p tournoi.PlayerID) bool {
	for _, x := range ids {
		if x == p {
			return true
		}
	}
	return false
}

// TestN26_T3E3 rejoue l'incident T3-E3 : Benoît Carrel (poule A, 3-0, qualifié) se retire avant
// le tableau. Le moteur propose de repêcher Pauline Brunet (3e de la poule A, Karim Delorme, 2e,
// étant déjà qualifié) ; la directrice confirme, et le tableau de 8 places se tire sans
// exemption, avec Pauline — la variante « repechage » de l'oracle de la simulation.
func TestN26_T3E3(t *testing.T) {
	b := bancT3(t)
	acts := b.st.Propose()
	if len(acts) != 1 || acts[0].Kind != tournoi.ActNextPhase {
		t.Fatalf("poules finies, sans retrait : seul le passage de phase est attendu : %v", acts)
	}
	b.retire("benoît-carrel")

	acts = b.st.Propose()
	reps := actionsDe(acts, tournoi.ActRepechage)
	if len(reps) != 1 || acts[0].Kind != tournoi.ActRepechage {
		t.Fatalf("un repêchage proposé en tête de file attendu : %v", acts)
	}
	r := reps[0]
	if r.A != "benoît-carrel" || r.B != "pauline-brunet" || r.Section != "poule:A" || r.Phase != 0 {
		t.Fatalf("repêchage proposé : %+v, attendu Pauline Brunet à la place de Benoît Carrel en poule:A", r)
	}
	if r.Label.Kind != tournoi.LabelRepechage || r.Label.Players != 1 {
		t.Errorf("libellé : %+v", r.Label)
	}
	if len(actionsDe(acts, tournoi.ActNextPhase)) != 1 {
		t.Errorf("le passage de phase reste proposé (le TD peut refuser le repêchage) : %v", acts)
	}

	b.confirme(r)
	acts = b.st.Propose()
	if len(acts) != 1 || acts[0].Kind != tournoi.ActNextPhase {
		t.Fatalf("après le repêchage, seul le passage de phase : %v", acts)
	}
	b.confirme(acts[0])

	ph := b.st.Phases[1]
	if len(ph.Entrants) != 8 || !contient(ph.Entrants, "pauline-brunet") || contient(ph.Entrants, "benoît-carrel") {
		t.Fatalf("entrants du tableau : %v", ph.Entrants)
	}
	acts = b.st.Propose()
	if len(acts) != 1 || acts[0].Kind != tournoi.ActDraw {
		t.Fatalf("tirage du tableau attendu : %v", acts)
	}
	for _, p := range acts[0].Draw.Slots {
		if p == tournoi.BYE {
			t.Errorf("tableau tiré avec une exemption : %v", acts[0].Draw.Slots)
		}
	}
	// Le tableau effectivement tiré dans la simulation.
	a := acts[0]
	a.Draw = &tournoi.Draw{Slots: t3Tableau}
	b.confirme(a)

	// Pauline joue le tableau, donc passe devant tous les joueurs restés en poule ; Benoît est
	// classé en poule, retiré, jamais qualifié.
	rp, rb := rangDe(b.st, "pauline-brunet"), rangDe(b.st, "benoît-carrel")
	if rb.Note.Kind != tournoi.NoteWithdrawn || rb.Note.Qualified {
		t.Errorf("Benoît : %+v, attendu retiré, non qualifié", rb)
	}
	if rp.Rank >= rb.Rank || rp.Rank > 8 {
		t.Errorf("Pauline (rang %d) doit être parmi les 8 du tableau, devant Benoît (rang %d)", rp.Rank, rb.Rank)
	}

	// Le rejeu du journal donne le même tableau, sans avertissement.
	re, err := tournoi.Replay(b.journal)
	if err != nil {
		t.Fatal(err)
	}
	if len(re.Warnings) != 0 {
		t.Errorf("avertissements au rejeu : %v", re.Warnings)
	}
	if got := re.Phases[1].Entrants; len(got) != 8 || !contient(got, "pauline-brunet") {
		t.Errorf("entrants au rejeu : %v", got)
	}
}

// TestN26_T3VarianteMoteur : la même épreuve quand la directrice passe outre le repêchage — la
// place devient une exemption, exactement comme avant que N26 soit tranchée. Un journal sans
// événement repechage se rejoue donc comme il a été joué.
func TestN26_T3VarianteMoteur(t *testing.T) {
	b := bancT3(t)
	b.retire("benoît-carrel")
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	if n := len(b.st.Phases[1].Entrants); n != 7 {
		t.Fatalf("sans repêchage, 7 entrants attendus : %d", n)
	}
	acts := b.st.Propose()
	if len(acts) != 2 || acts[0].Kind != tournoi.ActRepechage || acts[1].Kind != tournoi.ActDraw {
		t.Fatalf("passage fait, tirage pas encore : le repêchage reste proposé devant le tirage : %v", acts)
	}
	b.confirme(acts[1])
	byes := 0
	for _, p := range acts[1].Draw.Slots {
		if p == tournoi.BYE {
			byes++
		}
	}
	if byes != 1 {
		t.Errorf("une exemption attendue à la place du retiré : %v", acts[1].Draw.Slots)
	}
	if reps := actionsDe(b.st.Propose(), tournoi.ActRepechage); len(reps) != 0 {
		t.Errorf("après le tirage, plus de repêchage : %v", reps)
	}
	err := b.st.Apply(tournoi.RepechageEvent(0, "poule:A", "benoît-carrel", "pauline-brunet", b.now))
	if err == nil {
		t.Error("un repêchage après le tirage doit être refusé")
	}
}

// TestN26_RetraitApresPassageAvantTirage : le retrait tombe entre le passage de phase et le
// tirage. Le retiré était déjà parmi les entrants ; le repêché prend sa place.
func TestN26_RetraitApresPassageAvantTirage(t *testing.T) {
	b := bancT3(t)
	b.confirme(b.st.Propose()[0]) // passage de phase
	b.retire("karim-delorme")
	acts := b.st.Propose()
	if len(acts) < 2 || acts[0].Kind != tournoi.ActRepechage || acts[0].B != "pauline-brunet" {
		t.Fatalf("repêchage de Pauline attendu devant le tirage : %v", acts)
	}
	b.confirme(acts[0])
	ph := b.st.Phases[1]
	if len(ph.Entrants) != 8 || contient(ph.Entrants, "karim-delorme") || !contient(ph.Entrants, "pauline-brunet") {
		t.Fatalf("entrants : %v", ph.Entrants)
	}
	if ph.Lives["pauline-brunet"] != 1 {
		t.Errorf("vies du repêché : %d", ph.Lives["pauline-brunet"])
	}
	draw := b.st.Propose()[0]
	for _, p := range draw.Draw.Slots {
		if p == tournoi.BYE {
			t.Errorf("exemption au tableau : %v", draw.Draw.Slots)
		}
	}
	if _, err := tournoi.Replay(b.journal); err != nil {
		t.Fatal(err)
	}
}

// TestN26_SuivantRetireLuiAussi : le suivant de la poule est lui-même retiré — on descend au
// suivant ; et un repêché qui se retire à son tour est remplacé de la même façon.
func TestN26_SuivantRetireLuiAussi(t *testing.T) {
	b := bancT3(t)
	b.retire("pauline-brunet")
	b.retire("benoît-carrel")
	acts := actionsDe(b.st.Propose(), tournoi.ActRepechage)
	if len(acts) != 1 || acts[0].B != "laure-gauthier" {
		t.Fatalf("Pauline retirée : Laure (0 victoire) attendue : %v", acts)
	}
	b.confirme(acts[0])

	b.retire("laure-gauthier") // la repêchée part à son tour : plus personne dans la poule A
	if acts := actionsDe(b.st.Propose(), tournoi.ActRepechage); len(acts) != 0 {
		t.Fatalf("poule A épuisée : aucun repêchage attendu : %v", acts)
	}
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	if n := len(b.st.Phases[1].Entrants); n != 7 {
		t.Errorf("poule épuisée : la place devient une exemption, 7 entrants attendus : %d", n)
	}
}

// TestN26_ChoixDuTD : le TD peut repêcher un autre joueur de la poule que celui proposé ; le
// moteur refuse un joueur qualifié, retiré ou d'une autre poule, et un « retiré » qui ne l'est pas.
func TestN26_ChoixDuTD(t *testing.T) {
	b := bancT3(t)
	b.retire("fabien-roux") // poule C : Fabien 3, Elsa 2, Gaëlle 1, Nadia 0
	acts := actionsDe(b.st.Propose(), tournoi.ActRepechage)
	if len(acts) != 1 || acts[0].B != "gaëlle-tessier" || acts[0].Section != "poule:C" {
		t.Fatalf("Gaëlle attendue en poule C : %v", acts)
	}
	for _, mauvais := range []tournoi.Event{
		tournoi.RepechageEvent(0, "poule:C", "fabien-roux", "elsa-marchand", b.now),    // déjà qualifiée
		tournoi.RepechageEvent(0, "poule:C", "fabien-roux", "pauline-brunet", b.now),   // autre poule
		tournoi.RepechageEvent(0, "poule:C", "elsa-marchand", "gaëlle-tessier", b.now), // pas retirée
		tournoi.RepechageEvent(0, "poule:A", "fabien-roux", "pauline-brunet", b.now),   // pas de la poule A
		tournoi.RepechageEvent(1, "poule:C", "fabien-roux", "gaëlle-tessier", b.now),   // pas une phase de poules
	} {
		if err := b.st.Apply(mauvais); err == nil {
			t.Errorf("repêchage accepté à tort : %+v", mauvais)
		}
	}
	b.applique(tournoi.RepechageEvent(0, "poule:C", "fabien-roux", "nadia-lefort", b.now))
	if acts := actionsDe(b.st.Propose(), tournoi.ActRepechage); len(acts) != 0 {
		t.Errorf("place pourvue : plus de proposition : %v", acts)
	}
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	if e := b.st.Phases[1].Entrants; len(e) != 8 || !contient(e, "nadia-lefort") {
		t.Errorf("entrants : %v", e)
	}
}

// TestN26_ExAequo : les suivants sont ex æquo. Le moteur ne départage pas : une proposition par
// candidat, le TD choisit.
func TestN26_ExAequo(t *testing.T) {
	groupes := [][]tournoi.PlayerID{{"a", "b", "c", "d"}, {"e", "f", "g", "h"}}
	b := nouveauBanc(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindRoundRobin, GroupSize: 4, Qualifiers: 1, Length: 5},
		{Kind: tournoi.KindBracket, Length: 7},
	}}, []tournoi.PlayerID{"a", "b", "c", "d", "e", "f", "g", "h"})
	// a et e gagnent tout ; b bat c, c bat d, d bat b : trois ex æquo à une victoire.
	cycle := map[tournoi.PlayerID]tournoi.PlayerID{"b": "c", "c": "d", "d": "b", "f": "g", "g": "h", "h": "f"}
	b.vainqueur = func(_ string, x, y tournoi.PlayerID) tournoi.PlayerID {
		switch {
		case x == "a" || x == "e":
			return x
		case y == "a" || y == "e":
			return y
		case cycle[x] == y:
			return x
		}
		return y
	}
	b.poules(groupes)
	b.retire("a")
	acts := actionsDe(b.st.Propose(), tournoi.ActRepechage)
	if len(acts) != 3 {
		t.Fatalf("trois candidats ex æquo attendus : %v", acts)
	}
	for i, want := range []tournoi.PlayerID{"b", "c", "d"} {
		if acts[i].B != want || acts[i].A != "a" || acts[i].Label.Players != 3 {
			t.Errorf("proposition %d : %+v", i, acts[i])
		}
	}
	b.confirme(acts[2])
	if acts := actionsDe(b.st.Propose(), tournoi.ActRepechage); len(acts) != 0 {
		t.Errorf("le choix du TD clôt la question : %v", acts)
	}
}

// TestN26_RetraitAnnule : un qualifié repêché puis réinscrit avant le tirage retrouve sa place —
// le repêchage ne tient qu'autant que le retrait.
func TestN26_RetraitAnnule(t *testing.T) {
	b := bancT3(t)
	b.retire("benoît-carrel")
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActRepechage)[0])
	b.applique(tournoi.PlayerAddedEvent(tournoi.Player{ID: "benoît-carrel", Name: "benoît-carrel"}, b.now))
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	e := b.st.Phases[1].Entrants
	if len(e) != 8 || !contient(e, "benoît-carrel") || contient(e, "pauline-brunet") {
		t.Errorf("entrants : %v", e)
	}
}
