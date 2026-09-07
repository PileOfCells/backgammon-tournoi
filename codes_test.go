package tournoi_test

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// champsLibres : les seules chaînes que le moteur laisse passer telles quelles, parce que le TD
// ou l'hôte les a lui-même saisies (nom de joueur, de club, de tournoi, de phase, annotation).
var champsLibres = map[string]bool{
	"text": true, "name": true, "club": true,
}

// estIdentifiant : un code qui sort du moteur est ASCII, sans espace et sans accent — jamais une
// phrase. C'est la forme de tous les codes (« bracket_round », « poule:A », « M12 »).
func estIdentifiant(s string) bool {
	for _, r := range s {
		if r > 127 || r == ' ' {
			return false
		}
	}
	return true
}

// aucunTexteAffichable vérifie que rien de destiné à un humain ne sort du moteur : toute chaîne
// est un identifiant, sauf les champs que le TD a saisis. C'est la raison d'être des codes
// structurés — le logiciel hôte affiche le tournoi dans la langue de son utilisateur, et le
// journal ne doit pas figer une langue.
func aucunTexteAffichable(t *testing.T, v any, contexte string) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s : %v", contexte, err)
	}
	var brut any
	if err := json.Unmarshal(b, &brut); err != nil {
		t.Fatalf("%s : %v", contexte, err)
	}
	var visite func(any, string)
	visite = func(x any, chemin string) {
		switch v := x.(type) {
		case map[string]any:
			for k, e := range v {
				if champsLibres[k] {
					continue
				}
				visite(e, chemin+"."+k)
			}
		case []any:
			for _, e := range v {
				visite(e, chemin)
			}
		case string:
			if !estIdentifiant(v) {
				t.Errorf("%s : %s vaut %q — une phrase destinée à l'affichage, pas un code", contexte, chemin, v)
			}
		}
	}
	visite(brut, contexte)
}

// TestAucunTexteHumainNeSortDuMoteur : ni Propose, ni Ranking, ni Warnings, ni le journal ne
// contiennent de phrase française. C'est la raison d'être des codes structurés : le logiciel
// hôte affiche le tournoi dans la langue de son utilisateur.
func TestAucunTexteHumainNeSortDuMoteur(t *testing.T) {
	for name, cfg := range configs() {
		name, cfg := name, cfg
		t.Run(name, func(t *testing.T) {
			joueurs := sim.Champ(24, 6, 2, 2, 10, rand.New(rand.NewSource(7)))
			var journal tournoi.Journal
			r := sim.Run(cfg, joueurs, sim.Options{Seed: 7, Hook: func(st *tournoi.State, ev tournoi.Event) error {
				journal = append(journal, ev)
				contièntActions(t, st)
				return nil
			}})
			if r.State == nil {
				t.Fatal("pas d'état")
			}
			aucunTexteAffichable(t, r.State.Ranking(), "classement final")
			aucunTexteAffichable(t, r.State.Warnings, "avertissements")
			aucunTexteAffichable(t, r.State.Infos, "informations")
			aucunTexteAffichable(t, journal, "journal")
		})
	}
}

func contièntActions(t *testing.T, st *tournoi.State) {
	t.Helper()
	aucunTexteAffichable(t, st.Propose(), "propositions")
}

// TestRenduFrancaisCouvreLesCodes : chaque code a un rendu français, et aucun ne retombe sur sa
// valeur brute — un code affiché tel quel serait illisible pour un utilisateur.
func TestRenduFrancaisCouvreLesCodes(t *testing.T) {
	labels := []tournoi.Label{
		{Kind: tournoi.LabelSwissGroup, Losses: 1, Match: 3},
		{Kind: tournoi.LabelRound, N: 3},
		{Kind: tournoi.LabelRematch},
		{Kind: tournoi.LabelCrossed},
		{Kind: tournoi.LabelBracketRound, N: 2},
		{Kind: tournoi.LabelFinal},
		{Kind: tournoi.LabelSemiFinal},
		{Kind: tournoi.LabelQuarterFinal},
		{Kind: tournoi.LabelConsolationRound, N: 2},
		{Kind: tournoi.LabelConsolationFinal},
		{Kind: tournoi.LabelLastChance},
		{Kind: tournoi.LabelGrandFinal},
		{Kind: tournoi.LabelGrandFinalRecharge},
		{Kind: tournoi.LabelOpening},
		{Kind: tournoi.LabelWinnersMatch},
		{Kind: tournoi.LabelLosersMatch},
		{Kind: tournoi.LabelDecider},
		{Kind: tournoi.LabelSingleMatch},
		{Kind: tournoi.LabelSingleElim},
		{Kind: tournoi.LabelPoolRound, N: 1},
		{Kind: tournoi.LabelBarrage, Section: "barrage:A"},
		{Kind: tournoi.LabelBarrageCross, Section: "barrage:A"},
		{Kind: tournoi.LabelDrawPools, N: 4},
		{Kind: tournoi.LabelDrawBarrage, Section: "poule:A", Players: 3, Spots: 2},
		{Kind: tournoi.LabelDrawBracket, N: 16},
		{Kind: tournoi.LabelDrawBlock, N: 1, Players: 4},
		{Kind: tournoi.LabelPhase, Text: "Suisse"},
	}
	for _, l := range labels {
		s := l.String()
		if s == "" || s == string(l.Kind) {
			t.Errorf("libellé %s : pas de rendu français (%q)", l.Kind, s)
		}
	}
	// Les libellés composés doivent inclure leur sous-libellé.
	for _, l := range []tournoi.Label{
		{Kind: tournoi.LabelMainDraw},
		{Kind: tournoi.LabelInBlock, N: 2, Section: "B2G1"},
		{Kind: tournoi.LabelInSection, Section: "poule:A"},
	} {
		s := l.String()
		if s == "" || s == string(l.Kind) {
			t.Errorf("libellé composé %s : pas de rendu français (%q)", l.Kind, s)
		}
	}
	notes := []tournoi.Note{
		{Kind: tournoi.NoteWinner}, {Kind: tournoi.NoteFinalist},
		{Kind: tournoi.NoteAlive, Lives: 2}, {Kind: tournoi.NoteRecord, Wins: 3, Losses: 1},
		{Kind: tournoi.NoteForfeit}, {Kind: tournoi.NoteRunning},
		{Kind: tournoi.NoteAwaitingDraw}, {Kind: tournoi.NoteUnranked},
		{Kind: tournoi.NoteSectionExit, Section: "main"},
		{Kind: tournoi.NoteSectionWinner, Section: "conso"},
		{Kind: tournoi.NotePoolRecord, Section: "poule:A", Wins: 2, Qualified: true},
	}
	for _, n := range notes {
		if s := n.String(); s == "" || s == string(n.Kind) {
			t.Errorf("note %s : pas de rendu français (%q)", n.Kind, s)
		}
	}
	for _, r := range []tournoi.ReasonCode{
		tournoi.ReasonMatchesRunning, tournoi.ReasonNoPairing,
		tournoi.ReasonWaitingBatch, tournoi.ReasonWaitingTable,
	} {
		if s := r.String(); s == "" || s == string(r) {
			t.Errorf("raison %s : pas de rendu français (%q)", r, s)
		}
	}
	infos := []tournoi.Info{
		{Code: tournoi.InfoEntersAt, Player: "tard", Phase: 1, Label: tournoi.Label{Kind: tournoi.LabelPhase, Text: "Tableau"}},
		{Code: tournoi.InfoEntersAt, Player: "tard", Phase: 0, Section: "conso"},
		{Code: tournoi.InfoNoEntry, Player: "tard"},
	}
	for _, i := range infos {
		if s := i.String(); s == "" || s == string(i.Code) {
			t.Errorf("information %s : pas de rendu français (%q)", i.Code, s)
		}
	}
	avertissements := []tournoi.Warning{
		{Code: tournoi.WarnBracketWrongPlayers, Match: "M1", Section: "main",
			Label: tournoi.Label{Kind: tournoi.LabelFinal}, A: "a", B: "b", ExpectedA: "c", ExpectedB: "d"},
		{Code: tournoi.WarnScoreOverLength, Match: "M1", Length: 7, ScoreA: 9, ScoreB: 2},
		{Code: tournoi.WarnEndsInBreak, Match: "M1"},
		{Code: tournoi.WarnEndsInBreak}, // porté par une PROPOSITION : il n'y a pas encore de match
		{Code: tournoi.WarnSlowMatch, Match: "M1"},
	}
	for _, w := range avertissements {
		if s := w.String(); s == "" || s == string(w.Code) {
			t.Errorf("avertissement %s : pas de rendu français (%q)", w.Code, s)
		}
	}
}

// TestJournalVersionEcrite : tout événement produit par le moteur porte la version du format.
func TestJournalVersionEcrite(t *testing.T) {
	st, created, err := tournoi.New(tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 5},
	}}, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != tournoi.JournalVersion {
		t.Errorf("événement de création : version %d, attendu %d", created.Version, tournoi.JournalVersion)
	}
	for i, n := range []string{"a", "b"} {
		p := tournoi.Player{ID: tournoi.PlayerID(n), Name: n}
		ev := tournoi.Event{Version: tournoi.JournalVersion, Kind: tournoi.EvPlayerAdded, Time: time.Now(), Player: &p}
		if err := st.Apply(ev); err != nil {
			t.Fatalf("joueur %d : %v", i, err)
		}
	}
	acts := st.Propose()
	if len(acts) == 0 {
		t.Fatal("aucune proposition")
	}
	ev, err := st.EventFromAction(acts[0], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if ev.Version != tournoi.JournalVersion {
		t.Errorf("événement d'action : version %d, attendu %d", ev.Version, tournoi.JournalVersion)
	}
	res := tournoi.ResultEvent(ev.MatchID, ev.A, 5, 2, time.Now())
	if res.Version != tournoi.JournalVersion {
		t.Errorf("événement de résultat : version %d, attendu %d", res.Version, tournoi.JournalVersion)
	}
}

// TestJournalVersionZeroSeRejoue : un journal écrit avant les codes structurés (aucun champ
// `version`, aucun champ `round`) se rejoue et donne le même classement. C'est la compatibilité
// ascendante promise par l'issue N1.
func TestJournalVersionZeroSeRejoue(t *testing.T) {
	const v0 = `[
 {"seq":0,"kind":"created","time":"2026-09-07T10:00:00Z","config":{"name":"Ancien","phases":[{"kind":"swiss_lives","length":5,"lives":2,"mode":"rounds"}]},"seed":42},
 {"seq":1,"kind":"player_added","time":"2026-09-07T10:01:00Z","player":{"id":"a","name":"Alice"}},
 {"seq":2,"kind":"player_added","time":"2026-09-07T10:01:00Z","player":{"id":"b","name":"Bob"}},
 {"seq":3,"kind":"player_added","time":"2026-09-07T10:01:00Z","player":{"id":"c","name":"Chloé"}},
 {"seq":4,"kind":"player_added","time":"2026-09-07T10:01:00Z","player":{"id":"d","name":"Dan"}},
 {"seq":5,"kind":"match_started","time":"2026-09-07T10:10:00Z","match_id":"M1","label":"Ronde 1","a":"a","b":"b","length":5},
 {"seq":6,"kind":"match_started","time":"2026-09-07T10:10:00Z","match_id":"M2","label":"Ronde 1","a":"c","b":"d","length":5},
 {"seq":7,"kind":"result","time":"2026-09-07T10:50:00Z","match_id":"M1","winner":"a","score_a":5,"score_b":3},
 {"seq":8,"kind":"result","time":"2026-09-07T10:55:00Z","match_id":"M2","winner":"c","score_a":5,"score_b":1}
]`
	j, err := tournoi.ParseJournal([]byte(v0))
	if err != nil {
		t.Fatalf("lecture du journal v0 : %v", err)
	}
	for _, ev := range j {
		if ev.Version != 0 {
			t.Fatalf("la fixture doit être un journal v0, événement %d porte la version %d", ev.Seq, ev.Version)
		}
	}
	st, err := tournoi.Replay(j)
	if err != nil {
		t.Fatalf("rejeu : %v", err)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("rejeu d'un journal v0 : %d avertissement(s) : %v", len(st.Warnings), st.Warnings)
	}
	if len(st.Matches) != 2 {
		t.Fatalf("2 matchs attendus, %d", len(st.Matches))
	}
	// Le libellé d'un événement v0 était une chaîne : il est illisible comme Label et vaut le
	// libellé vide. Ce que le moteur en tirait — le numéro de ronde — vient désormais du champ
	// Round, absent d'un journal v0 : la phase reste donc à la ronde 0, sans que rien ne casse.
	r := st.Ranking()
	if len(r) != 4 {
		t.Fatalf("classement de 4 joueurs attendu, %d", len(r))
	}
	gagnants := map[tournoi.PlayerID]bool{"a": true, "c": true}
	for _, rk := range r[:2] {
		if !gagnants[rk.Player] {
			t.Errorf("les deux premiers doivent être les vainqueurs de la ronde 1, trouvé %s", rk.Player)
		}
	}
	// Et il se re-sérialise sans perdre les événements.
	b, err := j.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := tournoi.ParseJournal(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(j2) != len(j) {
		t.Errorf("aller-retour du journal v0 : %d événements au lieu de %d", len(j2), len(j))
	}
}

// TestRondeVientDuChampPasDuLibelle : le numéro de ronde est une donnée de l'événement. Avant
// N1, il était relu dans le texte « Ronde k » — un libellé traduit aurait cassé le comptage.
func TestRondeVientDuChampPasDuLibelle(t *testing.T) {
	st, created, err := tournoi.New(tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 5, Mode: "rounds"},
	}}, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := tournoi.Journal{created}
	for _, n := range []string{"a", "b", "c", "d"} {
		p := tournoi.Player{ID: tournoi.PlayerID(n), Name: n}
		ev := tournoi.Event{Version: tournoi.JournalVersion, Kind: tournoi.EvPlayerAdded, Time: time.Now(), Player: &p}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		journal = append(journal, ev)
	}
	acts := st.Propose()
	if len(acts) == 0 {
		t.Fatal("aucune proposition")
	}
	vu := false
	for _, a := range acts {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		if a.Round != 1 {
			t.Errorf("proposition de ronde : Round = %d, attendu 1", a.Round)
		}
		if a.Label.Kind != tournoi.LabelRound || a.Label.N != 1 {
			t.Errorf("libellé de ronde : %+v", a.Label)
		}
		ev, err := st.EventFromAction(a, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if ev.Round != 1 {
			t.Errorf("événement : Round = %d, attendu 1", ev.Round)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		vu = true
	}
	if !vu {
		t.Fatal("aucun match proposé")
	}
	if st.Phases[0].Round != 1 {
		t.Errorf("la phase doit être en ronde 1, elle est en ronde %d", st.Phases[0].Round)
	}
}

// TestNomsDeSectionSontDesIdentifiants : une section ne porte pas de français, puisque son nom
// voyage dans le journal et dans la base du logiciel hôte.
func TestNomsDeSectionSontDesIdentifiants(t *testing.T) {
	for name, cfg := range configs() {
		name, cfg := name, cfg
		t.Run(name, func(t *testing.T) {
			joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(11)))
			r := sim.Run(cfg, joueurs, sim.Options{Seed: 11})
			if r.State == nil {
				t.Fatal("pas d'état")
			}
			for _, ph := range r.State.Phases {
				for _, sec := range ph.Sections {
					if !estIdentifiant(sec.Name) {
						t.Errorf("section %q : un nom de section est un identifiant, pas un libellé", sec.Name)
					}
				}
			}
		})
	}
}

// TestToutEvenementPorteLaVersion : une simulation complète ne produit que des événements à la
// version courante. Un événement fabriqué sans version serait relu comme un journal ancien —
// d'où les constructeurs de events.go.
func TestToutEvenementPorteLaVersion(t *testing.T) {
	for name, cfg := range configs() {
		name, cfg := name, cfg
		t.Run(name, func(t *testing.T) {
			joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(5)))
			r := sim.Run(cfg, joueurs, sim.Options{Seed: 5, Hook: func(_ *tournoi.State, ev tournoi.Event) error {
				if ev.Version != tournoi.JournalVersion {
					t.Errorf("événement %s : version %d, attendu %d", ev.Kind, ev.Version, tournoi.JournalVersion)
				}
				return nil
			}})
			if r.Err != nil {
				t.Fatal(r.Err)
			}
		})
	}
}
