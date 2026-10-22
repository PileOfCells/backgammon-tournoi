package tournoi_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
	"github.com/PileOfCells/backgammon-tournoi/rules"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// rules/testdata/appariements.json : des instantanés de phase (rules.Phase) pris par le moteur
// dans des tournois simulés — chaque configuration de sim_test.go, à 5 joueurs, et à 13
// perturbé —, avec ce qu'en tirent Pair, PhaseDone, Rank, RankSection, PoolQualified et
// RemainingMatches, puis les classements de phase et leur fusion (MergeRankings). Les tests de
// rules rejouent le fichier contre les fonctions pures ; ce test-ci le réécrit à l'identique
// depuis le moteur, et vérifie au passage que les enveloppes (Ranking, SectionRanking) rendent
// ce que rendent les règles. La parité des propositions elles-mêmes est la trace dorée
// (trace_dores_test.go).

var pasRelevés = map[int]bool{3: true, 15: true, 40: true}

func lignes[T any](rk []T) []string {
	var out []string
	for _, r := range rk {
		b, _ := json.Marshal(r)
		out = append(out, string(b))
	}
	return out
}

// normaliser : une ligne de classement du moteur et une de rules s'écrivent de même, la note
// vide mise à part (omise d'un côté, écrite vide de l'autre).
func normaliser(rk []tournoi.Rank) []rules.Standing {
	out := make([]rules.Standing, len(rk))
	for i, r := range rk {
		var n rules.Note
		b, _ := json.Marshal(r.Note)
		_ = json.Unmarshal(b, &n)
		out[i] = rules.Standing{Player: rules.Ref(r.Player), Rank: r.Rank, Note: n}
	}
	return out
}

func casRègles(t *testing.T) dores.Rules {
	cfgs := configs()
	var noms []string
	for n := range cfgs {
		noms = append(noms, n)
	}
	sort.Strings(noms)
	out := dores.Rules{Phases: []dores.PhaseCase{}, Merges: []dores.MergeCase{}}
	for _, nom := range noms {
		for _, cas := range []struct {
			P    int
			pert bool
		}{{5, false}, {13, true}} {
			nomCas := fmt.Sprintf("%s P=%d", nom, cas.P)
			if cas.pert {
				nomCas += " perturbé"
			}
			rng := rand.New(rand.NewSource(int64(cas.P * 1000)))
			joueurs := sim.Champ(cas.P, 6, 2, 2, 10, rng)
			for i := range joueurs {
				if i%3 == 1 {
					joueurs[i].Club = fmt.Sprintf("club%d", i%4)
				}
			}
			fusion := func(st *tournoi.State, quand string) {
				var parPhase [][]rules.Standing
				for i := range st.Phases {
					parPhase = append(parPhase, rules.Rank(*st.RulesPhase(i)))
				}
				fus := rules.MergeRankings(parPhase)
				if a, b := lignes(normaliser(st.Ranking())), lignes(fus); fmt.Sprint(a) != fmt.Sprint(b) {
					t.Fatalf("%s %s : Ranking n'est pas rules.MergeRankings\n%v\n%v", nomCas, quand, a, b)
				}
				out.Merges = append(out.Merges, dores.MergeCase{Name: nomCas + " " + quand, ByPhase: parPhase, Out: fus})
			}
			var dernier *tournoi.State
			tracer(cfgs[nom], joueurs, 0, cas.pert, func(st *tournoi.State, pas int) {
				dernier = st
				if !pasRelevés[pas] || st.Current < 0 || st.Finished {
					return
				}
				p := *st.RulesPhase(st.Current)
				c := dores.PhaseCase{Name: fmt.Sprintf("%s pas %d", nomCas, pas), Phase: p,
					Seed: rules.Seed{Base: st.Seed, Events: st.NEvents, Phase: st.Current}}
				c.Out = sortiesPhase(c.Phase, c.Seed)
				for nomSec, rk := range c.Out.Sections {
					if a, b := lignes(normaliser(st.SectionRanking(nomSec))), lignes(rk); fmt.Sprint(a) != fmt.Sprint(b) {
						t.Fatalf("%s : SectionRanking(%s) n'est pas rules.RankSection", c.Name, nomSec)
					}
				}
				out.Phases = append(out.Phases, c)
			})
			if dernier != nil {
				fusion(dernier, "fin")
			}
		}
	}
	return out
}

// sortiesPhase : ce que les règles tirent d'un instantané.
func sortiesPhase(p rules.Phase, seed rules.Seed) dores.PhaseOut {
	o := dores.PhaseOut{Pairs: rules.Pair(p, seed), Done: rules.PhaseDone(p), Rank: rules.Rank(p),
		Remaining: rules.RemainingMatches(p)}
	if o.Pairs == nil {
		o.Pairs = []rules.Pairing{}
	}
	if o.Rank == nil {
		o.Rank = []rules.Standing{}
	}
	if len(p.Sections) > 0 {
		o.Sections = map[string][]rules.Standing{}
		for _, sec := range p.Sections {
			o.Sections[sec.Name] = rules.RankSection(sec, p.Withdrawn)
		}
	}
	if p.Format == rules.FormatRoundRobin {
		o.Qualified = rules.PoolQualified(p)
	}
	return o
}

// TestDoresAppariements : le moteur réécrit rules/testdata/appariements.json à l'identique.
func TestDoresAppariements(t *testing.T) {
	if testing.Short() && !*majDores {
		t.Skip("rejoué contre les fonctions pures par les tests de rules")
	}
	comparerDore(t, "rules/testdata/appariements.json", casRègles(t))
}
