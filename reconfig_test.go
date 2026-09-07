package tournoi_test

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// suisseCommence : un suisse de n joueurs dont un match est lancé — le cas où la phase est
// « commencée » au sens de reconfig.go.
func suisseCommence(t *testing.T, cfg tournoi.Config, n int) *tournoi.State {
	t.Helper()
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(cfg, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(n, 6, 2, 2, 10, rand.New(rand.NewSource(1))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range st.Propose() {
		if a.Kind != tournoi.ActStartMatch {
			continue
		}
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
		break
	}
	if !st.Phases[0].Started {
		t.Fatal("la phase devrait être commencée")
	}
	return st
}

func suisseBascule() tournoi.Config {
	return tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Target: 16},
		{Kind: tournoi.KindLivesBracket, Length: 9},
	}}
}

// TestFormatDUnePhaseCommenceeNeChangePlus : le refus doit dire LAQUELLE et POURQUOI, sans quoi
// le TD ne peut que renoncer.
func TestFormatDUnePhaseCommenceeNeChangePlus(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 16)
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Kind = tournoi.KindBracket
	next.Phases[0].Target = 0
	err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last))
	if err == nil {
		t.Fatal("changer le format d'une phase commencée doit être refusé")
	}
	for _, attendu := range []string{"phase 0", tournoi.KindSwissLives, tournoi.KindBracket, "lancés"} {
		if !strings.Contains(err.Error(), attendu) {
			t.Errorf("le refus ne dit pas %q : %v", attendu, err)
		}
	}
	if st.Config.Phases[0].Kind != tournoi.KindSwissLives {
		t.Error("la configuration a changé malgré le refus")
	}
}

// TestPhaseOuverteNeSeRetirePas : une phase que le tournoi a déjà ouverte ne disparaît pas de la
// configuration ; les joueurs y sont.
func TestPhaseOuverteNeSeRetirePas(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 16)
	next := st.Config
	next.Phases = []tournoi.PhaseConfig{}
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err == nil {
		t.Fatal("une configuration sans phase doit être refusée")
	}
}

// TestBasculeAbaisseeEnCours : le cas de 22 h — le directeur baisse la bascule pour finir plus
// tôt. La phase courante n'est pas perturbée.
func TestBasculeAbaisseeEnCours(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 32)
	enCours := len(st.Running())
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Target = 8
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err != nil {
		t.Fatalf("baisser la bascule doit être accepté : %v", err)
	}
	if st.Config.Phases[0].Target != 8 || st.Phases[0].Cfg.Target != 8 {
		t.Errorf("bascule non répercutée : config %d, phase %d", st.Config.Phases[0].Target, st.Phases[0].Cfg.Target)
	}
	if len(st.Running()) != enCours {
		t.Errorf("les matchs en cours ont bougé : %d au lieu de %d", len(st.Running()), enCours)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements après reconfiguration : %v", st.Warnings)
	}
}

// TestPhaseAjouteeApresLaCourante : ajouter un tableau final qu'on n'avait pas prévu.
func TestPhaseAjouteeApresLaCourante(t *testing.T) {
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Target: 16}}}
	st := suisseCommence(t, cfg, 32)
	avant := len(st.Phases[0].Entrants)
	next := st.Config
	next.Phases = append(append([]tournoi.PhaseConfig(nil), next.Phases...),
		tournoi.PhaseConfig{Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11})
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err != nil {
		t.Fatalf("ajouter une phase après la courante doit être accepté : %v", err)
	}
	if len(st.Config.Phases) != 2 {
		t.Fatalf("%d phases dans la configuration, attendu 2", len(st.Config.Phases))
	}
	if len(st.Phases) != 1 || st.Phases[0].Cfg.Kind != tournoi.KindSwissLives || len(st.Phases[0].Entrants) != avant {
		t.Errorf("la phase courante a été perturbée : %d phases ouvertes, %d entrants", len(st.Phases), len(st.Phases[0].Entrants))
	}
	// et le tournoi se termine bien par le tableau ajouté
	fin := joueJusquAuBout(t, st)
	if len(fin.Phases) != 2 {
		t.Fatalf("%d phases jouées, attendu 2 — la phase ajoutée n'a pas été atteinte", len(fin.Phases))
	}
}

// joueJusquAuBout : joue toutes les propositions, le vainqueur étant toujours le joueur A.
func joueJusquAuBout(t *testing.T, st *tournoi.State) *tournoi.State {
	t.Helper()
	now := st.Last
	for étape := 0; !st.Finished && étape < 20000; étape++ {
		acts := st.Propose()
		avancé := false
		for _, a := range acts {
			if a.Kind == tournoi.ActWait {
				continue
			}
			now = now.Add(time.Minute)
			ev, err := st.EventFromAction(a, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatalf("%s : %v", a, err)
			}
			avancé = true
			if a.Kind == tournoi.ActDraw || a.Kind == tournoi.ActNextPhase || a.Kind == tournoi.ActFinish {
				break
			}
		}
		for _, m := range st.Running() {
			now = now.Add(time.Minute)
			if err := st.Apply(tournoi.ResultEvent(m.ID, m.A, m.Length, 0, now)); err != nil {
				t.Fatal(err)
			}
			avancé = true
		}
		if !avancé {
			t.Fatalf("blocage : %v", acts)
		}
	}
	if !st.Finished {
		t.Fatal("le tournoi ne se termine pas")
	}
	return st
}

// TestLengthChangedResteLu : les journaux existants n'ont pas à être réécrits.
func TestLengthChangedResteLu(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 32)
	if err := st.Apply(tournoi.LengthChangedEvent(0, 5, st.Last)); err != nil {
		t.Fatal(err)
	}
	if st.Phases[0].Length != 5 {
		t.Fatalf("length_changed sans effet : %d", st.Phases[0].Length)
	}
	// Une reconfiguration qui ne touche pas à la longueur ne l'efface pas : le TD vient de la
	// saisir à la main, et il dirige.
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Target = 8
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err != nil {
		t.Fatal(err)
	}
	if st.Phases[0].Length != 5 {
		t.Errorf("la reconfiguration a effacé le length_changed : %d au lieu de 5", st.Phases[0].Length)
	}
	// En revanche, une longueur explicitement changée dans la configuration l'emporte.
	next.Phases[0].Length = 11
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err != nil {
		t.Fatal(err)
	}
	if st.Phases[0].Length != 11 {
		t.Errorf("la longueur de la configuration n'est pas prise : %d au lieu de 11", st.Phases[0].Length)
	}
}

// TestReouvertureEtCorrection : un résultat faux découvert après la clôture. Le classement final
// de la clôture suivante tient compte de la correction.
func TestReouvertureEtCorrection(t *testing.T) {
	joueurs := sim.Champ(8, 6, 2, 2, 10, rand.New(rand.NewSource(9)))
	r := sim.Run(configs()["elim_simple"], joueurs, sim.Options{Seed: 9})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	st, err := tournoi.Replay(r.Journal)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Finished {
		t.Fatal("le tournoi simulé devrait être clos")
	}
	finale := st.Matches[st.MatchOrder[len(st.MatchOrder)-1]]
	vainqueur, perdant := finale.Winner, finale.Loser()
	if st.Final[0].Player != vainqueur {
		t.Fatalf("le vainqueur du dernier match (%s) devrait être premier, or c'est %s", vainqueur, st.Final[0].Player)
	}
	// Corriger sans rouvrir n'a pas de sens : le classement final est figé.
	now := st.Last.Add(time.Hour)
	if err := st.Apply(tournoi.ReopenedEvent(now)); err != nil {
		t.Fatal(err)
	}
	if st.Finished || st.Final != nil {
		t.Fatal("la réouverture doit effacer la clôture et le classement final")
	}
	if err := st.Apply(tournoi.CorrectionEvent(finale.ID, perdant, 0, finale.Length, now)); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(tournoi.Event{Version: tournoi.JournalVersion, Kind: tournoi.EvFinished, Time: now}); err != nil {
		t.Fatal(err)
	}
	if st.Final[0].Player != perdant {
		t.Errorf("après correction, le premier devrait être %s, c'est %s", perdant, st.Final[0].Player)
	}
	// et le journal complet se rejoue à l'identique
	j := append(append(tournoi.Journal(nil), r.Journal...),
		tournoi.ReopenedEvent(now),
		tournoi.CorrectionEvent(finale.ID, perdant, 0, finale.Length, now),
		tournoi.Event{Version: tournoi.JournalVersion, Kind: tournoi.EvFinished, Time: now})
	st2, err := tournoi.Replay(j)
	if err != nil {
		t.Fatal(err)
	}
	if st2.Final[0].Player != perdant {
		t.Errorf("rejeu : premier %s, attendu %s", st2.Final[0].Player, perdant)
	}
}

// TestReouvertureDUnTournoiNonClos : le dire vaut mieux que le faire en silence.
func TestReouvertureDUnTournoiNonClos(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 32)
	if err := st.Apply(tournoi.ReopenedEvent(st.Last)); err == nil {
		t.Error("rouvrir un tournoi qui n'est pas clos doit être refusé")
	}
}

// TestLaConfigurationDuJournalNestPasModifiee : Validate remplit les défauts en place ; sans
// copie, elle écrirait dans l'événement que l'hôte a stocké. On ne modifie jamais le journal.
func TestLaConfigurationDuJournalNestPasModifiee(t *testing.T) {
	st := suisseCommence(t, suisseBascule(), 32)
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Target = 8
	next.Phases[1].Name = ""
	ev := tournoi.ConfigChangedEvent(next, st.Last)
	if err := st.Apply(ev); err != nil {
		t.Fatal(err)
	}
	st.Config.Phases[0].Length = 99
	if ev.Config.Phases[0].Length == 99 {
		t.Error("l'état et l'événement du journal partagent le même tableau de phases")
	}
	if ev.Config.Phases[0].Target != 8 {
		t.Error("l'événement a été altéré")
	}
}
