package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// joueLeTournoi : lance et termine tous les matchs proposés jusqu'à la clôture (le premier
// joueur nommé gagne toujours). Sert à vérifier qu'un tableau reste jouable jusqu'au bout.
func joueLeTournoi(t *testing.T, st *tournoi.State) {
	t.Helper()
	for étapes := 0; !st.Finished && étapes < 500; étapes++ {
		avancé := false
		for _, a := range st.Propose() {
			if a.Kind == tournoi.ActWait {
				continue
			}
			ev, err := st.EventFromAction(a, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Apply(ev); err != nil {
				t.Fatal(err)
			}
			avancé = true
			if a.Kind == tournoi.ActStartMatch {
				if err := st.Apply(tournoi.ResultEvent(ev.MatchID, a.A, a.Length, 0, time.Now())); err != nil {
					t.Fatal(err)
				}
			}
			if a.Kind != tournoi.ActStartMatch {
				break // l'état a changé : re-proposer
			}
		}
		if !avancé {
			break
		}
	}
	if !st.Finished {
		t.Fatal("le tournoi ne se termine pas")
	}
}

// tableauDeSeizeAQuatorze : un tableau de 16 places pour 14 joueurs — donc deux exemptions,
// exactement la situation du retardataire.
func tableauDeSeizeAQuatorze(t *testing.T) *tournoi.State {
	t.Helper()
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5}}}, 14)
	tire(t, st)
	return st
}

// TestRetardatairePrendUnePlaceDExemption : il joue le tour 1 à la place laissée vide, le
// tableau reste cohérent, et personne n'est retiré.
func TestRetardatairePrendUnePlaceDExemption(t *testing.T) {
	st := tableauDeSeizeAQuatorze(t)
	slots := st.FreeSlots()
	if len(slots) != 2 {
		t.Fatalf("14 joueurs dans un tableau de 16 : %d places libres, attendu 2", len(slots))
	}
	// l'exempt de cette place avançait tout seul : après l'arrivée, il a un adversaire
	sec := st.Phases[0].Sections[0]
	var avant tournoi.PlayerID
	for i := range sec.Matches {
		if sec.Matches[i].Key == slots[0].Key {
			avant = sec.Matches[i].Winner
		}
	}
	if avant == "" {
		t.Fatal("la place d'exemption devait faire avancer son occupant")
	}

	tard := tournoi.Player{ID: "tard", Name: "Retardataire"}
	if err := st.Apply(tournoi.PlayerAddedAtSlotEvent(tard, slots[0], time.Now())); err != nil {
		t.Fatalf("le retardataire doit pouvoir prendre une place libre : %v", err)
	}
	var m *tournoi.GMatch
	for i := range sec.Matches {
		if sec.Matches[i].Key == slots[0].Key {
			m = &sec.Matches[i]
		}
	}
	if m == nil {
		t.Fatal("match de la place introuvable")
	}
	if m.Done || m.Walkover {
		t.Error("la place n'est plus une exemption : le match doit se jouer")
	}
	if m.Players[0] != "tard" && m.Players[1] != "tard" {
		t.Errorf("le retardataire n'occupe pas la place : %v", m.Players)
	}
	if m.Players[0] != avant && m.Players[1] != avant {
		t.Errorf("l'ancien exempt %s doit affronter le retardataire : %v", avant, m.Players)
	}
	if len(st.Warnings) != 0 {
		t.Errorf("aucun avertissement attendu : %v", st.Warnings)
	}
	if len(st.Infos) != 0 {
		t.Errorf("le retardataire a une place : plus rien à signaler, %v", st.Infos)
	}
	if len(st.FreeSlots()) != 1 {
		t.Errorf("il reste une seule place libre, %d trouvées", len(st.FreeSlots()))
	}
	joueLeTournoi(t, st)
	if n := len(st.Final); n != 15 {
		t.Errorf("classement de %d joueurs, attendu 15", n)
	}
}

// TestPlaceDUnTourCommenceRefusee : dès qu'un match du tour a été lancé, la place n'est plus
// disponible — les autres joueurs ont sous les yeux un tableau qui ne changera plus.
func TestPlaceDUnTourCommenceRefusee(t *testing.T) {
	st := tableauDeSeizeAQuatorze(t)
	slots := st.FreeSlots()
	lanceUnMatch(t, st) // un match du tour 1 démarre
	if got := st.FreeSlots(); len(got) != 0 {
		t.Errorf("le tour a commencé : plus aucune place ne doit être offerte, %d le sont", len(got))
	}
	err := st.Apply(tournoi.PlayerAddedAtSlotEvent(tournoi.Player{ID: "tard", Name: "Tard"}, slots[0], time.Now()))
	if err == nil {
		t.Error("une place d'un tour commencé doit être refusée")
	}
}

// TestPlaceInconnueRefusee : le moteur ne devine pas ce que le TD a voulu dire.
func TestPlaceInconnueRefusee(t *testing.T) {
	st := tableauDeSeizeAQuatorze(t)
	err := st.Apply(tournoi.PlayerAddedAtSlotEvent(tournoi.Player{ID: "tard"},
		tournoi.Slot{Phase: 0, Section: "main", Key: "main.9.9"}, time.Now()))
	if err == nil {
		t.Error("une place inconnue doit être refusée")
	}
}

// TestEntersAtNommeLaPhaseDEntree : sans place libre, le joueur est inscrit et le moteur dit où
// il entrera — au lieu de le laisser inscrit nulle part, sans que rien ne le signale.
func TestEntersAtNommeLaPhaseDEntree(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5},
		{Kind: tournoi.KindBracket, Name: "Repêchage", Length: 5, Entry: "all"}}}, 4)
	tire(t, st)
	if len(st.FreeSlots()) != 0 {
		t.Fatalf("un tableau de 4 pour 4 joueurs n'a aucune place libre, %d trouvées", len(st.FreeSlots()))
	}
	if err := st.Apply(tournoi.PlayerAddedEvent(tournoi.Player{ID: "tard", Name: "Tard"}, time.Now())); err != nil {
		t.Fatal(err)
	}
	if len(st.Infos) != 1 {
		t.Fatalf("une information attendue, %d : %v", len(st.Infos), st.Infos)
	}
	i := st.Infos[0]
	if i.Code != tournoi.InfoEntersAt || i.Player != "tard" || i.Phase != 1 {
		t.Errorf("information inattendue : %+v", i)
	}
	if i.Label.Kind != tournoi.LabelPhase || i.Label.Text != "Repêchage" {
		t.Errorf("l'information doit nommer la phase d'entrée : %+v", i.Label)
	}
}

// TestEntersAtNommeLaSectionQuandUnePlaceAttend : une place libre est une entrée plus proche
// qu'une phase à venir ; c'est elle qu'on signale.
func TestEntersAtNommeLaSectionQuandUnePlaceAttend(t *testing.T) {
	st := tableauDeSeizeAQuatorze(t)
	if err := st.Apply(tournoi.PlayerAddedEvent(tournoi.Player{ID: "tard", Name: "Tard"}, time.Now())); err != nil {
		t.Fatal(err)
	}
	if len(st.Infos) != 1 || st.Infos[0].Code != tournoi.InfoEntersAt || st.Infos[0].Section != "main" {
		t.Fatalf("le moteur doit signaler la place libre du principal : %v", st.Infos)
	}
}

// TestAucuneEntreePossibleSeDit : mieux vaut le dire que laisser un inscrit disparaître de
// l'affichage.
func TestAucuneEntreePossibleSeDit(t *testing.T) {
	st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 5}}}, 4)
	tire(t, st)
	if err := st.Apply(tournoi.PlayerAddedEvent(tournoi.Player{ID: "tard", Name: "Tard"}, time.Now())); err != nil {
		t.Fatal(err)
	}
	if len(st.Infos) != 1 || st.Infos[0].Code != tournoi.InfoNoEntry {
		t.Fatalf("aucune entrée possible : attendu no_entry, %v", st.Infos)
	}
}

// TestRetardataireSeRejoue : le journal contient tout ce qu'il faut ; le rejeu donne le même
// tableau, le même classement, et toujours aucun avertissement.
func TestRetardataireSeRejoue(t *testing.T) {
	cfg := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 5}}}
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
	for i := 0; i < 14; i++ {
		id := tournoi.PlayerID(string(rune('a' + i)))
		add(tournoi.PlayerAddedEvent(tournoi.Player{ID: id, Name: string(id)}, time.Now()))
	}
	for _, a := range st.Propose() {
		if a.Kind == tournoi.ActDraw {
			ev, err := st.EventFromAction(a, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			add(ev)
			break
		}
	}
	add(tournoi.PlayerAddedAtSlotEvent(tournoi.Player{ID: "tard", Name: "Tard"}, st.FreeSlots()[0], time.Now()))

	st2, err := tournoi.Replay(j)
	if err != nil {
		t.Fatalf("rejeu : %v", err)
	}
	if len(st2.Warnings) != 0 {
		t.Errorf("rejeu : avertissements %v", st2.Warnings)
	}
	if len(st2.Phases[0].Entrants) != len(st.Phases[0].Entrants) {
		t.Errorf("rejeu : %d entrants, attendu %d", len(st2.Phases[0].Entrants), len(st.Phases[0].Entrants))
	}
	if len(st2.FreeSlots()) != len(st.FreeSlots()) {
		t.Errorf("rejeu : %d places libres, attendu %d", len(st2.FreeSlots()), len(st.FreeSlots()))
	}
}
