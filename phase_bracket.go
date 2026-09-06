package tournoi

import (
	"fmt"
	"math/rand"
	"sort"
)

// ---- Tableaux : élimination simple, tableau à vies (exemptions), consolante, dernière chance,
// double élimination (réconciliation, recharge) ----

func pow2ceil(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// drawSlots tire les places du premier tour. Les joueurs à 2 vies occupent une paire (exempts) ;
// les BYE supplémentaires sont répartis en seconde position des paires restantes.
func drawSlots(entrants []PlayerID, lives map[PlayerID]int, rng *rand.Rand) []PlayerID {
	ids := sortedIDs(entrants)
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	sum := 0
	var deux, une []PlayerID
	for _, p := range ids {
		if lives[p] >= 2 {
			deux = append(deux, p)
			sum += 2
		} else {
			une = append(une, p)
			sum++
		}
	}
	size := pow2ceil(sum)
	if size < 2 {
		size = 2
	}
	nPairs := size / 2
	pairs := make([][2]PlayerID, nPairs)
	filled := make([]bool, nPairs)
	// exempts : une paire sur deux d'abord, pour les écarter dans le tableau
	order := []int{}
	for i := 0; i < nPairs; i += 2 {
		order = append(order, i)
	}
	for i := 1; i < nPairs; i += 2 {
		order = append(order, i)
	}
	k := 0
	for _, p := range deux {
		pairs[order[k]] = [2]PlayerID{p, BYE}
		filled[order[k]] = true
		k++
	}
	// paires restantes : joueurs à une vie deux à deux ; BYE supplémentaires en seconde position
	extra := size - sum
	rest := append([]PlayerID{}, une...)
	for i := 0; i < nPairs; i++ {
		if filled[i] {
			continue
		}
		var a, b PlayerID = BYE, BYE
		if len(rest) > 0 {
			a, rest = rest[0], rest[1:]
		}
		if extra > 0 && a != BYE {
			extra--
		} else if len(rest) > 0 {
			b, rest = rest[0], rest[1:]
		}
		pairs[i] = [2]PlayerID{a, b}
	}
	slots := make([]PlayerID, 0, size)
	for _, pr := range pairs {
		slots = append(slots, pr[0], pr[1])
	}
	return slots
}

// buildBracket crée les sections d'un tableau à partir des places tirées.
func (s *State) buildBracket(ph *PhaseState, slots []PlayerID) {
	cfg := ph.Cfg
	main := bracketSection("main", "main", slots, ph.Length, cfg.FinalLength)
	ph.Sections = []*Section{main}
	if cfg.Consolation && len(main.Rounds) >= 2 {
		conso := consolationSection("conso", main, ph.Length)
		ph.Sections = append(ph.Sections, conso)
		if cfg.LastChance && len(conso.Rounds) >= 3 {
			// dernière chance : perdants des rondes de consolante sauf les deux dernières
			var srcs []Src
			for r := 0; r < len(conso.Rounds)-2; r++ {
				for _, i := range conso.Rounds[r] {
					srcs = append(srcs, Src{From: i, Section: "conso", Loser: true})
				}
			}
			ph.Sections = append(ph.Sections, bracketFromSrcs("last", "last", srcs, ph.Length))
		}
		if cfg.Reconciliation {
			gf := &Section{Name: "gf", Kind: "gf"}
			mf, cf := len(main.Matches)-1, len(conso.Matches)-1
			gf.Matches = append(gf.Matches, GMatch{Key: "gf.0", Label: "Grande finale", Length: ph.Length,
				Src: [2]Src{{From: mf, Section: "main"}, {From: cf, Section: "conso"}}})
			if cfg.FinalLength > 0 {
				gf.Matches[0].Length = cfg.FinalLength
			}
			if cfg.Recharge {
				gf.Matches = append(gf.Matches, GMatch{Key: "gf.1", Label: "Grande finale (recharge)", Length: gf.Matches[0].Length,
					Src: [2]Src{{From: 0}, {From: 0, Loser: true}}, Cond: true, CondFrom: 0, CondSide: 1})
			}
			gf.Rounds = [][]int{{0}}
			if cfg.Recharge {
				gf.Rounds = append(gf.Rounds, []int{1})
			}
			ph.Sections = append(ph.Sections, gf)
		}
	}
	ph.Drawn = true
	ph.resolve()
}

// bracketFromSrcs : tableau à élimination simple dont les places du premier tour sont des sources
// (perdants d'autres matchs) ; complété par des BYE en seconde position.
func bracketFromSrcs(name, kind string, srcs []Src, length int) *Section {
	size := pow2ceil(len(srcs))
	if size < 2 {
		size = 2
	}
	slots := make([]PlayerID, size)
	for i := range slots {
		slots[i] = BYE
	}
	sec := bracketSection(name, kind, slots, length, 0)
	// remplace les BYE du premier tour par les sources, en première position de chaque paire d'abord
	k := 0
	for _, mi := range sec.Rounds[0] {
		if k < len(srcs) {
			sec.Matches[mi].Src[0] = srcs[k]
			k++
		}
	}
	for _, mi := range sec.Rounds[0] {
		if k < len(srcs) {
			sec.Matches[mi].Src[1] = srcs[k]
			k++
		}
	}
	for i := range sec.Matches {
		if i < len(sec.Rounds[0]) {
			sec.Matches[i].Label = "Dernière chance, tour 1"
		} else {
			sec.Matches[i].Label = "Dernière chance, " + sec.Matches[i].Label
		}
	}
	return sec
}

func (s *State) applyDraw(ph *PhaseState, section string, d *Draw) error {
	switch ph.Cfg.Kind {
	case KindBracket, KindLivesBracket:
		if len(d.Slots) < 2 || (len(d.Slots)&(len(d.Slots)-1)) != 0 {
			return fmt.Errorf("tirage : le nombre de places doit être une puissance de 2")
		}
		s.buildBracket(ph, d.Slots)
	case KindGSL:
		return s.applyGSLDraw(ph, d)
	case KindRoundRobin:
		return s.applyRRDraw(ph, section, d)
	default:
		return fmt.Errorf("la phase %s n'a pas de tirage", ph.Cfg.Kind)
	}
	return nil
}

func (s *State) proposeBracket(ph *PhaseState) []Action {
	if !ph.Drawn {
		if len(ph.Entrants) < 2 {
			return nil
		}
		slots := drawSlots(ph.Entrants, ph.Lives, s.rng())
		return []Action{{Kind: ActDraw, Phase: ph.Index, Label: fmt.Sprintf("%s : tableau de %d places", ph.Cfg.Name, len(slots)),
			Draw: &Draw{Slots: slots, Lives: copyLives(ph)}}}
	}
	var acts []Action
	for _, r := range s.readyFree(ph) {
		lbl := r.g.Label
		if r.sec.Kind == "conso" || r.sec.Kind == "last" || r.sec.Kind == "gf" {
			lbl = r.g.Label
		} else if r.sec.Kind == "main" && len(ph.Sections) > 1 {
			lbl = "Principal, " + r.g.Label
		}
		acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Section: r.sec.Name, Key: r.g.Key, Label: lbl,
			A: r.g.Players[0], B: r.g.Players[1], Length: r.g.Length})
	}
	return acts
}

func copyLives(ph *PhaseState) map[PlayerID]int {
	out := map[PlayerID]int{}
	for _, p := range ph.Entrants {
		out[p] = ph.Lives[p]
	}
	return out
}

// bracketRanking : vainqueur (grande finale ou principal), puis par section et profondeur atteinte.
func (s *State) bracketRanking(ph *PhaseState) []Rank {
	prio := map[string]int{"gf": 4, "main": 3, "conso": 2, "last": 1, "se": 3}
	type sc struct {
		p     PlayerID
		score int
		note  string
	}
	scores := map[PlayerID]sc{}
	for _, p := range ph.Entrants {
		scores[p] = sc{p, -1, "non classé"}
	}
	if !ph.Drawn {
		var out []Rank
		for _, p := range ph.Entrants {
			out = append(out, Rank{Player: p, Rank: 1, Note: "en attente du tirage"})
		}
		return out
	}
	// pour chaque joueur, on retient la « sortie » de plus faible priorité (dernière section jouée)
	for _, sec := range ph.Sections {
		for r, idx := range sec.Rounds {
			for _, i := range idx {
				g := sec.Matches[i]
				if !g.Done || g.Walkover || g.Skipped {
					continue
				}
				depth := r
				// perdant : sortie dans cette section à la profondeur r
				cur := scores[g.Loser]
				sc2 := prio[sec.Kind]*1000 + depth
				if cur.score < 0 || prio[sec.Kind] < cur.score/1000 || (prio[sec.Kind] == cur.score/1000 && sc2 > cur.score) {
					scores[g.Loser] = sc{g.Loser, sc2, fmt.Sprintf("%s, %s", sec.Name, g.Label)}
				}
			}
		}
	}
	// vainqueurs de section : profondeur maximale
	for _, sec := range ph.Sections {
		if len(sec.Matches) == 0 {
			continue
		}
		last := sec.Matches[len(sec.Matches)-1]
		if sec.Kind == "gf" {
			// vainqueur de la GF = celui qui a gagné le dernier match joué de la section
			for i := len(sec.Matches) - 1; i >= 0; i-- {
				if sec.Matches[i].Done && !sec.Matches[i].Skipped {
					last = sec.Matches[i]
					break
				}
			}
		}
		if last.Done && !last.Skipped && last.Winner != BYE {
			w := last.Winner
			cur := scores[w]
			sc2 := prio[sec.Kind]*1000 + 999
			if cur.score < 0 || sc2 > cur.score || sec.Kind == "gf" {
				scores[w] = sc{w, sc2, "vainqueur " + sec.Name}
			}
			if sec.Kind == "gf" { // le perdant de la GF est finaliste, devant tout le monde
				l := last.Loser
				scores[l] = sc{l, prio["gf"]*1000 + 998, "finaliste"}
			}
		}
	}
	var list []sc
	for _, p := range ph.Entrants {
		v := scores[p]
		if s.Withdrawn[p] && v.score < 0 {
			v.note = "forfait"
		}
		if v.score < 0 {
			v.score = 0
			if v.note == "non classé" {
				v.note = "en cours"
				v.score = 5000
			}
		}
		list = append(list, v)
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].score > list[b].score })
	out := make([]Rank, len(list))
	rank := 1
	for i := range list {
		if i > 0 && list[i].score != list[i-1].score {
			rank = i + 1
		}
		out[i] = Rank{Player: list[i].p, Rank: rank, Note: list[i].note}
	}
	return out
}

// bracketSurvivors : vainqueur final si le tableau est terminé, sinon joueurs encore en course.
func (s *State) bracketSurvivors(ph *PhaseState) []PlayerID {
	if !ph.Drawn {
		return ph.Entrants
	}
	r := s.bracketRanking(ph)
	if ph.allDone() && len(r) > 0 {
		return []PlayerID{r[0].Player}
	}
	var out []PlayerID
	for _, rk := range r {
		if rk.Note == "en cours" {
			out = append(out, rk.Player)
		}
	}
	return out
}
