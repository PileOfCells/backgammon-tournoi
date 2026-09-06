package tournoi

import "fmt"

// gmatch retrouve le match de graphe (section, clé).
func (ph *PhaseState) gmatch(section, key string) *GMatch {
	if key == "" {
		return nil
	}
	for _, sec := range ph.Sections {
		if sec.Name != section {
			continue
		}
		for i := range sec.Matches {
			if sec.Matches[i].Key == key {
				return &sec.Matches[i]
			}
		}
	}
	return nil
}

func (ph *PhaseState) section(name string) *Section {
	for _, sec := range ph.Sections {
		if sec.Name == name {
			return sec
		}
	}
	return nil
}

// resolve remplit les places de tous les graphes à partir des résultats, propage les exemptions
// (walkover) et les matchs conditionnels. Idempotent.
func (ph *PhaseState) resolve() {
	for changed := true; changed; {
		changed = false
		for _, sec := range ph.Sections {
			for i := range sec.Matches {
				g := &sec.Matches[i]
				if g.Cond {
					from := &sec.Matches[g.CondFrom]
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
					var p PlayerID
					if src.Player != "" {
						p = src.Player
					} else if src.From >= 0 {
						s := sec
						if src.Section != "" {
							s = ph.section(src.Section)
						}
						if s != nil && src.From < len(s.Matches) && s.Matches[src.From].Done {
							from := &s.Matches[src.From]
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
				if !g.Done && g.Players[0] != "" && g.Players[1] != "" {
					if g.Players[0] == BYE && g.Players[1] == BYE {
						g.Done, g.Walkover, g.Winner, g.Loser = true, true, BYE, BYE
						changed = true
					} else if g.Players[0] == BYE || g.Players[1] == BYE {
						g.Done, g.Walkover = true, true
						if g.Players[0] == BYE {
							g.Winner, g.Loser = g.Players[1], BYE
						} else {
							g.Winner, g.Loser = g.Players[0], BYE
						}
						changed = true
					}
				}
			}
		}
	}
}

// ready renvoie les matchs de graphe prêts à être lancés (places connues, pas encore lancés).
func (ph *PhaseState) ready() []readyMatch {
	var out []readyMatch
	for _, sec := range ph.Sections {
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.Done || g.MatchID != "" || g.Players[0] == "" || g.Players[1] == "" || g.Players[0] == BYE || g.Players[1] == BYE {
				continue
			}
			out = append(out, readyMatch{sec, g})
		}
	}
	return out
}

// readyFree : matchs prêts dont aucun joueur n'est occupé, sans doublon de joueur dans le lot.
func (s *State) readyFree(ph *PhaseState) []readyMatch {
	var out []readyMatch
	pris := map[PlayerID]bool{}
	for _, r := range ph.ready() {
		a, b := r.g.Players[0], r.g.Players[1]
		if pris[a] || pris[b] || s.busy(a) || s.busy(b) {
			continue
		}
		pris[a], pris[b] = true, true
		out = append(out, r)
	}
	return out
}

type readyMatch struct {
	sec *Section
	g   *GMatch
}

// allDone : tous les matchs de toutes les sections sont terminés (ou sautés).
func (ph *PhaseState) allDone() bool {
	for _, sec := range ph.Sections {
		for i := range sec.Matches {
			if !sec.Matches[i].Done {
				return false
			}
		}
	}
	return true
}

// ---- constructeurs de graphes ----

// bracketSection construit un tableau à élimination simple de 2^k places à partir des slots
// (BYE = place vide). Les labels de tour sont classiques.
func bracketSection(name, kind string, slots []PlayerID, length, finalLength int) *Section {
	size := len(slots)
	sec := &Section{Name: name, Kind: kind}
	rounds := 0
	for n := size; n > 1; n /= 2 {
		rounds++
	}
	// tour 0
	var prevIdx []int
	for i := 0; i < size/2; i++ {
		sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, 0, i), Label: roundLabel(0, rounds),
			Length: length, Src: [2]Src{{Player: slots[2*i], From: -1}, {Player: slots[2*i+1], From: -1}}})
		prevIdx = append(prevIdx, len(sec.Matches)-1)
	}
	sec.Rounds = append(sec.Rounds, prevIdx)
	for r := 1; r < rounds; r++ {
		var idx []int
		for i := 0; i < len(prevIdx)/2; i++ {
			l := length
			if r == rounds-1 && finalLength > 0 {
				l = finalLength
			}
			sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, r, i), Label: roundLabel(r, rounds),
				Length: l, Src: [2]Src{{From: prevIdx[2*i]}, {From: prevIdx[2*i+1]}}})
			idx = append(idx, len(sec.Matches)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prevIdx = idx
	}
	return sec
}

func roundLabel(r, rounds int) string {
	switch rounds - r {
	case 1:
		return "Finale"
	case 2:
		return "Demi-finale"
	case 3:
		return "Quart de finale"
	}
	return fmt.Sprintf("Tour %d", r+1)
}

// consolationSection construit la consolante progressive d'un tableau principal `main` de 2^k
// places : ronde 0 = perdants du tour 1 entre eux ; ronde 2r-2 = survivants contre perdants du
// tour r+1 du principal ; ronde 2r-1 = entre survivants. Renvoie la section (vainqueur = dernier match).
func consolationSection(name string, main *Section, length int) *Section {
	sec := &Section{Name: name, Kind: "conso"}
	rounds := len(main.Rounds)
	if rounds < 2 {
		return sec
	}
	// ronde 0 : perdants du tour 0 du principal, appariés deux à deux
	var prev []int
	first := main.Rounds[0]
	for i := 0; i+1 < len(first); i += 2 {
		sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, 0, i/2), Label: "Consolante tour 1", Length: length,
			Src: [2]Src{{From: first[i], Section: main.Name, Loser: true}, {From: first[i+1], Section: main.Name, Loser: true}}})
		prev = append(prev, len(sec.Matches)-1)
	}
	if len(first) == 1 { // tableau de 2 : pas de consolante possible
		return sec
	}
	sec.Rounds = append(sec.Rounds, prev)
	cr := 1
	for r := 1; r < rounds; r++ {
		// fusion : survivants contre perdants du tour r du principal (ordre inversé pour éviter les
		// rencontres immédiates entre joueurs venant de la même moitié)
		drops := main.Rounds[r]
		var idx []int
		for i := 0; i < len(prev); i++ {
			d := drops[len(drops)-1-i]
			sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, cr, i), Label: fmt.Sprintf("Consolante tour %d", cr+1), Length: length,
				Src: [2]Src{{From: prev[i]}, {From: d, Section: main.Name, Loser: true}}})
			idx = append(idx, len(sec.Matches)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prev = idx
		cr++
		if len(prev) == 1 {
			break
		}
		// tour interne
		idx = nil
		for i := 0; i+1 < len(prev); i += 2 {
			sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, cr, i/2), Label: fmt.Sprintf("Consolante tour %d", cr+1), Length: length,
				Src: [2]Src{{From: prev[i]}, {From: prev[i+1]}}})
			idx = append(idx, len(sec.Matches)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		prev = idx
		cr++
	}
	if n := len(sec.Matches); n > 0 {
		sec.Matches[n-1].Label = "Finale consolante"
	}
	return sec
}

// gslSection construit un groupe GSL de 4 (5 matchs), 3 (3 matchs) ou 2 (1 match) joueurs.
// Sorties : vainqueur du match des gagnants (0 défaite), vainqueur du décisif (1 défaite).
func gslSection(name string, players []PlayerID, length int) *Section {
	sec := &Section{Name: name, Kind: "gsl"}
	k := func(i int) string { return fmt.Sprintf("%s.%d", name, i) }
	switch len(players) {
	case 4:
		a, b, c, d := players[0], players[1], players[2], players[3]
		sec.Matches = []GMatch{
			{Key: k(0), Label: "Ouverture", Length: length, Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}},
			{Key: k(1), Label: "Ouverture", Length: length, Src: [2]Src{{Player: c, From: -1}, {Player: d, From: -1}}},
			{Key: k(2), Label: "Match des gagnants", Length: length, Src: [2]Src{{From: 0}, {From: 1}}},
			{Key: k(3), Label: "Match des perdants", Length: length, Src: [2]Src{{From: 0, Loser: true}, {From: 1, Loser: true}}},
			{Key: k(4), Label: "Match décisif", Length: length, Src: [2]Src{{From: 2, Loser: true}, {From: 3}}},
		}
		sec.Rounds = [][]int{{0, 1}, {2, 3}, {4}}
	case 3:
		a, b, c := players[0], players[1], players[2]
		sec.Matches = []GMatch{
			{Key: k(0), Label: "Ouverture", Length: length, Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}},
			{Key: k(1), Label: "Match des gagnants", Length: length, Src: [2]Src{{From: 0}, {Player: c, From: -1}}},
			{Key: k(2), Label: "Match décisif", Length: length, Src: [2]Src{{From: 0, Loser: true}, {From: 1, Loser: true}}},
		}
		sec.Rounds = [][]int{{0}, {1}, {2}}
	case 2:
		sec.Matches = []GMatch{{Key: k(0), Label: "Match", Length: length, Src: [2]Src{{Player: players[0], From: -1}, {Player: players[1], From: -1}}}}
		sec.Rounds = [][]int{{0}}
	}
	return sec
}

// seSection : mini-tableau à élimination simple pour un groupe de 2 à 4 joueurs à une vie.
func seSection(name string, players []PlayerID, length int) *Section {
	slots := make([]PlayerID, 4)
	for i := range slots {
		slots[i] = BYE
	}
	switch len(players) {
	case 4:
		copy(slots, players)
	case 3:
		slots[0], slots[1], slots[2] = players[0], players[1], players[2]
	case 2:
		slots = []PlayerID{players[0], players[1]}
	default:
		return &Section{Name: name, Kind: "se"}
	}
	sec := bracketSection(name, "se", slots, length, 0)
	for i := range sec.Matches {
		sec.Matches[i].Label = "Élimination directe"
	}
	return sec
}

// rrSection : poule toutes rondes (table de Berger) pour n joueurs.
func rrSection(name string, players []PlayerID, length int) *Section {
	sec := &Section{Name: name, Kind: "poule"}
	n := len(players)
	ids := append([]PlayerID{}, players...)
	if n%2 == 1 {
		ids = append(ids, BYE)
		n++
	}
	for r := 0; r < n-1; r++ {
		var idx []int
		for i := 0; i < n/2; i++ {
			a, b := ids[i], ids[n-1-i]
			if a == BYE || b == BYE {
				continue
			}
			sec.Matches = append(sec.Matches, GMatch{Key: fmt.Sprintf("%s.%d.%d", name, r, i), Label: fmt.Sprintf("Poule, ronde %d", r+1), Length: length,
				Src: [2]Src{{Player: a, From: -1}, {Player: b, From: -1}}})
			idx = append(idx, len(sec.Matches)-1)
		}
		sec.Rounds = append(sec.Rounds, idx)
		// rotation (le premier reste fixe)
		ids = append([]PlayerID{ids[0], ids[n-1]}, ids[1:n-1]...)
	}
	return sec
}
