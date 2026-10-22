package tournoi_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/internal/dores"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// testdata/trace_moteur.json fige ce que le moteur PROPOSE et CLASSE à chaque pas d'un tournoi
// simulé : Propose (avec et sans External), Ranking, SectionRanking de chaque section, Statuses,
// avertissements. Il a été écrit par le moteur d'AVANT le second lot de rules (appariements et
// classement en fonctions pures), et prouve que les enveloppes rendent la même chose, à l'octet.
//
// Les cas sont les configurations de sim_test.go, à chaque effectif de la liste et à trois
// graines, chacun deux fois : tel quel, puis PERTURBÉ (retraits, indisponibilités, joueurs
// occupés ailleurs), ce qui force les branches de bord — forfaits dans un graphe, orphelins d'une
// ronde, repêchage, secours. Un pas est résumé par une empreinte SHA-256 cumulée : le fichier
// reste petit, et le classement final est écrit en clair pour qu'un écart se lise.
//
// `go test -run TestDores . -args -update` le réécrit, ce qui n'a de sens qu'après un changement
// d'appariement ou de classement VOULU (une mineure au changelog des règles).

type casTrace struct {
	Name     string   `json:"name"`
	Steps    int      `json:"steps"`
	Events   int      `json:"events"`
	Err      string   `json:"err,omitempty"`
	Digest   string   `json:"digest"`
	Final    []string `json:"final"`
	Sections []string `json:"sections,omitempty"`
}

// enregistrer ajoute à l'empreinte tout ce que le moteur dit de l'état à l'instant now.
func enregistrer(h hash.Hash, st *tournoi.State, now time.Time, ext tournoi.External) {
	ecrire := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		h.Write(b)
		h.Write([]byte{'\n'})
	}
	ecrire(st.ProposeAt(now))
	ecrire(st.ProposeWith(now, ext))
	ecrire(st.Ranking())
	for _, nom := range nomsDeSections(st) {
		ecrire(st.SectionRanking(nom))
	}
	ecrire(st.Statuses())
	ecrire(st.Warnings)
	ecrire(st.Infos)
}

func nomsDeSections(st *tournoi.State) []string {
	vu := map[string]bool{}
	var out []string
	for _, ph := range st.Phases {
		for _, sec := range ph.Sections {
			if !vu[sec.Name] {
				vu[sec.Name] = true
				out = append(out, sec.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

type enJeu struct {
	id   tournoi.MatchID
	end  time.Time
	a, b tournoi.PlayerID
	n    int
}

// tracer joue le tournoi comme sim.Run, en enregistrant chaque pas ; perturber y ajoute des
// retraits, des indisponibilités et des joueurs occupés ailleurs, à des pas fixés par la graine.
func tracer(cfg tournoi.Config, players []tournoi.Player, seed int64, perturber bool, voir ...func(*tournoi.State, int)) casTrace {
	h := sha256.New()
	rng := rand.New(rand.NewSource(seed))
	alea := rand.New(rand.NewSource(seed*31 + 17))
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	st, ev, err := tournoi.New(cfg, seed, now)
	if err != nil {
		return casTrace{Err: err.Error()}
	}
	c := casTrace{}
	events := 1
	apply := func(e tournoi.Event) error {
		e.Seq = events
		if err := st.Apply(e); err != nil {
			return err
		}
		events++
		return nil
	}
	_ = ev
	for i := range players {
		if err := apply(tournoi.PlayerAddedEvent(players[i], now)); err != nil {
			c.Err = err.Error()
			return finir(c, h, st, events)
		}
	}
	externe := func() tournoi.External {
		ext := tournoi.External{BusyTables: []int{1}}
		if len(st.Order) > 0 {
			ext.BusyPlayers = []tournoi.PlayerID{st.Order[alea.Intn(len(st.Order))]}
		}
		return ext
	}
	var cours []enJeu
	for steps := 1; !st.Finished; steps++ {
		c.Steps = steps
		if steps > 6000 {
			c.Err = "sans fin"
			break
		}
		enregistrer(h, st, now, externe())
		for _, f := range voir {
			f(st, steps)
		}
		if perturber && steps > 3 {
			switch {
			case steps%17 == 5:
				// un retrait : le joueur pris au hasard, s'il n'est pas déjà parti
				p := st.Order[alea.Intn(len(st.Order))]
				if !st.Withdrawn[p] && len(st.Withdrawn) < len(st.Order)/3 {
					if err := apply(tournoi.PlayerWithdrawnEvent(p, now)); err != nil {
						c.Err = err.Error()
						return finir(c, h, st, events)
					}
					for i := 0; i < len(cours); i++ {
						if cours[i].a == p || cours[i].b == p {
							cours = append(cours[:i], cours[i+1:]...)
							i--
						}
					}
				}
			case steps%13 == 6:
				p := st.Order[alea.Intn(len(st.Order))]
				if err := apply(tournoi.PlayerUnavailableEvent(p, now.Add(25*time.Minute), now)); err != nil {
					c.Err = err.Error()
					return finir(c, h, st, events)
				}
			}
		}
		acts := st.ProposeAt(now)
		if perturber && steps%19 == 9 {
			// Un appariement à la main, au suisse : le A de la première proposition contre le A
			// de la deuxième. Leurs adversaires prévus deviennent des orphelins de la ronde.
			var libres []tournoi.Action
			for _, a := range acts {
				if a.Kind == tournoi.ActStartMatch && a.Section == "" && a.Reason == tournoi.ReasonNone {
					libres = append(libres, a)
				}
			}
			if len(libres) >= 2 {
				main := libres[0]
				main.B = libres[1].A
				e, err := st.EventFromAction(main, now)
				if err == nil {
					err = apply(e)
				}
				if err != nil {
					c.Err = err.Error()
					return finir(c, h, st, events)
				}
				cours = append(cours, enJeu{e.MatchID, now.Add(sim.Duree(main.Length, 8, rng)), main.A, main.B, main.Length})
				continue
			}
		}
		progressed := false
		for _, a := range acts {
			if a.Kind == tournoi.ActWait {
				continue
			}
			if a.Kind == tournoi.ActStartMatch && a.Reason != tournoi.ReasonNone {
				continue // retenue (absent, sans table) : l'hôte attend
			}
			e, err := st.EventFromAction(a, now)
			if err != nil {
				c.Err = err.Error()
				return finir(c, h, st, events)
			}
			if err := apply(e); err != nil {
				c.Err = err.Error()
				return finir(c, h, st, events)
			}
			progressed = true
			if a.Kind == tournoi.ActStartMatch {
				cours = append(cours, enJeu{e.MatchID, now.Add(sim.Duree(a.Length, 8, rng)), a.A, a.B, a.Length})
			}
			if a.Kind == tournoi.ActDraw || a.Kind == tournoi.ActRepechage || a.Kind == tournoi.ActNextPhase || a.Kind == tournoi.ActFinish {
				break
			}
		}
		if progressed {
			continue
		}
		// Une attente : l'échéance d'un lot, ou la fin d'une indisponibilité, ou un match.
		var attente time.Time
		for _, a := range acts {
			if a.Kind == tournoi.ActWait && !a.Until.IsZero() && a.Until.After(now) {
				attente = a.Until
			}
		}
		if len(cours) == 0 {
			if !attente.IsZero() {
				now = attente
				continue
			}
			now = now.Add(10 * time.Minute) // indisponibilité à échéance, ou blocage : le temps passe
			if steps > 5000 {
				c.Err = "blocage"
				break
			}
			continue
		}
		sort.SliceStable(cours, func(i, j int) bool { return cours[i].end.Before(cours[j].end) })
		if !attente.IsZero() && attente.Before(cours[0].end) {
			now = attente
			continue
		}
		m := cours[0]
		cours = cours[1:]
		if m.end.After(now) {
			now = m.end
		}
		if st.Matches[m.id].Status != tournoi.Running {
			continue
		}
		w := m.a
		if rng.Float64() >= sim.PGain(st.Players[m.a].Rating, st.Players[m.b].Rating, m.n) {
			w = m.b
		}
		sa, sb := m.n, rng.Intn(m.n)
		if w == m.b {
			sa, sb = sb, sa
		}
		if err := apply(tournoi.ResultEvent(m.id, w, sa, sb, now)); err != nil {
			c.Err = err.Error()
			return finir(c, h, st, events)
		}
	}
	enregistrer(h, st, now, tournoi.External{})
	return finir(c, h, st, events)
}

func finir(c casTrace, h hash.Hash, st *tournoi.State, events int) casTrace {
	c.Events = events
	c.Digest = hex.EncodeToString(h.Sum(nil))
	rk := st.Final
	if rk == nil {
		rk = st.Ranking()
	}
	c.Final = []string{}
	for _, r := range rk {
		n, _ := json.Marshal(r.Note)
		c.Final = append(c.Final, fmt.Sprintf("%d %s %s", r.Rank, r.Player, n))
	}
	for _, nom := range nomsDeSections(st) {
		var parts []string
		for _, r := range st.SectionRanking(nom) {
			parts = append(parts, fmt.Sprintf("%d:%s", r.Rank, r.Player))
		}
		c.Sections = append(c.Sections, fmt.Sprintf("%s %v", nom, parts))
	}
	return c
}

// casTraces : les configurations de sim_test.go, triées par nom, aux effectifs de bord et
// courants, trois graines, tel quel puis perturbé. court : les seuls effectifs ≤ 8.
func casTraces(court bool) []struct {
	nom string
	run func() casTrace
} {
	var noms []string
	cfgs := configs()
	for n := range cfgs {
		noms = append(noms, n)
	}
	sort.Strings(noms)
	effectifs := []int{2, 3, 4, 5, 7, 8, 13, 32}
	var out []struct {
		nom string
		run func() casTrace
	}
	for _, nom := range noms {
		cfg := cfgs[nom]
		for _, P := range effectifs {
			for k := int64(0); k < 3; k++ {
				for _, pert := range []bool{false, true} {
					nomCas := fmt.Sprintf("%s P=%d k=%d", nom, P, k)
					if pert {
						nomCas += " perturbé"
					}
					P, k, pert := P, k, pert
					if court && P > 8 {
						nomCas = "" // présent dans le fichier, pas rejoué en mode court
					}
					out = append(out, struct {
						nom string
						run func() casTrace
					}{nomCas, func() casTrace {
						rng := rand.New(rand.NewSource(int64(P*1000) + k))
						joueurs := sim.Champ(P, 6, 2, 2, 10, rng)
						for i := range joueurs {
							if i%3 == 1 {
								joueurs[i].Club = fmt.Sprintf("club%d", i%4)
							}
						}
						return tracer(cfg, joueurs, k, pert)
					}})
				}
			}
		}
	}
	return out
}

// TestDoresTrace : la trace complète du moteur, comparée cas par cas au fichier doré.
func TestDoresTrace(t *testing.T) {
	const chemin = "testdata/trace_moteur.json"
	cas := casTraces(testing.Short() && !*majDores)
	if *majDores {
		var tout []casTrace
		for _, c := range cas {
			r := c.run()
			r.Name = c.nom
			tout = append(tout, r)
		}
		comparerDore(t, chemin, tout)
		return
	}
	var dore []casTrace
	if err := dores.Read(chemin, &dore); err != nil {
		t.Fatal(err)
	}
	if len(dore) != len(cas) {
		t.Fatalf("%s : %d cas, %d attendus", chemin, len(dore), len(cas))
	}
	for i, c := range cas {
		if c.nom == "" {
			continue
		}
		r := c.run()
		r.Name = c.nom
		a, _ := json.Marshal(r)
		b, _ := json.Marshal(dore[i])
		if string(a) != string(b) {
			t.Errorf("%s : la trace du moteur a changé.\n--- produit ---\n%s\n--- doré ---\n%s", c.nom, a, b)
		}
	}
}
