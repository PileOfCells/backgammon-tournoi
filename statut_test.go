package tournoi_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// Statut d'un joueur (statut.go) : « est-ce que je joue ? ».

func statutDe(st *tournoi.State, p tournoi.PlayerID) tournoi.PlayerStatus {
	for _, s := range st.Statuses() {
		if s.Player == p {
			return s
		}
	}
	return tournoi.PlayerStatus{}
}

func parStatut(st *tournoi.State) map[tournoi.StatusKind][]tournoi.PlayerID {
	out := map[tournoi.StatusKind][]tournoi.PlayerID{}
	for _, s := range st.Statuses() {
		out[s.Kind] = append(out[s.Kind], s.Player)
	}
	return out
}

func memes(a, b []tournoi.PlayerID) bool {
	x := append([]tournoi.PlayerID(nil), a...)
	y := append([]tournoi.PlayerID(nil), b...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	sort.Slice(y, func(i, j int) bool { return y[i] < y[j] })
	return fmt.Sprint(x) == fmt.Sprint(y)
}

// gardien vérifie, événement après événement, ce que le statut promet : un éliminé ne rejoue
// plus, un retiré est « retiré », les qualifiés annoncés sont les entrants du passage de phase,
// le vainqueur est le premier du classement final.
type gardien struct {
	t       *testing.T
	nom     string
	avant   []tournoi.PlayerStatus
	elimine map[tournoi.PlayerID]bool
}

func nouveauGardien(t *testing.T, nom string) *gardien {
	return &gardien{t: t, nom: nom, elimine: map[tournoi.PlayerID]bool{}}
}

func (g *gardien) apres(st *tournoi.State, ev tournoi.Event) {
	g.t.Helper()
	switch ev.Kind {
	case tournoi.EvMatchStarted:
		for _, p := range []tournoi.PlayerID{ev.A, ev.B} {
			if g.elimine[p] {
				g.t.Errorf("%s : %s annoncé éliminé, et il joue %s", g.nom, p, ev.MatchID)
			}
		}
	case tournoi.EvNextPhase:
		var annonces []tournoi.PlayerID
		for _, s := range g.avant {
			if s.Kind == tournoi.StatusQualified && s.Phase == st.Current {
				annonces = append(annonces, s.Player)
			}
		}
		if !memes(annonces, st.Phases[st.Current].Entrants) {
			g.t.Errorf("%s : qualifiés annoncés %v, entrants du passage %v", g.nom, annonces, st.Phases[st.Current].Entrants)
		}
	case tournoi.EvFinished:
		for _, r := range st.Final {
			if k := statutDe(st, r.Player).Kind; (r.Rank == 1) != (k == tournoi.StatusWinner) && k != tournoi.StatusWithdrawn {
				g.t.Errorf("%s : %s, rang %d, statut %s", g.nom, r.Player, r.Rank, k)
			}
		}
	case tournoi.EvResultCorrected, tournoi.EvMatchCancelled, tournoi.EvConfigChanged, tournoi.EvReopened:
		g.elimine = map[tournoi.PlayerID]bool{} // une correction peut rendre la vie
	}
	g.avant = st.Statuses()
	if len(g.avant) != len(st.Order) {
		g.t.Errorf("%s : %d statuts pour %d inscrits", g.nom, len(g.avant), len(st.Order))
	}
	for _, s := range g.avant {
		if (s.Kind == tournoi.StatusWithdrawn) != st.Withdrawn[s.Player] {
			g.t.Errorf("%s : %s retiré=%v, statut %s", g.nom, s.Player, st.Withdrawn[s.Player], s.Kind)
		}
		if s.Kind == tournoi.StatusEliminated {
			g.elimine[s.Player] = true
		}
		if s.Kind == tournoi.StatusBye || s.Kind == tournoi.StatusEliminated {
			for _, m := range st.Running() {
				if m.Has(s.Player) {
					g.t.Errorf("%s : %s %s et en train de jouer %s", g.nom, s.Player, s.Kind, m.ID)
				}
			}
		}
		if s.Kind == "" || s.String() == string(s.Kind) {
			g.t.Errorf("%s : statut sans rendu : %+v", g.nom, s)
		}
	}
}

// TestStatutsInvariants : tous les formats de la matrice, plus les entrées top:N et all qui
// rendent un parcours fini indécis ou qualifiant.
func TestStatutsInvariants(t *testing.T) {
	cfgs := configs()
	cfgs["suisse_top4_tableau"] = tournoi.Config{Name: "Suisse puis top 4", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 5, Target: 8},
		{Kind: tournoi.KindBracket, Length: 7, Entry: "top:4"},
	}}
	cfgs["tableau_puis_suisse_all"] = tournoi.Config{Name: "Tableau puis suisse ouvert", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5},
		{Kind: tournoi.KindSwissLives, Length: 5, Entry: "all"},
	}}
	seeds := 3
	if testing.Short() {
		seeds = 1
	}
	for name, cfg := range cfgs {
		for _, n := range []int{11, 16, 24} {
			for k := 0; k < seeds; k++ {
				nom := fmt.Sprintf("%s/%d/%d", name, n, k)
				g := nouveauGardien(t, nom)
				players := sim.Champ(n, 6, 2, 2, 10, rand.New(rand.NewSource(int64(k))))
				sim.Run(cfg, players, sim.Options{Seed: int64(k), Hook: func(st *tournoi.State, ev tournoi.Event) error {
					g.apres(st, ev)
					aucunTexteAffichable(t, st.Statuses(), nom+" statuts")
					return nil
				}})
			}
		}
	}
}

// ---- Scénario T3 (N26) : un repêché est qualifié, un retiré non ----

func TestStatutsN26_T3(t *testing.T) {
	b := bancT3(t)
	// Poules finies : deux qualifiés par poule pour le tableau (phase 1), les autres éliminés.
	k := parStatut(b.st)
	if len(k[tournoi.StatusQualified]) != 8 || len(k[tournoi.StatusEliminated]) != 8 {
		t.Fatalf("fin des poules : %v", k)
	}
	if s := statutDe(b.st, "benoît-carrel"); s.Kind != tournoi.StatusQualified || s.Phase != 1 {
		t.Errorf("Benoît (3-0) : %+v", s)
	}
	if s := statutDe(b.st, "olivier-caron"); s.Kind != tournoi.StatusEliminated || s.Phase != 0 {
		t.Errorf("Olivier, battu au barrage : %+v", s)
	}

	b.retire("benoît-carrel")
	if s := statutDe(b.st, "benoît-carrel"); s.Kind != tournoi.StatusWithdrawn {
		t.Errorf("Benoît retiré : %+v", s)
	}
	// Repêchage proposé, pas encore tranché : Pauline n'est ni qualifiée ni éliminée.
	if s := statutDe(b.st, "pauline-brunet"); s.Kind != tournoi.StatusUndecided || s.Phase != 1 {
		t.Errorf("Pauline, candidate au repêchage : %+v", s)
	}
	if s := statutDe(b.st, "laure-gauthier"); s.Kind != tournoi.StatusEliminated {
		t.Errorf("Laure, 4e de la poule A : %+v", s)
	}

	b.confirme(actionsDe(b.st.Propose(), tournoi.ActRepechage)[0])
	if s := statutDe(b.st, "pauline-brunet"); s.Kind != tournoi.StatusQualified || s.Phase != 1 {
		t.Errorf("Pauline repêchée : %+v", s)
	}
	g := nouveauGardien(t, "T3")
	g.avant = b.st.Statuses()
	ev := b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	g.apres(b.st, ev)
	// Passage fait, tableau pas tiré : les entrants restent « qualifiés ».
	if s := statutDe(b.st, "pauline-brunet"); s.Kind != tournoi.StatusQualified || s.Phase != 1 {
		t.Errorf("Pauline, avant le tirage : %+v", s)
	}
	a := b.st.Propose()[0]
	a.Draw = &tournoi.Draw{Slots: t3Tableau}
	b.confirme(a)
	k = parStatut(b.st)
	if len(k[tournoi.StatusPlaying]) != 8 || len(k[tournoi.StatusBye]) != 0 {
		t.Errorf("tableau de 8 sans exemption : %v", k)
	}
	if s := statutDe(b.st, "pauline-brunet"); s.Section != "main" || s.Round != 1 {
		t.Errorf("Pauline joue le premier tour du principal : %+v", s)
	}
}

// La directrice passe outre : la place devient une exemption, que le statut annonce.
func TestStatutsN26_T3VarianteMoteur(t *testing.T) {
	b := bancT3(t)
	b.retire("benoît-carrel")
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0])
	// Passage fait sans repêchage : Pauline reste candidate jusqu'au tirage.
	if s := statutDe(b.st, "pauline-brunet"); s.Kind != tournoi.StatusUndecided || s.Phase != 1 {
		t.Errorf("Pauline, avant le tirage : %+v", s)
	}
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActDraw)[0])
	if s := statutDe(b.st, "pauline-brunet"); s.Kind != tournoi.StatusEliminated {
		t.Errorf("tableau tiré, repêchage clos : Pauline %+v", s)
	}
	byes := parStatut(b.st)[tournoi.StatusBye]
	if len(byes) != 1 {
		t.Fatalf("une exemption attendue : %v", parStatut(b.st))
	}
	if s := statutDe(b.st, byes[0]); s.Round != 2 || s.Section != "main" || s.Label.Kind != tournoi.LabelSemiFinal {
		t.Errorf("l'exempté entre en demi-finale (tour 2) : %+v", s)
	}
}

// ---- Feuilles de la simulation blunderDB 2026-10 (T1, T2) ----

type feuille struct {
	Players []tournoi.Player `json:"players"`
	Events  []struct {
		Type   string           `json:"type"`
		Phase  int              `json:"phase"`
		A      tournoi.PlayerID `json:"a"`
		B      tournoi.PlayerID `json:"b"`
		Winner tournoi.PlayerID `json:"winner"`
		Slots  []*string        `json:"slots"`
		Player json.RawMessage  `json:"player"`
	} `json:"events"`
}

// rejoueFeuille mène l'épreuve de la feuille (suisse 2 vies continu jusqu'à 16 vies, puis
// tableau à vies) en vérifiant les statuts à chaque événement ; voir(i) est appelé avant
// l'événement i de la feuille.
func rejoueFeuille(t *testing.T, fichier string, voir func(i int, b *banc)) *banc {
	t.Helper()
	brut, err := os.ReadFile(fichier)
	if err != nil {
		t.Fatal(err)
	}
	var f feuille
	if err := json.Unmarshal(brut, &f); err != nil {
		t.Fatal(err)
	}
	cfg := tournoi.Config{Name: fichier, Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Lives: 2, Target: 16, Length: 5},
		{Kind: tournoi.KindLivesBracket, Length: 7},
	}}
	now := time.Date(2026, 10, 4, 14, 0, 0, 0, time.UTC)
	st, ev, err := tournoi.New(cfg, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	b := &banc{t: t, st: st, journal: tournoi.Journal{ev}, now: now}
	g := nouveauGardien(t, fichier)
	applique := func(ev tournoi.Event) tournoi.Event {
		b.applique(ev)
		g.apres(b.st, b.journal[len(b.journal)-1])
		return b.journal[len(b.journal)-1]
	}
	for _, p := range f.Players {
		applique(tournoi.PlayerAddedEvent(p, b.now))
	}
	for i, e := range f.Events {
		voir(i, b)
		switch e.Type {
		case "late":
			var p tournoi.Player
			if err := json.Unmarshal(e.Player, &p); err != nil {
				t.Fatal(err)
			}
			applique(tournoi.PlayerAddedEvent(p, b.now))
		case "withdraw":
			var p tournoi.PlayerID
			if err := json.Unmarshal(e.Player, &p); err != nil {
				t.Fatal(err)
			}
			applique(tournoi.PlayerWithdrawnEvent(p, b.now))
		case "next_phase":
			ev, err := b.st.EventFromAction(actionsDe(b.st.Propose(), tournoi.ActNextPhase)[0], b.now)
			if err != nil {
				t.Fatal(err)
			}
			applique(ev)
		case "draw":
			a := actionsDe(b.st.Propose(), tournoi.ActDraw)[0]
			d := *a.Draw
			d.Slots = nil
			for _, s := range e.Slots {
				if s == nil {
					d.Slots = append(d.Slots, tournoi.BYE)
				} else {
					d.Slots = append(d.Slots, tournoi.PlayerID(*s))
				}
			}
			a.Draw = &d
			ev, err := b.st.EventFromAction(a, b.now)
			if err != nil {
				t.Fatal(err)
			}
			applique(ev)
		case "match":
			a := tournoi.Action{Kind: tournoi.ActStartMatch, Phase: e.Phase, A: e.A, B: e.B, Length: 5}
			if e.Phase > 0 {
				a = tournoi.Action{}
				for _, p := range actionsDe(b.st.Propose(), tournoi.ActStartMatch) {
					if (p.A == e.A && p.B == e.B) || (p.A == e.B && p.B == e.A) {
						a = p
					}
				}
				if a.Kind == "" {
					t.Fatalf("événement %d : %s contre %s pas proposé : %v", i, e.A, e.B, b.st.Propose())
				}
			}
			ev, err := b.st.EventFromAction(a, b.now)
			if err != nil {
				t.Fatalf("événement %d : %v", i, err)
			}
			ev = applique(ev)
			applique(tournoi.ResultEvent(ev.MatchID, e.Winner, 5, 0, b.now))
		default:
			t.Fatalf("événement %d : type %q", i, e.Type)
		}
	}
	voir(len(f.Events), b)
	ev, err = b.st.EventFromAction(actionsDe(b.st.Propose(), tournoi.ActFinish)[0], b.now)
	if err != nil {
		t.Fatal(err)
	}
	applique(ev)
	return b
}

// T1 : 16 joueurs, suisse 2 vies jusqu'à 16 vies, tableau de 16 places à 4 exemptions.
func TestStatutsSimulationT1(t *testing.T) {
	const (
		suisseFini = 16 // événements de la feuille : 16 matchs de suisse, passage, tirage
		tire       = 18
	)
	b := rejoueFeuille(t, "testdata/simulation-2026-10/T1.json", func(i int, b *banc) {
		k := parStatut(b.st)
		switch i {
		case 0:
			if len(k[tournoi.StatusPlaying]) != 16 {
				t.Errorf("début du suisse : %v", k)
			}
		case suisseFini:
			if len(k[tournoi.StatusQualified]) != 12 || len(k[tournoi.StatusEliminated]) != 4 {
				t.Errorf("suisse fini : 12 qualifiés et 4 éliminés attendus : %v", k)
			}
		case tire:
			// Les quatre invaincus, tirés face à une place vide, entrent au deuxième tour.
			if !memes(k[tournoi.StatusBye], []tournoi.PlayerID{"lazare-quintrec", "fulbert-arsonval", "katell-brasseur", "nestor-bellefond"}) {
				t.Errorf("exemptés : %v", k[tournoi.StatusBye])
			}
			for _, p := range k[tournoi.StatusBye] {
				if s := statutDe(b.st, p); s.Round != 2 || s.Section != "main" || s.Phase != 1 {
					t.Errorf("%s : %+v", p, s)
				}
			}
			if len(k[tournoi.StatusPlaying]) != 8 || len(k[tournoi.StatusEliminated]) != 4 {
				t.Errorf("tableau tiré : %v", k)
			}
		case tire + 1:
			// Désiré a perdu son premier tour : plus de match, éliminé du tournoi.
			if s := statutDe(b.st, "désiré-oudinot"); s.Kind != tournoi.StatusEliminated || s.Phase != 1 {
				t.Errorf("Désiré : %+v", s)
			}
			if s := statutDe(b.st, "lazare-quintrec"); s.Kind != tournoi.StatusBye {
				t.Errorf("Lazare attend Prosper : %+v", s)
			}
		}
	})
	k := parStatut(b.st)
	if !memes(k[tournoi.StatusWinner], []tournoi.PlayerID{"gaëlle-picquenot"}) || len(k[tournoi.StatusEliminated]) != 15 {
		t.Errorf("fin : %v", k)
	}
}

// T2 : 32 joueurs, un retardataire, un forfait suivi d'un retrait.
func TestStatutsSimulationT2(t *testing.T) {
	b := rejoueFeuille(t, "testdata/simulation-2026-10/T2.json", func(i int, b *banc) {
		switch i {
		case 4: // juste après l'arrivée de Gaël : il entre au suisse avec ses deux vies
			if s := statutDe(b.st, "gaël-tessier"); s.Kind != tournoi.StatusPlaying || s.Phase != 0 {
				t.Errorf("retardataire : %+v", s)
			}
		case 35:
			if s := statutDe(b.st, "valérie-mace"); s.Kind != tournoi.StatusWithdrawn {
				t.Errorf("Valérie retirée : %+v", s)
			}
		case 52: // passage fait, tableau pas tiré
			k := parStatut(b.st)
			if len(k[tournoi.StatusQualified]) != len(b.st.Phases[1].Entrants) {
				t.Errorf("avant le tirage : %v", k)
			}
		}
	})
	k := parStatut(b.st)
	if !memes(k[tournoi.StatusWinner], []tournoi.PlayerID{"ugo-benard"}) || !memes(k[tournoi.StatusWithdrawn], []tournoi.PlayerID{"valérie-mace"}) {
		t.Errorf("fin : %v", k)
	}
}

// Un perdant du principal reversé en consolante porte la note section_exit du principal ; son
// statut, lui, dit qu'il joue encore — en consolante.
func TestStatutConsolanteEnCours(t *testing.T) {
	cfg := tournoi.Config{Name: "Tableau et consolante", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5, Consolation: true},
	}}
	var ids []tournoi.PlayerID
	for i := 0; i < 8; i++ {
		ids = append(ids, tournoi.PlayerID(fmt.Sprintf("p%d", i)))
	}
	b := nouveauBanc(t, cfg, ids)
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActDraw)[0])
	a := actionsDe(b.st.Propose(), tournoi.ActStartMatch)[0]
	ev := b.confirme(a)
	b.applique(tournoi.ResultEvent(ev.MatchID, a.A, 5, 0, b.now))
	if r := rangDe(b.st, a.B); r.Note.Kind != tournoi.NoteSectionExit || r.Note.Section != "main" {
		t.Fatalf("note du perdant : %+v", r.Note)
	}
	s := statutDe(b.st, a.B)
	if s.Kind == tournoi.StatusEliminated || s.Section != "conso" {
		t.Errorf("perdant reversé en consolante : %+v", s)
	}
}

// Le principal est fini, la consolante pas : son vainqueur n'a plus de match et n'est pas
// éliminé — c'est le vainqueur du tournoi.
func TestStatutVainqueurAvantFinConsolante(t *testing.T) {
	cfg := tournoi.Config{Name: "Tableau et consolante", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5, Consolation: true},
	}}
	var ids []tournoi.PlayerID
	for i := 0; i < 8; i++ {
		ids = append(ids, tournoi.PlayerID(fmt.Sprintf("p%d", i)))
	}
	b := nouveauBanc(t, cfg, ids)
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActDraw)[0])
	var champion tournoi.PlayerID
	for i := 0; i < 20; i++ {
		var a *tournoi.Action
		for _, p := range actionsDe(b.st.Propose(), tournoi.ActStartMatch) {
			if p.Section == "main" {
				a = &p
				break
			}
		}
		if a == nil {
			break
		}
		ev := b.confirme(*a)
		b.applique(tournoi.ResultEvent(ev.MatchID, a.A, 5, 0, b.now))
		champion = a.A
	}
	if len(actionsDe(b.st.Propose(), tournoi.ActStartMatch)) == 0 {
		t.Fatal("la consolante devait rester à jouer")
	}
	if s := statutDe(b.st, champion); s.Kind != tournoi.StatusWinner {
		t.Errorf("vainqueur du principal, consolante en cours : %+v", s)
	}
	k := parStatut(b.st)
	if len(k[tournoi.StatusPlaying]) == 0 || len(k[tournoi.StatusWinner]) != 1 {
		t.Errorf("statuts : %v", k)
	}
}

// Suisse par rondes, effectif impair : le bye de la ronde est annoncé, avec la ronde du retour.
func TestStatutByeSuisseParRondes(t *testing.T) {
	cfg := tournoi.Config{Name: "Rondes", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 5, Mode: "rounds"},
	}}
	b := nouveauBanc(t, cfg, []tournoi.PlayerID{"a", "b", "c", "d", "e"})
	var exempt tournoi.PlayerID
	for _, a := range b.st.Propose() {
		ev := b.confirme(a)
		if a.Kind == tournoi.ActBye {
			exempt = ev.ID
		}
	}
	if exempt == "" {
		t.Fatal("un bye attendu à la ronde 1")
	}
	s := statutDe(b.st, exempt)
	if s.Kind != tournoi.StatusBye || s.Round != 2 || s.Label.Kind != tournoi.LabelRound || s.Label.N != 2 {
		t.Errorf("exempté de la ronde 1 : %+v", s)
	}
	if k := parStatut(b.st); len(k[tournoi.StatusPlaying]) != 4 {
		t.Errorf("statuts : %v", k)
	}
	for _, m := range b.st.Running() {
		b.applique(tournoi.ResultEvent(m.ID, m.A, 5, 0, b.now))
	}
	if s := statutDe(b.st, exempt); s.Kind != tournoi.StatusBye {
		t.Errorf("ronde 1 finie, ronde 2 pas commencée : %+v", s)
	}
	b.confirme(actionsDe(b.st.Propose(), tournoi.ActStartMatch)[0])
	if s := statutDe(b.st, exempt); s.Kind != tournoi.StatusPlaying {
		t.Errorf("ronde 2 ouverte : %+v", s)
	}
}
