package tournoi_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// ---- Bascule Σvies = 2^k en mode rondes ----

// TestBasculeEnModeRondesTombeJuste : la bascule vers un tableau demande que la somme des vies
// tombe JUSTE sur une puissance de deux. Chaque match la fait décroître d'une unité, mais une
// ronde en lance beaucoup d'un coup. Avant le budget de proposeSwissRound, la somme atterrissait
// entre 12 et 15 pour une cible de 16 — jamais dessus — et le tableau se remplissait de byes qui
// n'avaient pas lieu d'être.
func TestBasculeEnModeRondesTombeJuste(t *testing.T) {
	const cible = 16
	cfg := tournoi.Config{Name: "Bascule", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds", Target: cible},
		{Kind: tournoi.KindLivesBracket, Length: 9}}}
	for _, P := range []int{20, 24, 32, 50, 64} {
		for k := 0; k < 4; k++ {
			joueurs := sim.Champ(P, 6, 2, 2, 10, rand.New(rand.NewSource(int64(P*1000+k))))
			r := sim.Run(cfg, joueurs, sim.Options{Seed: int64(k)})
			if r.Err != nil {
				t.Fatalf("P=%d k=%d : %v", P, k, r.Err)
			}
			if len(r.State.Phases) < 2 {
				t.Fatalf("P=%d k=%d : la phase de tableau n'a pas été atteinte", P, k)
			}
			ph := r.State.Phases[1]
			somme, exempts := 0, 0
			for _, p := range ph.Entrants {
				somme += ph.Lives[p]
				if ph.Lives[p] >= 2 {
					exempts++
				}
			}
			if somme != cible {
				t.Errorf("P=%d k=%d : somme des vies à l'entrée du tableau = %d, attendu %d",
					P, k, somme, cible)
			}
			// un tableau complet : les seules places vides sont les partenaires des exempts
			var byes int
			for _, ev := range r.Journal {
				if ev.Kind == tournoi.EvDraw && ev.Phase == 1 && ev.Draw != nil {
					if len(ev.Draw.Slots) != cible {
						t.Errorf("P=%d k=%d : tableau de %d places, attendu %d", P, k, len(ev.Draw.Slots), cible)
					}
					for _, s := range ev.Draw.Slots {
						if s == tournoi.BYE {
							byes++
						}
					}
				}
			}
			if byes != exempts {
				t.Errorf("P=%d k=%d : %d places vides pour %d exempts — le tableau n'est pas plein",
					P, k, byes, exempts)
			}
		}
	}
}

// ---- Journal version 0 ----

// TestFixtureJournalV0 : un journal écrit avant les codes structurés — aucun champ `version`,
// aucun champ `round`, un `label` qui était une CHAÎNE — se rejoue et donne le classement
// attendu. La fixture est dans le dépôt, pour qu'elle ne puisse pas être « mise à jour » par
// mégarde en même temps que le moteur.
func TestFixtureJournalV0(t *testing.T) {
	b, err := os.ReadFile("testdata/journal_v0.json")
	if err != nil {
		t.Fatal(err)
	}
	j, err := tournoi.ParseJournal(b)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	for _, ev := range j {
		if ev.Version != 0 {
			t.Fatalf("la fixture doit être un journal v0, l'événement %d porte la version %d", ev.Seq, ev.Version)
		}
	}
	st, err := tournoi.Replay(j)
	if err != nil {
		t.Fatalf("rejeu : %v", err)
	}
	if !st.Finished {
		t.Fatal("le tournoi de la fixture devrait être clos")
	}
	if len(st.Warnings) != 0 {
		t.Errorf("avertissements au rejeu d'un journal v0 : %v", st.Warnings)
	}
	attendu := []struct {
		rang   int
		joueur tournoi.PlayerID
	}{
		{1, "j12"}, {2, "j09"}, {3, "j04"}, {3, "j07"}, {5, "j01"}, {5, "j10"},
		{7, "j05"}, {7, "j06"}, {7, "j08"}, {10, "j02"}, {10, "j03"}, {10, "j11"},
	}
	if len(st.Final) != len(attendu) {
		t.Fatalf("classement de %d joueurs, attendu %d", len(st.Final), len(attendu))
	}
	for i, a := range attendu {
		if st.Final[i].Rank != a.rang || st.Final[i].Player != a.joueur {
			t.Errorf("place %d : %d %s, attendu %d %s", i, st.Final[i].Rank, st.Final[i].Player, a.rang, a.joueur)
		}
	}
	// Le numéro de ronde était relu dans le texte « Ronde k » : la conversion à la lecture doit
	// l'avoir retrouvé.
	if st.Phases[0].Round < 2 {
		t.Errorf("la ronde d'un journal v0 n'a pas été retrouvée : %d", st.Phases[0].Round)
	}
}

// ---- Fuzzing ----

// cohérent vérifie ce qui doit rester vrai quoi qu'on applique. Une erreur est une réponse
// acceptable d'Apply ; un état incohérent ne l'est pas.
func cohérent(t *testing.T, st *tournoi.State) {
	t.Helper()
	if len(st.Matches) != len(st.MatchOrder) {
		t.Fatalf("%d matchs pour %d identifiants ordonnés", len(st.Matches), len(st.MatchOrder))
	}
	vus := map[tournoi.MatchID]bool{}
	for _, id := range st.MatchOrder {
		m := st.Matches[id]
		if m == nil {
			t.Fatalf("match %s dans l'ordre mais absent de la carte", id)
		}
		if vus[id] {
			t.Fatalf("match %s deux fois dans l'ordre", id)
		}
		vus[id] = true
		if m.A == m.B {
			t.Fatalf("match %s : un joueur contre lui-même", id)
		}
		if m.Status == tournoi.Finished && m.Winner != m.A && m.Winner != m.B {
			t.Fatalf("match %s terminé : vainqueur %s absent du match", id, m.Winner)
		}
	}
	for _, ph := range st.Phases {
		for _, p := range ph.Entrants {
			if _, ok := st.Players[p]; !ok {
				t.Fatalf("phase %d : entrant inconnu %s", ph.Index, p)
			}
		}
		for p, n := range ph.Losses {
			if n < 0 {
				t.Fatalf("phase %d : %s a %d défaites", ph.Index, p, n)
			}
		}
	}
	rangs := map[tournoi.PlayerID]bool{}
	for _, rk := range st.Ranking() {
		if rangs[rk.Player] {
			t.Fatalf("le joueur %s apparaît deux fois au classement", rk.Player)
		}
		rangs[rk.Player] = true
	}
	st.Propose() // ne doit pas paniquer
}

func cfgFuzz() tournoi.Config {
	return tournoi.Config{Name: "Fuzz", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindSwissLives, Length: 7, Target: 4},
		{Kind: tournoi.KindLivesBracket, Length: 9}}}
}

// FuzzApply : des suites d'événements tirées au hasard, appliquées une à une. Aucun panic,
// aucun état incohérent — une erreur est une réponse, un arrêt n'en est pas une.
//
// Les octets ne sont pas du JSON mais un PLAN : chaque triplet choisit un type d'événement et
// ses paramètres parmi ce que l'état contient. Du JSON aléatoire n'atteindrait presque jamais un
// événement valide, et ne testerait donc que le décodeur.
func FuzzApply(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 1, 1, 2, 0, 0, 3, 0, 0, 4, 1, 2})
	f.Add([]byte{1, 0, 0, 1, 1, 0, 2, 0, 5, 2, 1, 9, 6, 0, 0, 5, 0, 0})
	f.Add([]byte{7, 3, 3, 8, 0, 0, 1, 2, 2, 2, 2, 2, 3, 3, 3})
	f.Fuzz(func(t *testing.T, plan []byte) {
		now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
		st, _, err := tournoi.New(cfgFuzz(), 1, now)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 8; i++ {
			p := tournoi.Player{ID: tournoi.PlayerID(fmt.Sprintf("p%d", i)), Name: fmt.Sprintf("J%d", i)}
			if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
				t.Fatal(err)
			}
		}
		joueur := func(b byte) tournoi.PlayerID { return tournoi.PlayerID(fmt.Sprintf("p%d", int(b)%8)) }
		match := func(b byte) tournoi.MatchID {
			if len(st.MatchOrder) == 0 {
				return "M1"
			}
			return st.MatchOrder[int(b)%len(st.MatchOrder)]
		}
		for i := 0; i+2 < len(plan); i += 3 {
			now = now.Add(time.Minute)
			a, b, c := plan[i], plan[i+1], plan[i+2]
			var ev tournoi.Event
			switch a % 10 {
			case 0: // confirmer une proposition du moteur
				acts := st.ProposeAt(now)
				if len(acts) == 0 {
					continue
				}
				act := acts[int(b)%len(acts)]
				if act.Kind == tournoi.ActWait {
					continue
				}
				e, err := st.EventFromAction(act, now)
				if err != nil {
					continue
				}
				ev = e
			case 1:
				ev = tournoi.ResultEvent(match(b), joueur(c), int(b)%20, int(c)%20, now)
			case 2:
				ev = tournoi.CorrectionEvent(match(b), joueur(c), int(c)%20, int(b)%20, now)
			case 3:
				ev = tournoi.CancelEvent(match(b), now)
			case 4:
				ev = tournoi.PlayerWithdrawnEvent(joueur(b), now)
			case 5:
				ev = tournoi.PlayerWithdrawnAfterCurrentEvent(joueur(b), now)
			case 6:
				ev = tournoi.LengthChangedEvent(int(b)%3, int(c)%25, now)
			case 7:
				ev = tournoi.ForfeitEvent(match(b), joueur(c), now)
			case 8:
				ev = tournoi.PlayerAddedEvent(tournoi.Player{ID: joueur(b), Name: "R"}, now)
			case 9:
				cfg := cfgFuzz()
				cfg.Phases[0].Target = 1 << (int(b) % 4)
				cfg.Phases[1].Length = 1 + int(c)%20
				ev = tournoi.ConfigChangedEvent(cfg, now)
			}
			_ = st.Apply(ev) // une erreur est une réponse acceptable
			cohérent(t, st)
		}
	})
}

// FuzzReplayJournal : des octets quelconques présentés comme un journal. C'est le fichier que
// l'hôte a stocké et qui revient tronqué, réordonné ou corrompu : Replay doit rendre une erreur,
// jamais paniquer.
func FuzzReplayJournal(f *testing.F) {
	for _, nom := range []string{"suisse2_rondes", "elim_conso"} {
		r := sim.Run(configs()[nom], sim.Champ(8, 6, 2, 2, 10, rand.New(rand.NewSource(1))), sim.Options{Seed: 1})
		if r.Err == nil {
			if b, err := json.Marshal(r.Journal); err == nil {
				f.Add(b)
			}
		}
	}
	f.Add([]byte(`[{"kind":"created"}]`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, données []byte) {
		j, err := tournoi.ParseJournal(données)
		if err != nil {
			return
		}
		st, err := tournoi.Replay(j)
		if st == nil {
			if err == nil {
				t.Fatal("Replay a rendu un état nul sans erreur")
			}
			return
		}
		cohérent(t, st)
		// et le journal se re-sérialise sans perdre d'événement
		if b, err := j.Bytes(); err == nil {
			j2, err := tournoi.ParseJournal(b)
			if err != nil {
				t.Fatalf("aller-retour du journal : %v", err)
			}
			if len(j2) != len(j) {
				t.Fatalf("aller-retour : %d événements au lieu de %d", len(j2), len(j))
			}
		}
	})
}
