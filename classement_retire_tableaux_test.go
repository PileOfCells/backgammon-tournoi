package tournoi_test

import (
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// TestRetireApresUneVictoireDansChaqueFormat (#25) : un joueur gagne son premier match puis se
// retire, hors de tout match. Son match suivant est perdu par forfait, sans être joué. Il reste
// classé sur son parcours (N20) : la note « retiré » dit où il est sorti, et il précède les
// joueurs éliminés avant lui — il n'est pas relégué en queue sans section.
func TestRetireApresUneVictoireDansChaqueFormat(t *testing.T) {
	formats := map[string]tournoi.PhaseConfig{
		"lives_bracket":    {Kind: tournoi.KindLivesBracket, Length: 7},
		"bracket":          {Kind: tournoi.KindBracket, Length: 7},
		"bracket_conso":    {Kind: tournoi.KindBracket, Length: 7, Consolation: true},
		"bracket_last":     {Kind: tournoi.KindBracket, Length: 7, Consolation: true, LastChance: true},
		"bracket_reconcil": {Kind: tournoi.KindBracket, Length: 7, Consolation: true, Reconciliation: true, Recharge: true},
		"gsl":              {Kind: tournoi.KindGSL, Length: 7},
		"round_robin":      {Kind: tournoi.KindRoundRobin, Length: 7, GroupSize: 4, Qualifiers: 2},
		"swiss_lives":      {Kind: tournoi.KindSwissLives, Length: 7, Lives: 2},
	}
	for nom, pc := range formats {
		t.Run(nom, func(t *testing.T) {
			now := time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC)
			st := tournoiDeTest(t, tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{pc}}, 16)
			var parti tournoi.PlayerID
			for étape := 0; étape < 500 && !st.Finished; étape++ {
				progrès := false
				for _, a := range st.ProposeAt(now) {
					if a.Kind == tournoi.ActWait {
						continue
					}
					ev, err := st.EventFromAction(a, now)
					if err != nil {
						t.Fatal(err)
					}
					if err := st.Apply(ev); err != nil {
						t.Fatal(err)
					}
					progrès = true
					if a.Kind != tournoi.ActStartMatch {
						break
					}
					now = now.Add(time.Minute)
					if err := st.Apply(tournoi.ResultEvent(ev.MatchID, a.A, 7, 3, now)); err != nil {
						t.Fatal(err)
					}
					if parti == "" {
						parti = a.A
						if err := st.Apply(tournoi.PlayerWithdrawnEvent(parti, now)); err != nil {
							t.Fatal(err)
						}
						// Aussitôt, avant même que son prochain adversaire soit connu : il n'est
						// plus « en course » et la note dit où il sort.
						if r := rangDe(st, parti); r.Note.Kind != tournoi.NoteWithdrawn ||
							(pc.Kind != tournoi.KindSwissLives && pc.Kind != tournoi.KindGSL && r.Note.Section == "") {
							t.Errorf("juste après le retrait : %+v", r.Note)
						}
						break
					}
				}
				if !progrès {
					break
				}
			}
			if !st.Finished || parti == "" {
				t.Fatalf("tournoi non terminé (%v) ou personne n'est parti (%q)", st.Finished, parti)
			}
			r := rangDe(st, parti)
			if r.Note.Kind != tournoi.NoteWithdrawn {
				t.Errorf("note du retiré : %+v, attendu %q", r.Note, tournoi.NoteWithdrawn)
			}
			if pc.Kind != tournoi.KindSwissLives && pc.Kind != tournoi.KindGSL && r.Note.Section == "" {
				t.Errorf("la note doit dire où il est sorti : %+v", r.Note)
			}
			if (pc.Kind == tournoi.KindSwissLives || pc.Kind == tournoi.KindGSL || pc.Kind == tournoi.KindRoundRobin) && r.Note.Wins != 1 {
				t.Errorf("la note doit porter sa victoire : %+v", r.Note)
			}
			// Il précède au moins un joueur, ou partage son rang avec un éliminé non retiré.
			classé := false
			for _, x := range st.Ranking() {
				if x.Rank > r.Rank || (x.Rank == r.Rank && x.Player != parti && x.Note.Kind != tournoi.NoteWithdrawn) {
					classé = true
				}
			}
			if !classé {
				t.Errorf("le retiré après une victoire est classé %d/16, en queue : %+v", r.Rank, r.Note)
			}
			if len(st.Warnings) != 0 {
				t.Errorf("avertissements : %v", st.Warnings)
			}
		})
	}
}
