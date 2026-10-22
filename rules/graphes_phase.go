package rules

import (
	"fmt"
	"math/rand"
	"sort"
)

// ---- Tableaux, blocs GSL, poules : tirages, construction des sections, appariements ----

// DrawSlots tire les places du premier tour d'un tableau. Les joueurs à 2 vies (lives) occupent
// une paire à eux seuls, contre un BYE (exempts du premier tour), une paire sur deux d'abord pour
// les écarter ; les BYE supplémentaires vont en seconde position des paires restantes. Avec
// seeding = SeedingRating, le hasard cède la place au placement classique par cote (ratings,
// PR : plus bas vaut mieux, 0 = inconnu, derrière tout le monde).
func DrawSlots(entrants []Ref, lives map[Ref]int, seeding string, ratings map[Ref]float64, seed int64) []Ref {
	return drawSlots(entrants, lives, seeding, ratings, generateur(seed))
}

func drawSlots(entrants []Ref, lives map[Ref]int, seeding string, ratings map[Ref]float64, rng *rand.Rand) []Ref {
	ids := sortedRefs(entrants)
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	sum := 0
	var deux, une []Ref
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
		return seededSlots(deux, une, ratings, size)
	}
	nPairs := size / 2
	pairs := make([][2]Ref, nPairs)
	filled := make([]bool, nPairs)
	order := []int{}
	for i := 0; i < nPairs; i += 2 {
		order = append(order, i)
	}
	for i := 1; i < nPairs; i += 2 {
		order = append(order, i)
	}
	k := 0
	for _, p := range deux {
		pairs[order[k]] = [2]Ref{p, Bye}
		filled[order[k]] = true
		k++
	}
	extra := size - sum
	rest := append([]Ref{}, une...)
	for i := 0; i < nPairs; i++ {
		if filled[i] {
			continue
		}
		var a, b Ref = Bye, Bye
		if len(rest) > 0 {
			a, rest = rest[0], rest[1:]
		}
		if extra > 0 && a != Bye {
			extra--
		} else if len(rest) > 0 {
			b, rest = rest[0], rest[1:]
		}
		pairs[i] = [2]Ref{a, b}
	}
	slots := make([]Ref, 0, size)
	for _, pr := range pairs {
		slots = append(slots, pr[0], pr[1])
	}
	return slots
}

// SeedOrder : pour chaque place d'un tableau de size places, le numéro de tête de série qui
// s'y installe — la construction classique par doublements (la place x d'un tableau de n devient
// les places x et 2n+1-x du tableau de 2n).
func SeedOrder(size int) []int {
	order := []int{1}
	for n := 1; n < size; n *= 2 {
		next := make([]int, 0, 2*n)
		for _, x := range order {
			next = append(next, x, 2*n+1-x)
		}
		order = next
	}
	return order
}

// seededSlots : les joueurs à deux vies passent devant tous les autres (les exemptions vont aux
// premières têtes), puis chacun des deux paquets par cote.
func seededSlots(deux, une []Ref, ratings map[Ref]float64, size int) []Ref {
	ordre := append(byStrength(deux, ratings), byStrength(une, ratings)...)
	slots := make([]Ref, size)
	for i := range slots {
		slots[i] = Bye
	}
	for place, tête := range SeedOrder(size) {
		if tête <= len(ordre) {
			slots[place] = ordre[tête-1]
		}
	}
	return slots
}

// byStrength : du meilleur au moins bon ; les cotes inconnues derrière, par identifiant.
func byStrength(g []Ref, ratings map[Ref]float64) []Ref {
	out := sortedRefs(g)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := ratings[out[i]], ratings[out[j]]
		inconnuI, inconnuJ := ri <= 0, rj <= 0
		if inconnuI != inconnuJ {
			return inconnuJ
		}
		if inconnuI {
			return false
		}
		return ri < rj
	})
	return out
}

// BracketSections : les sections d'un tableau tiré — le principal ; la consolante, la dernière
// chance (perdants des rondes de consolante sauf les deux dernières) et la grande finale (avec
// recharge conditionnelle) selon les règles. Les places ne sont pas résolues (Resolve).
func BracketSections(slots []Ref, length int, r PhaseRules) []Section {
	lp := LengthPlan{Default: length, Final: r.FinalLength, ByRound: r.Lengths}
	main := BracketSection(SectionMain, SectionMain, slots, lp)
	secs := []Section{main}
	if r.Consolation && len(main.Rounds) >= 2 {
		conso := ConsolationSection(SectionConso, main, length)
		secs = append(secs, conso)
		if r.LastChance && len(conso.Rounds) >= 3 {
			var srcs []Src
			for k := 0; k < len(conso.Rounds)-2; k++ {
				for _, i := range conso.Rounds[k] {
					srcs = append(srcs, Src{From: i, Section: SectionConso, Loser: true})
				}
			}
			secs = append(secs, LastChanceSection(SectionLast, srcs, length))
		}
		if r.Reconciliation {
			gf := Section{Name: SectionGrandFinal, Kind: SectionGrandFinal}
			mf, cf := len(main.Nodes)-1, len(conso.Nodes)-1
			// la grande finale EST la finale du tournoi : elle prend la longueur de finale
			gf.Nodes = append(gf.Nodes, Node{Key: "gf.0", Label: Label{Kind: LabelGrandFinal}, Length: lp.At(0, 1),
				Src: [2]Src{{From: mf, Section: SectionMain}, {From: cf, Section: SectionConso}}})
			if r.Recharge {
				gf.Nodes = append(gf.Nodes, Node{Key: "gf.1", Label: Label{Kind: LabelGrandFinalRecharge}, Length: gf.Nodes[0].Length,
					Src: [2]Src{{From: 0}, {From: 0, Loser: true}}, Cond: true, CondFrom: 0, CondSide: 1})
			}
			gf.Rounds = [][]int{{0}}
			if r.Recharge {
				gf.Rounds = append(gf.Rounds, []int{1})
			}
			secs = append(secs, gf)
		}
	}
	return secs
}

// Partition découpe une liste (déjà mélangée) en groupes GSL de 4, 3, 2 ou 1.
func Partition(g []Ref) [][]Ref {
	n := len(g)
	var sizes []int
	q, r := n/4, n%4
	switch r {
	case 0:
		for i := 0; i < q; i++ {
			sizes = append(sizes, 4)
		}
	case 3:
		for i := 0; i < q; i++ {
			sizes = append(sizes, 4)
		}
		sizes = append(sizes, 3)
	case 2:
		if q >= 1 {
			for i := 0; i < q-1; i++ {
				sizes = append(sizes, 4)
			}
			sizes = append(sizes, 3, 3)
		} else {
			sizes = append(sizes, 2)
		}
	case 1:
		if q >= 1 {
			for i := 0; i < q-1; i++ {
				sizes = append(sizes, 4)
			}
			sizes = append(sizes, 3, 2)
		} else {
			sizes = append(sizes, 1)
		}
	}
	var out [][]Ref
	k := 0
	for _, sz := range sizes {
		out = append(out, g[k:k+sz])
		k += sz
	}
	return out
}

// GSLBlockSections : les sections du bloc `block` d'un GSL tiré en groupes — un groupe GSL pour
// les joueurs sans défaite, un mini-tableau à élimination simple pour ceux qui en ont une
// (losses donne les défaites de chacun ; le premier joueur du groupe décide). Un groupe de moins
// de deux joueurs n'a pas de section mais garde son numéro.
func GSLBlockSections(block int, groups [][]Ref, losses map[Ref]int, length int) []Section {
	var out []Section
	for i, g := range groups {
		if len(g) < 2 {
			continue
		}
		name := fmt.Sprintf("B%dG%d", block, i+1)
		var sec Section
		if losses[g[0]] == 0 {
			sec = GSLSection(name, g, length)
		} else {
			sec = SESection(name, g, length)
		}
		sec.Block, sec.Group = block, i+1
		out = append(out, sec)
	}
	return out
}

// PoolSections : les poules d'un tirage en groupes, nommées poule:A, poule:B…
func PoolSections(groups [][]Ref, length int) []Section {
	var out []Section
	for i, g := range groups {
		sec := PoolSection(PoolSectionName(i), g, length)
		sec.Group = i + 1
		out = append(out, sec)
	}
	return out
}

// BarrageSection : le barrage de la poule pool entre les ex æquo tied, pour les places que la
// poule laisse à départager (PoolTie).
func BarrageSection(pool Section, tied []Ref, qualifiers int) Section {
	_, spots := PoolTie(pool, qualifiers)
	return Section{Name: BarrageSectionName(pool.Name), Kind: SectionBarrage, Players: tied, Spots: spots}
}

// ---- GSL ----

func (v *vue) gslBlockDone() bool {
	for i := range v.Sections {
		if v.Sections[i].Block == v.Round && !v.Sections[i].Done() {
			return false
		}
	}
	return true
}

func (v *vue) gslDone() bool {
	if v.Running > 0 || !v.gslBlockDone() {
		return false
	}
	if v.Rules.Target > 0 && v.livesLeft() <= v.Rules.Target {
		return true
	}
	return len(v.alive()) <= 1
}

// pairGSL : les matchs prêts du bloc courant ; le bloc fini, le tirage du suivant — groupes GSL
// pour les joueurs sans défaite, mini-tableaux pour ceux qui en ont une — ou la finale quand il
// ne reste qu'un joueur de chaque côté.
func (v *vue) pairGSL(seed Seed) []Pairing {
	if v.gslDone() {
		return nil
	}
	if !v.gslBlockDone() {
		var acts []Pairing
		for _, r := range readyFree(v.Sections, refSet(v.Busy)) {
			sec, g := &v.Sections[r.Section], &v.Sections[r.Section].Nodes[r.Node]
			if sec.Block != v.Round {
				continue
			}
			acts = append(acts, Pairing{Kind: PairMatch, Section: sec.Name, Key: g.Key,
				Label: Label{Kind: LabelInBlock, N: sec.Block, Section: sec.Name}.With(g.Label), A: g.Players[0], B: g.Players[1], Length: g.Length})
		}
		return acts
	}
	if v.Running > 0 {
		return nil
	}
	rng := generateur(seed.Draw())
	var g0, g1 []Ref
	for _, p := range v.alive() {
		if v.entrant(p).Losses == 0 {
			g0 = append(g0, p)
		} else {
			g1 = append(g1, p)
		}
	}
	if len(g0) == 1 && len(g1) == 1 {
		return []Pairing{{Kind: PairMatch, Label: Label{Kind: LabelFinal}, A: g0[0], B: g1[0], Length: v.Length}}
	}
	var groups [][]Ref
	for _, g := range [][]Ref{g0, g1} {
		g = sortedRefs(g)
		rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		groups = append(groups, Partition(g)...)
	}
	return []Pairing{{Kind: PairDraw, Label: Label{Kind: LabelDrawBlock, N: v.Round + 1, Players: len(groups)}, Groups: groups}}
}

// ---- Tableaux ----

func (v *vue) pairBracket(seed Seed) []Pairing {
	if !v.Drawn {
		if len(v.Entrants) < 2 {
			return nil
		}
		ids := make([]Ref, len(v.Entrants))
		lives := make(map[Ref]int, len(v.Entrants))
		ratings := make(map[Ref]float64, len(v.Entrants))
		for i, e := range v.Entrants {
			ids[i], lives[e.Player], ratings[e.Player] = e.Player, e.Lives, e.Rating
		}
		slots := DrawSlots(ids, lives, v.Rules.Seeding, ratings, seed.Draw())
		return []Pairing{{Kind: PairDraw,
			Label: Label{Kind: LabelDrawBracket, N: len(slots)}.With(Label{Kind: LabelPhase, Text: v.Name}),
			Slots: slots}}
	}
	var acts []Pairing
	for _, r := range readyFree(v.Sections, refSet(v.Busy)) {
		sec, g := &v.Sections[r.Section], &v.Sections[r.Section].Nodes[r.Node]
		lbl := g.Label
		if sec.Kind == SectionMain && len(v.Sections) > 1 {
			lbl = Label{Kind: LabelMainDraw}.With(g.Label)
		}
		acts = append(acts, Pairing{Kind: PairMatch, Section: sec.Name, Key: g.Key, Label: lbl,
			A: g.Players[0], B: g.Players[1], Length: g.Length})
	}
	return acts
}

// ---- Poules et barrages ----

// PoolWins : les victoires de chaque joueur d'une poule (les forfaits ne comptent pas ; tout
// joueur de la poule figure, même à zéro).
func PoolWins(sec Section) map[Ref]int {
	w := map[Ref]int{}
	for i := range sec.Nodes {
		g := sec.Nodes[i]
		w[g.Src[0].Player] += 0
		w[g.Src[1].Player] += 0
		if g.Done && !g.Walkover {
			w[g.Winner]++
		}
	}
	return w
}

// parVictoires : les joueurs d'une poule par identifiant, puis (stable) par victoires
// décroissantes.
func parVictoires(w map[Ref]int) []Ref {
	ids := make([]Ref, 0, len(w))
	for p := range w {
		ids = append(ids, p)
	}
	ids = sortedRefs(ids)
	sort.SliceStable(ids, func(i, j int) bool { return w[ids[i]] > w[ids[j]] })
	return ids
}

// PoolTie : les joueurs à départager par barrage — ex æquo à la frontière des qualifiés — et le
// nombre de places qui leur reste. Aucun quand la frontière est nette ou la poule trop petite.
func PoolTie(sec Section, qualifiers int) (tied []Ref, spots int) {
	w := PoolWins(sec)
	ids := parVictoires(w)
	q := qualifiers
	if q >= len(ids) {
		return nil, 0
	}
	if w[ids[q-1]] != w[ids[q]] {
		return nil, 0
	}
	val := w[ids[q-1]]
	spots = q
	for _, p := range ids {
		if w[p] > val {
			spots--
		} else if w[p] == val {
			tied = append(tied, p)
		}
	}
	return tied, spots
}

// barrageLosses : défaites de chaque joueur d'un barrage, et ses matchs en cours.
func (v *vue) barrageLosses(name string) (losses map[Ref]int, running int) {
	losses = map[Ref]int{}
	for _, m := range v.Barrages {
		if m.Section != name || m.Status == Cancelled {
			continue
		}
		if m.Status == Running {
			running++
		} else {
			losses[m.loser()]++
		}
	}
	return losses, running
}

// barrageAlive : les joueurs d'un barrage qui ont moins de deux défaites.
func (v *vue) barrageAlive(bs *Section) []Ref {
	losses, _ := v.barrageLosses(bs.Name)
	var out []Ref
	for _, p := range bs.Players {
		if losses[p] < 2 {
			out = append(out, p)
		}
	}
	return out
}

func (v *vue) section(name string) *Section {
	if i := sectionIndex(v.Sections, name); i >= 0 {
		return &v.Sections[i]
	}
	return nil
}

func (v *vue) barragesDone() bool {
	for i := range v.Sections {
		sec := v.Sections[i]
		if sec.Kind != SectionPool {
			continue
		}
		tied, _ := PoolTie(sec, v.Rules.Qualifiers)
		if len(tied) == 0 {
			continue
		}
		bs := v.section(BarrageSectionName(sec.Name))
		if bs == nil || len(v.barrageAlive(bs)) > bs.Spots {
			return false
		}
	}
	return true
}

func (v *vue) pairPools(seed Seed) []Pairing {
	if !v.Drawn {
		n := len(v.Entrants)
		if n < 2 {
			return nil
		}
		rng := generateur(seed.Draw())
		ids := make([]Ref, n)
		for i, e := range v.Entrants {
			ids[i] = e.Player
		}
		ids = sortedRefs(ids)
		rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		ng := (n + v.Rules.GroupSize - 1) / v.Rules.GroupSize
		if ng < 1 {
			ng = 1
		}
		groups := make([][]Ref, ng)
		for i, p := range ids {
			groups[i%ng] = append(groups[i%ng], p)
		}
		return []Pairing{{Kind: PairDraw, Label: Label{Kind: LabelDrawPools, N: ng}, Groups: groups}}
	}
	busy := refSet(v.Busy)
	var acts []Pairing
	for _, r := range readyFree(v.Sections, busy) {
		sec, g := &v.Sections[r.Section], &v.Sections[r.Section].Nodes[r.Node]
		acts = append(acts, Pairing{Kind: PairMatch, Section: sec.Name, Key: g.Key,
			Label: Label{Kind: LabelInSection, Section: sec.Name}.With(g.Label), A: g.Players[0], B: g.Players[1], Length: g.Length})
	}
	ailleurs := refSet(v.Elsewhere)
	for i := range v.Sections {
		sec := v.Sections[i]
		if sec.Kind != SectionPool || !sec.Done() {
			continue
		}
		tied, spots := PoolTie(sec, v.Rules.Qualifiers)
		if len(tied) == 0 {
			continue
		}
		bname := BarrageSectionName(sec.Name)
		bs := v.section(bname)
		if bs == nil {
			acts = append(acts, Pairing{Kind: PairDraw, Section: bname,
				Label: Label{Kind: LabelDrawBarrage, Section: sec.Name, Players: len(tied), Spots: spots}, Groups: [][]Ref{tied}})
			continue
		}
		acts = append(acts, v.pairBarrage(bs, busy, ailleurs, seed)...)
	}
	return acts
}

// pairBarrage : un barrage est un suisse à deux vies qui s'arrête quand il ne reste que Spots
// joueurs ; secours : le match croisé des deux joueurs qui ont le moins de défaites.
func (v *vue) pairBarrage(bs *Section, busy, ailleurs map[Ref]bool, seed Seed) []Pairing {
	losses, running := v.barrageLosses(bs.Name)
	alive := v.barrageAlive(bs)
	budget := len(alive) - running - bs.Spots
	if budget <= 0 {
		return nil
	}
	rng := generateur(seed.Draw())
	var free []Ref
	for _, p := range alive {
		if !busy[p] && !ailleurs[p] {
			free = append(free, p)
		}
	}
	var acts []Pairing
	for l := 0; l < 2 && budget > 0; l++ {
		var g []Ref
		for _, p := range free {
			if losses[p] == l {
				g = append(g, p)
			}
		}
		g = sortedRefs(g)
		rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		for i := 0; i+1 < len(g) && budget > 0; i += 2 {
			acts = append(acts, Pairing{Kind: PairMatch, Section: bs.Name, Label: Label{Kind: LabelBarrage, Section: bs.Name}, A: g[i], B: g[i+1], Length: v.Length})
			budget--
		}
	}
	if len(acts) == 0 && running == 0 && len(free) >= 2 {
		fr := sortedRefs(free)
		sort.SliceStable(fr, func(i, j int) bool { return losses[fr[i]] < losses[fr[j]] })
		acts = append(acts, Pairing{Kind: PairMatch, Section: bs.Name, Label: Label{Kind: LabelBarrageCross, Section: bs.Name}, A: fr[0], B: fr[1], Length: v.Length})
	}
	return acts
}

// PoolQualified : les qualifiés de chaque poule, après barrage, repêchés compris — chaque
// qualifié retiré remplacé par son repêché (en chaîne si le repêché s'est retiré à son tour).
// Un barrage non terminé ne qualifie encore personne.
func PoolQualified(p Phase) []Ref {
	v := voir(&p)
	return v.repecher(v.qualifiedBase())
}

func (v *vue) qualifiedBase() []Ref {
	var out []Ref
	for i := range v.Sections {
		sec := v.Sections[i]
		if sec.Kind != SectionPool {
			continue
		}
		w := PoolWins(sec)
		ids := parVictoires(w)
		tied, spots := PoolTie(sec, v.Rules.Qualifiers)
		if len(tied) == 0 {
			out = append(out, ids[:min(v.Rules.Qualifiers, len(ids))]...)
			continue
		}
		val := w[tied[0]]
		for _, p := range ids {
			if w[p] > val {
				out = append(out, p)
			}
		}
		if bs := v.section(BarrageSectionName(sec.Name)); bs != nil {
			alive := v.barrageAlive(bs)
			if len(alive) <= spots {
				out = append(out, alive...)
			}
		}
	}
	return out
}

func (v *vue) repecher(base []Ref) []Ref {
	if len(v.Repechages) == 0 {
		return base
	}
	seen := map[Ref]bool{}
	out := make([]Ref, 0, len(base))
	for _, p := range base {
		for i := 0; v.withdrawn[p] && i <= len(v.Repechages); i++ {
			r, ok := v.Repechages[p]
			if !ok {
				break
			}
			p = r
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
