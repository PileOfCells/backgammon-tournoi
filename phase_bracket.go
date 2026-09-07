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
//
// Avec seeding == SeedingRating, le hasard cède la place au placement classique par cote
// (seeding.go) ; sans l'option — le défaut — le tirage est intégralement aléatoire, ce qui est
// un choix de conception et non un oubli (voir SeedingRating).
func drawSlots(entrants []PlayerID, lives map[PlayerID]int, seeding string, rating func(PlayerID) float64, rng *rand.Rand) []PlayerID {
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
	if seeding == SeedingRating {
		return seededSlots(deux, une, rating, size)
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
	main := bracketSection(secMain, secMain, slots, bracketLengths(ph))
	ph.Sections = []*Section{main}
	if cfg.Consolation && len(main.Rounds) >= 2 {
		conso := consolationSection(secConso, main, ph.Length)
		ph.Sections = append(ph.Sections, conso)
		if cfg.LastChance && len(conso.Rounds) >= 3 {
			// dernière chance : perdants des rondes de consolante sauf les deux dernières
			var srcs []Src
			for r := 0; r < len(conso.Rounds)-2; r++ {
				for _, i := range conso.Rounds[r] {
					srcs = append(srcs, Src{From: i, Section: secConso, Loser: true})
				}
			}
			ph.Sections = append(ph.Sections, bracketFromSrcs(secLast, secLast, srcs, ph.Length))
		}
		if cfg.Reconciliation {
			gf := &Section{Name: secGrandFinal, Kind: secGrandFinal}
			mf, cf := len(main.Matches)-1, len(conso.Matches)-1
			// la grande finale EST la finale du tournoi : elle prend la longueur de finale,
			// qu'elle vienne de Lengths ou de FinalLength (lengthPlan.at d'un tableau à un tour).
			gf.Matches = append(gf.Matches, GMatch{Key: "gf.0", Label: Label{Kind: LabelGrandFinal}, Length: bracketLengths(ph).at(0, 1),
				Src: [2]Src{{From: mf, Section: secMain}, {From: cf, Section: secConso}}})
			if cfg.Recharge {
				gf.Matches = append(gf.Matches, GMatch{Key: "gf.1", Label: Label{Kind: LabelGrandFinalRecharge}, Length: gf.Matches[0].Length,
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
	ph.resolve(s.Withdrawn)
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
	sec := bracketSection(name, kind, slots, plain(length))
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
			sec.Matches[i].Label = Label{Kind: LabelLastChance}
		} else {
			sec.Matches[i].Label = Label{Kind: LabelLastChance}.with(sec.Matches[i].Label)
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
		slots := drawSlots(ph.Entrants, ph.Lives, ph.Cfg.Seeding, s.rating, s.rng())
		return []Action{{Kind: ActDraw, Phase: ph.Index,
			Label: Label{Kind: LabelDrawBracket, N: len(slots)}.with(Label{Kind: LabelPhase, Text: PhaseName(ph.Cfg)}),
			Draw:  &Draw{Slots: slots, Lives: copyLives(ph)}}}
	}
	var acts []Action
	for _, r := range s.readyFree(ph) {
		lbl := r.g.Label
		if r.sec.Kind == secMain && len(ph.Sections) > 1 {
			lbl = Label{Kind: LabelMainDraw}.with(r.g.Label)
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
	prio := map[string]int{secGrandFinal: 4, secMain: 3, secConso: 2, secLast: 1, "se": 3}
	type sc struct {
		p     PlayerID
		score int
		note  Note
	}
	scores := map[PlayerID]sc{}
	for _, p := range ph.Entrants {
		scores[p] = sc{p, -1, Note{Kind: NoteUnranked}}
	}
	if !ph.Drawn {
		var out []Rank
		for _, p := range ph.Entrants {
			out = append(out, Rank{Player: p, Rank: 1, Note: Note{Kind: NoteAwaitingDraw}})
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
					scores[g.Loser] = sc{g.Loser, sc2, Note{Kind: NoteSectionExit, Section: sec.Name, Sub: labelPtr(g.Label)}}
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
		if sec.Kind == secGrandFinal {
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
			if cur.score < 0 || sc2 > cur.score || sec.Kind == secGrandFinal {
				scores[w] = sc{w, sc2, Note{Kind: NoteSectionWinner, Section: sec.Name}}
			}
			if sec.Kind == secGrandFinal { // le perdant de la GF est finaliste, devant tout le monde
				l := last.Loser
				scores[l] = sc{l, prio[secGrandFinal]*1000 + 998, Note{Kind: NoteFinalist}}
			}
		}
	}
	var list []sc
	for _, p := range ph.Entrants {
		v := scores[p]
		if s.Withdrawn[p] && v.score < 0 {
			v.note = Note{Kind: NoteForfeit}
		}
		if v.score < 0 {
			v.score = 0
			if v.note.Kind == NoteUnranked {
				v.note = Note{Kind: NoteRunning}
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
		if rk.Note.Kind == NoteRunning {
			out = append(out, rk.Player)
		}
	}
	return out
}

// SectionRanking : le classement propre d'une section (main, conso, last, gf, un groupe GSL, une
// poule), et non le classement général du tournoi.
//
// Une consolante a ses propres prix, donc son propre classement : celui de ses joueurs, par tour
// atteint DANS CETTE SECTION. Le classement général mélange les sections par priorité (un
// vainqueur de consolante passe derrière un demi-finaliste du principal) ; ici on ne regarde que
// la section, et le vainqueur de la consolante est premier de la consolante.
//
// La section est cherchée de la dernière phase vers la première : c'est la plus récente qui
// porte ce nom qui compte, un tournoi pouvant enchaîner deux tableaux.
func (s *State) SectionRanking(name string) []Rank {
	for i := len(s.Phases) - 1; i >= 0; i-- {
		if sec := s.Phases[i].section(name); sec != nil {
			return s.sectionRanking(s.Phases[i], sec)
		}
	}
	return nil
}

func (s *State) sectionRanking(ph *PhaseState, sec *Section) []Rank {
	// Ordre de rencontre dans le graphe : stable, et indépendant de toute map.
	var ordre []PlayerID
	vu := map[PlayerID]bool{}
	for i := range sec.Matches {
		for _, p := range sec.Matches[i].Players {
			if p == "" || p == BYE || vu[p] {
				continue
			}
			vu[p] = true
			ordre = append(ordre, p)
		}
	}
	profondeur := map[PlayerID]int{}
	notes := map[PlayerID]Note{}
	if sec.Kind == secKindPool || sec.Kind == secKindBarrage {
		// Une poule n'a pas de tour atteint : le classement y est le nombre de victoires.
		for i := range sec.Matches {
			g := sec.Matches[i]
			if g.Done && !g.Skipped && g.Winner != BYE {
				profondeur[g.Winner]++
			}
		}
		for _, p := range ordre {
			notes[p] = Note{Kind: NotePoolRecord, Section: sec.Name, Wins: profondeur[p]}
		}
	} else {
		for r, idx := range sec.Rounds {
			for _, i := range idx {
				g := sec.Matches[i]
				if !g.Done || g.Skipped || g.Loser == BYE || g.Loser == "" {
					continue
				}
				profondeur[g.Loser] = r
				notes[g.Loser] = Note{Kind: NoteSectionExit, Section: sec.Name, Sub: labelPtr(g.Label)}
				if g.Walkover {
					notes[g.Loser] = Note{Kind: NoteForfeit}
				}
			}
		}
		// Vainqueur de la section : le gagnant du dernier match joué. Les matchs sont rangés
		// tour par tour, donc le dernier index joué est le plus profond.
		for i := len(sec.Matches) - 1; i >= 0; i-- {
			g := sec.Matches[i]
			if g.Done && !g.Skipped && g.Winner != BYE && g.Winner != "" {
				profondeur[g.Winner] = len(sec.Rounds)
				notes[g.Winner] = Note{Kind: NoteSectionWinner, Section: sec.Name}
				break
			}
		}
		for _, p := range ordre {
			if _, sorti := profondeur[p]; !sorti {
				profondeur[p] = len(sec.Rounds) // encore en course : devant les éliminés
				notes[p] = Note{Kind: NoteRunning}
			}
		}
	}
	list := append([]PlayerID(nil), ordre...)
	sort.SliceStable(list, func(a, b int) bool { return profondeur[list[a]] > profondeur[list[b]] })
	out := make([]Rank, len(list))
	rang := 1
	for i, p := range list {
		if i > 0 && profondeur[p] != profondeur[list[i-1]] {
			rang = i + 1
		}
		out[i] = Rank{Player: p, Rank: rang, Note: notes[p]}
	}
	return out
}
