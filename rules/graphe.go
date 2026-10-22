package rules

import "fmt"

// ---- Graphes de matchs : tableaux, groupes GSL, poules, consolantes ----
//
// Une Section est une liste de nœuds (Node) dont chaque place vient d'un joueur fixé ou du
// vainqueur/perdant d'un autre nœud, de la même section ou d'une autre (les perdants du
// principal alimentent la consolante). Les constructeurs ne font que poser des sources ;
// Resolve remplit les places à partir des résultats, jusqu'au point fixe ; Ready donne les
// nœuds lançables. Un hôte tient ses sections comme il veut et les passe ici à chaque appel.

// Bye est la référence réservée d'une place vide (exemption).
const Bye Ref = "BYE"

// Src est l'origine d'une place : un joueur, ou le vainqueur/perdant du nœud d'indice From de la
// section Section (la même section si vide). From vaut -1 pour un joueur fixé.
type Src struct {
	Player  Ref    `json:"player,omitempty"`
	From    int    `json:"from"`
	Section string `json:"section,omitempty"`
	Loser   bool   `json:"loser,omitempty"`
}

// Node est un match du graphe. Players, Winner, Loser, Done, Walkover et Skipped sont l'état
// résolu ; Match est l'identifiant opaque du match lancé sur ce nœud (vide : pas lancé).
type Node struct {
	Key      string `json:"key"`
	Label    Label  `json:"label"`
	Length   int    `json:"length"`
	Src      [2]Src `json:"src"`
	Players  [2]Ref `json:"players"`
	Match    string `json:"match,omitempty"`
	Winner   Ref    `json:"winner,omitempty"`
	Loser    Ref    `json:"loser,omitempty"`
	Done     bool   `json:"done,omitempty"`
	Walkover bool   `json:"walkover,omitempty"`  // terminé sans être joué : exemption ou forfait d'un retiré
	Skipped  bool   `json:"skipped,omitempty"`   // nœud conditionnel non joué (recharge)
	Cond     bool   `json:"cond,omitempty"`      // nœud conditionnel
	CondFrom int    `json:"cond_from,omitempty"` // il existe si le vainqueur de CondFrom…
	CondSide int    `json:"cond_side,omitempty"` // …est le joueur CondSide de ce nœud-là
}

// Section est un graphe de matchs. Rounds range les indices de nœuds tour par tour. Un barrage
// n'a pas de nœuds : ses joueurs (Players) s'apparient au fil des résultats, pour Spots places.
type Section struct {
	Name    string  `json:"name"`
	Kind    string  `json:"kind"`
	Group   int     `json:"group,omitempty"`
	Block   int     `json:"block,omitempty"`
	Nodes   []Node  `json:"nodes,omitempty"`
	Rounds  [][]int `json:"rounds,omitempty"`
	Players []Ref   `json:"players,omitempty"`
	Spots   int     `json:"spots,omitempty"`
}

// Done : tous les nœuds de la section sont terminés ou sautés.
func (s Section) Done() bool {
	for i := range s.Nodes {
		if !s.Nodes[i].Done {
			return false
		}
	}
	return true
}

// AllDone : tous les nœuds de toutes les sections sont terminés ou sautés.
func AllDone(secs []Section) bool {
	for i := range secs {
		if !secs[i].Done() {
			return false
		}
	}
	return true
}

func sectionIndex(secs []Section, name string) int {
	for i := range secs {
		if secs[i].Name == name {
			return i
		}
	}
	return -1
}

// Resolve remplit les places de toutes les sections à partir des résultats : propagation des
// vainqueurs et perdants, exemptions (un BYE perd d'office), forfaits des retirés (un nœud non
// lancé contre un retiré est gagné par l'autre), nœuds conditionnels. Les places déjà
// remplies restent ; le résultat est le point fixe. Les sections données ne sont pas modifiées :
// Resolve travaille sur une copie et la renvoie.
func Resolve(secs []Section, withdrawn []Ref) []Section {
	out := cloneSections(secs)
	resolve(out, refSet(withdrawn))
	return out
}

func cloneSections(secs []Section) []Section {
	out := make([]Section, len(secs))
	for i, s := range secs {
		out[i] = s
		out[i].Nodes = append([]Node(nil), s.Nodes...)
	}
	return out
}

func refSet(refs []Ref) map[Ref]bool {
	m := make(map[Ref]bool, len(refs))
	for _, r := range refs {
		m[r] = true
	}
	return m
}

func resolve(secs []Section, withdrawn map[Ref]bool) {
	for changed := true; changed; {
		changed = false
		for si := range secs {
			sec := &secs[si]
			for i := range sec.Nodes {
				g := &sec.Nodes[i]
				if g.Cond {
					from := &sec.Nodes[g.CondFrom]
					if from.Done && !g.Skipped && !g.Done {
						if from.Winner != from.Players[g.CondSide] { // condition non remplie
							g.Skipped, g.Done = true, true
							changed = true
							continue
						}
					}
				}
				for k := 0; k < 2; k++ {
					src := g.Src[k]
					var p Ref
					if src.Player != "" {
						p = src.Player
					} else if src.From >= 0 {
						s := sec
						if src.Section != "" {
							s = nil
							if j := sectionIndex(secs, src.Section); j >= 0 {
								s = &secs[j]
							}
						}
						if s != nil && src.From < len(s.Nodes) && s.Nodes[src.From].Done {
							from := &s.Nodes[src.From]
							if from.Skipped {
								continue
							}
							if src.Loser {
								p = from.Loser
							} else {
								p = from.Winner
							}
						}
					}
					if p != "" && g.Players[k] != p {
						g.Players[k] = p
						changed = true
					}
				}
				if !g.Done && g.Match == "" && g.Players[0] != "" && g.Players[1] != "" &&
					g.Players[0] != Bye && g.Players[1] != Bye && (withdrawn[g.Players[0]] || withdrawn[g.Players[1]]) {
					g.Done, g.Walkover = true, true
					g.Winner, g.Loser = g.Players[0], g.Players[1] // les deux retirés : ordre du tableau
					if withdrawn[g.Players[0]] && !withdrawn[g.Players[1]] {
						g.Winner, g.Loser = g.Players[1], g.Players[0]
					}
					changed = true
					continue
				}
				if !g.Done && g.Players[0] != "" && g.Players[1] != "" {
					if g.Players[0] == Bye && g.Players[1] == Bye {
						g.Done, g.Walkover, g.Winner, g.Loser = true, true, Bye, Bye
						changed = true
					} else if g.Players[0] == Bye || g.Players[1] == Bye {
						g.Done, g.Walkover = true, true
						if g.Players[0] == Bye {
							g.Winner, g.Loser = g.Players[1], Bye
						} else {
							g.Winner, g.Loser = g.Players[0], Bye
						}
						changed = true
					}
				}
			}
		}
	}
}

// Slot désigne un nœud : l'indice de sa section et son indice dans la section.
type Slot struct {
	Section int `json:"section"`
	Node    int `json:"node"`
}

// Ready : les nœuds lançables — deux joueurs connus, aucun BYE, ni terminés ni lancés —, dans
// l'ordre des sections puis des nœuds.
func Ready(secs []Section) []Slot {
	var out []Slot
	for si := range secs {
		for i := range secs[si].Nodes {
			g := &secs[si].Nodes[i]
			if g.Done || g.Match != "" || g.Players[0] == "" || g.Players[1] == "" || g.Players[0] == Bye || g.Players[1] == Bye {
				continue
			}
			out = append(out, Slot{si, i})
		}
	}
	return out
}

// ReadyFree : les nœuds lançables dont aucun joueur n'est occupé (busy), sans qu'un joueur
// figure deux fois dans le lot : le premier nœud qui le prend l'emporte.
func ReadyFree(secs []Section, busy []Ref) []Slot {
	return readyFree(secs, refSet(busy))
}

func readyFree(secs []Section, busy map[Ref]bool) []Slot {
	var out []Slot
	pris := map[Ref]bool{}
	for _, r := range Ready(secs) {
		g := &secs[r.Section].Nodes[r.Node]
		a, b := g.Players[0], g.Players[1]
		if pris[a] || pris[b] || busy[a] || busy[b] {
			continue
		}
		pris[a], pris[b] = true, true
		out = append(out, r)
	}
	return out
}

// ---- Constructeurs ----

// LengthPlan dit la longueur des matchs d'un tableau, tour par tour. Trois sources se
// superposent, de la plus précise à la plus générale : la liste par tour (ByRound, du DERNIER
// tour vers le premier), la longueur de finale, la longueur par défaut.
type LengthPlan struct {
	Default int   `json:"default"`
	Final   int   `json:"final,omitempty"`    // 0 : Default
	ByRound []int `json:"by_round,omitempty"` // plus courte que le tableau : complétée par Default
}

// At : longueur du tour r (0 = premier) d'un tableau de `rounds` tours.
func (lp LengthPlan) At(r, rounds int) int {
	if i := rounds - 1 - r; i >= 0 && i < len(lp.ByRound) && lp.ByRound[i] > 0 {
		return lp.ByRound[i]
	}
	if r == rounds-1 && lp.Final > 0 {
		return lp.Final
	}
	return lp.Default
}

// BracketSection : un tableau à élimination simple de len(slots) places (une puissance de 2 ;
// Bye = place vide), libellés de tour classiques.
func BracketSection(name, kind string, slots []Ref, lp LengthPlan) Section {
	size := len(slots)
	sec := Section{Name: name, Kind: kind}
	rounds := 0
	for n := size; n > 1; n /= 2 {
		rounds++
	}
	var prevIdx []int
	for i := 0; i < size/2; i++ {
		sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, 0, i), Label: RoundLabel(0, rounds),
			Length: lp.At(0, rounds), Src: [2]Src{{Player: slots[2*i], From: -1}, {Player: slots[2*i+1], From: -1}}})
		prevIdx = append(prevIdx, len(sec.Nodes)-1)
	}
	sec.Rounds = append(sec.Rounds, prevIdx)
	for r := 1; r < rounds; r++ {
		var idx []int
		for i := 0; i < len(prevIdx)/2; i++ {
			sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, r, i), Label: RoundLabel(r, rounds),
				Length: lp.At(r, rounds), Src: [2]Src{{From: prevIdx[2*i]}, {From: prevIdx[2*i+1]}}})
			idx = append(idx, len(sec.Nodes)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prevIdx = idx
	}
	return sec
}

// ConsolationSection : la consolante progressive du tableau principal main (2^k places). Ronde
// 0 : perdants du premier tour entre eux ; puis, alternativement, les survivants contre les
// perdants du tour suivant du principal (dans l'ordre inverse, pour éviter les rencontres
// immédiates entre joueurs d'une même moitié) et un tour entre survivants.
func ConsolationSection(name string, main Section, length int) Section {
	sec := Section{Name: name, Kind: SectionConso}
	rounds := len(main.Rounds)
	if rounds < 2 {
		return sec
	}
	var prev []int
	first := main.Rounds[0]
	for i := 0; i+1 < len(first); i += 2 {
		sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, 0, i/2), Label: Label{Kind: LabelConsolationRound, N: 1}, Length: length,
			Src: [2]Src{{From: first[i], Section: main.Name, Loser: true}, {From: first[i+1], Section: main.Name, Loser: true}}})
		prev = append(prev, len(sec.Nodes)-1)
	}
	if len(first) == 1 { // tableau de 2 : pas de consolante possible
		return sec
	}
	sec.Rounds = append(sec.Rounds, prev)
	cr := 1
	for r := 1; r < rounds; r++ {
		drops := main.Rounds[r]
		var idx []int
		for i := 0; i < len(prev); i++ {
			d := drops[len(drops)-1-i]
			sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, cr, i), Label: Label{Kind: LabelConsolationRound, N: cr + 1}, Length: length,
				Src: [2]Src{{From: prev[i]}, {From: d, Section: main.Name, Loser: true}}})
			idx = append(idx, len(sec.Nodes)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prev = idx
		cr++
		if len(prev) == 1 {
			break
		}
		idx = nil
		for i := 0; i+1 < len(prev); i += 2 {
			sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, cr, i/2), Label: Label{Kind: LabelConsolationRound, N: cr + 1}, Length: length,
				Src: [2]Src{{From: prev[i]}, {From: prev[i+1]}}})
			idx = append(idx, len(sec.Nodes)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prev = idx
		cr++
	}
	if n := len(sec.Nodes); n > 0 {
		sec.Nodes[n-1].Label = Label{Kind: LabelConsolationFinal}
	}
	return sec
}

// LastChanceSection : un tableau à élimination simple dont les places du premier tour sont des
// sources (perdants d'autres nœuds), en première position de chaque paire d'abord, complété par
// des BYE.
func LastChanceSection(name string, srcs []Src, length int) Section {
	size := pow2ceil(len(srcs))
	if size < 2 {
		size = 2
	}
	slots := make([]Ref, size)
	for i := range slots {
		slots[i] = Bye
	}
	sec := BracketSection(name, name, slots, LengthPlan{Default: length})
	k := 0
	for _, mi := range sec.Rounds[0] {
		if k < len(srcs) {
			sec.Nodes[mi].Src[0] = srcs[k]
			k++
		}
	}
	for _, mi := range sec.Rounds[0] {
		if k < len(srcs) {
			sec.Nodes[mi].Src[1] = srcs[k]
			k++
		}
	}
	for i := range sec.Nodes {
		if i < len(sec.Rounds[0]) {
			sec.Nodes[i].Label = Label{Kind: LabelLastChance}
		} else {
			sec.Nodes[i].Label = Label{Kind: LabelLastChance}.With(sec.Nodes[i].Label)
		}
	}
	return sec
}

// GSLSection : un groupe GSL de 4 (5 matchs), 3 (3 matchs) ou 2 joueurs (1 match). Sorties :
// le vainqueur du match des gagnants (aucune défaite) et celui du décisif (une défaite).
func GSLSection(name string, players []Ref, length int) Section {
	sec := Section{Name: name, Kind: SectionGSL}
	k := func(i int) string { return fmt.Sprintf("%s.%d", name, i) }
	switch len(players) {
	case 4:
		a, b, c, d := players[0], players[1], players[2], players[3]
		sec.Nodes = []Node{
			{Key: k(0), Label: Label{Kind: LabelOpening}, Length: length, Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}},
			{Key: k(1), Label: Label{Kind: LabelOpening}, Length: length, Src: [2]Src{{Player: c, From: -1}, {Player: d, From: -1}}},
			{Key: k(2), Label: Label{Kind: LabelWinnersMatch}, Length: length, Src: [2]Src{{From: 0}, {From: 1}}},
			{Key: k(3), Label: Label{Kind: LabelLosersMatch}, Length: length, Src: [2]Src{{From: 0, Loser: true}, {From: 1, Loser: true}}},
			{Key: k(4), Label: Label{Kind: LabelDecider}, Length: length, Src: [2]Src{{From: 2, Loser: true}, {From: 3}}},
		}
		sec.Rounds = [][]int{{0, 1}, {2, 3}, {4}}
	case 3:
		a, b, c := players[0], players[1], players[2]
		sec.Nodes = []Node{
			{Key: k(0), Label: Label{Kind: LabelOpening}, Length: length, Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}},
			{Key: k(1), Label: Label{Kind: LabelWinnersMatch}, Length: length, Src: [2]Src{{From: 0}, {Player: c, From: -1}}},
			{Key: k(2), Label: Label{Kind: LabelDecider}, Length: length, Src: [2]Src{{From: 0, Loser: true}, {From: 1, Loser: true}}},
		}
		sec.Rounds = [][]int{{0}, {1}, {2}}
	case 2:
		sec.Nodes = []Node{{Key: k(0), Label: Label{Kind: LabelSingleMatch}, Length: length, Src: [2]Src{{Player: players[0], From: -1}, {Player: players[1], From: -1}}}}
		sec.Rounds = [][]int{{0}}
	}
	return sec
}

// SESection : un mini-tableau à élimination simple pour un groupe de 2 à 4 joueurs à une vie
// (bloc GSL, groupe d'une défaite).
func SESection(name string, players []Ref, length int) Section {
	slots := make([]Ref, 4)
	for i := range slots {
		slots[i] = Bye
	}
	switch len(players) {
	case 4:
		copy(slots, players)
	case 3:
		slots[0], slots[1], slots[2] = players[0], players[1], players[2]
	case 2:
		slots = []Ref{players[0], players[1]}
	default:
		return Section{Name: name, Kind: SectionSE}
	}
	sec := BracketSection(name, SectionSE, slots, LengthPlan{Default: length})
	for i := range sec.Nodes {
		sec.Nodes[i].Label = Label{Kind: LabelSingleElim}
	}
	return sec
}

// PoolSection : une poule toutes rondes (table de Berger) ; un nombre impair de joueurs donne
// une exemption par ronde, sans nœud.
func PoolSection(name string, players []Ref, length int) Section {
	sec := Section{Name: name, Kind: SectionPool}
	n := len(players)
	ids := append([]Ref{}, players...)
	if n%2 == 1 {
		ids = append(ids, Bye)
		n++
	}
	for r := 0; r < n-1; r++ {
		var idx []int
		for i := 0; i < n/2; i++ {
			a, b := ids[i], ids[n-1-i]
			if a == Bye || b == Bye {
				continue
			}
			sec.Nodes = append(sec.Nodes, Node{Key: fmt.Sprintf("%s.%d.%d", name, r, i), Label: Label{Kind: LabelPoolRound, N: r + 1}, Length: length,
				Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}})
			idx = append(idx, len(sec.Nodes)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		ids = append([]Ref{ids[0], ids[n-1]}, ids[1:n-1]...) // rotation, le premier reste fixe
	}
	return sec
}

func pow2ceil(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}
