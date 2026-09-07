package tournoi

import "fmt"

// ---- Blocs GSL : suisse à 2 vies dont l'appariement est figé par blocs de petits groupes ----

// partition découpe une liste (déjà mélangée) en groupes de 4, 3, 2 ou 1.
func partition(g []PlayerID) [][]PlayerID {
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
	var out [][]PlayerID
	k := 0
	for _, sz := range sizes {
		out = append(out, g[k:k+sz])
		k += sz
	}
	return out
}

func (s *State) gslBlockDone(ph *PhaseState) bool {
	for _, sec := range ph.Sections {
		if sec.Block != ph.Round {
			continue
		}
		for i := range sec.Matches {
			if !sec.Matches[i].Done {
				return false
			}
		}
	}
	return true
}

func (s *State) gslDone(ph *PhaseState) bool {
	if s.runningInPhase(ph) > 0 || !s.gslBlockDone(ph) {
		return false
	}
	if ph.Cfg.Target > 0 && s.sumLives(ph) <= ph.Cfg.Target {
		return true
	}
	return len(s.alive(ph)) <= 1
}

func (s *State) proposeGSL(ph *PhaseState) []Action {
	if s.gslDone(ph) {
		return nil
	}
	if !s.gslBlockDone(ph) {
		var acts []Action
		for _, r := range s.readyFree(ph) {
			if r.sec.Block != ph.Round {
				continue
			}
			acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Section: r.sec.Name, Key: r.g.Key,
				Label: Label{Kind: LabelInBlock, N: r.sec.Block, Section: r.sec.Name}.with(r.g.Label), A: r.g.Players[0], B: r.g.Players[1], Length: r.g.Length})
		}
		return acts
	}
	if s.runningInPhase(ph) > 0 { // finale ou matchs hors bloc en cours
		return nil
	}
	// nouveau bloc : groupes GSL pour les 0 défaite, mini-tableaux pour les 1 défaite
	rng := s.rng()
	alive := s.alive(ph)
	var g0, g1 []PlayerID
	for _, p := range alive {
		if ph.Losses[p] == 0 {
			g0 = append(g0, p)
		} else {
			g1 = append(g1, p)
		}
	}
	if len(g0) == 1 && len(g1) == 1 { // finale (recharge implicite : si le 1 défaite gagne, ils rejouent)
		return []Action{{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelFinal}, A: g0[0], B: g1[0], Length: ph.Length}}
	}
	var groups [][]PlayerID
	for _, g := range [][]PlayerID{g0, g1} {
		g = sortedIDs(g)
		rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		groups = append(groups, partition(g)...)
	}
	return []Action{{Kind: ActDraw, Phase: ph.Index, Label: Label{Kind: LabelDrawBlock, N: ph.Round + 1, Players: len(groups)},
		Draw: &Draw{Groups: groups, Lives: copyLives(ph)}}}
}

func (s *State) applyGSLDraw(ph *PhaseState, d *Draw) error {
	ph.Round++
	ph.Drawn = true
	for i, g := range d.Groups {
		if len(g) < 2 {
			continue
		}
		name := fmt.Sprintf("B%dG%d", ph.Round, i+1)
		var sec *Section
		if ph.Losses[g[0]] == 0 {
			sec = gslSection(name, g, ph.Length)
		} else {
			sec = seSection(name, g, ph.Length)
		}
		sec.Block, sec.Group = ph.Round, i+1
		ph.Sections = append(ph.Sections, sec)
	}
	ph.resolve(s.Withdrawn)
	return nil
}
