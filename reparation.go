package tournoi

import "sort"

// La réparation d'un graphe désaccordé par une correction.
//
// Corriger un résultat de tableau après coup recalcule l'état et lève un avertissement (« match
// joué par le mauvais joueur »), mais le directeur se retrouvait devant un arbre faux, avec les
// matchs suivants déjà joués par les mauvaises personnes, et rien pour l'aider. Le moteur
// PROPOSE désormais la réparation ; il ne l'applique pas.
//
// Rien n'est appliqué d'office, et c'est la même règle que partout ailleurs : le moteur propose,
// le TD décide. Ne rien confirmer laisse l'état tel quel, avertissement compris — un directeur
// peut très bien préférer laisser le tableau tel qu'il a été joué et le noter à la main.
//
// La réparation est une ANNULATION EN SÉRIE : le match joué par les mauvais joueurs, puis tout
// ce qui en descend. Un demi-finaliste faux fait une finale fausse, même si la finale, elle,
// oppose bien les deux joueurs que le graphe attendait — elle les attendait pour de mauvaises
// raisons. Les annulations sont proposées du plus profond au moins profond : on défait le plus
// récent d'abord, comme on le ferait sur le papier.
//
// Les relances correctes ne sont pas fabriquées ici : une fois les annulations confirmées, les
// places redeviennent libres et le moteur les propose comme n'importe quel match prêt. C'est
// pourquoi elles arrivent à l'appel SUIVANT — et c'est bien ainsi : tant que le TD n'a pas
// confirmé, il n'y a rien à relancer.

// place repère un match de graphe : sa section et son rang dans celle-ci.
type place struct{ sec, i int }

// proposeRepair renvoie les annulations à confirmer pour remettre les graphes d'accord avec les
// résultats. Vide dans un tournoi mené normalement.
func (s *State) proposeRepair(ph *PhaseState) []Action {
	if len(ph.Sections) == 0 {
		return nil
	}
	// 1. Les places dont le match n'a pas été joué par les joueurs attendus.
	//
	// Cette passe tourne à CHAQUE proposition et ne trouve rien la quasi-totalité du temps :
	// elle n'alloue donc rien tant qu'il n'y a rien à réparer. Une carte allouée ici coûtait
	// plus cher que tout le reste du moteur réuni.
	var touchée map[place]bool
	for si, sec := range ph.Sections {
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.MatchID == "" || g.Players[0] == "" || g.Players[1] == "" {
				continue
			}
			m := s.matchOf(g)
			if m == nil {
				continue
			}
			if (g.Players[0] == m.A && g.Players[1] == m.B) || (g.Players[0] == m.B && g.Players[1] == m.A) {
				continue
			}
			if touchée == nil {
				touchée = map[place]bool{}
			}
			touchée[place{si, i}] = true
		}
	}
	if len(touchée) == 0 {
		return nil
	}
	index := map[string]int{}
	for i, sec := range ph.Sections {
		index[sec.Name] = i
	}
	// 2. Tout ce qui en descend l'est aussi : un vainqueur faux fait un match suivant faux.
	//    Point fixe, comme resolve : les sources peuvent traverser les sections (les perdants du
	//    principal alimentent la consolante).
	for changé := true; changé; {
		changé = false
		for si, sec := range ph.Sections {
			for i := range sec.Matches {
				p := place{si, i}
				if touchée[p] {
					continue
				}
				for _, src := range sec.Matches[i].Src {
					if src.Player != "" || src.From < 0 {
						continue
					}
					amont := place{si, src.From}
					if src.Section != "" {
						j, ok := index[src.Section]
						if !ok {
							continue
						}
						amont = place{j, src.From}
					}
					if touchée[amont] {
						touchée[p] = true
						changé = true
						break
					}
				}
			}
		}
	}
	// 3. Les annulations, du plus profond au moins profond.
	var places []place
	for p := range touchée {
		places = append(places, p)
	}
	sort.Slice(places, func(a, b int) bool {
		if places[a].sec != places[b].sec {
			return places[a].sec > places[b].sec
		}
		return places[a].i > places[b].i
	})
	var acts []Action
	for _, p := range places {
		sec := ph.Sections[p.sec]
		g := &sec.Matches[p.i]
		m := s.matchOf(g)
		if m == nil {
			continue
		}
		acts = append(acts, Action{Kind: ActCancelMatch, Phase: ph.Index, Section: sec.Name,
			Key: g.Key, Label: g.Label, Match: m.ID, A: m.A, B: m.B})
	}
	return acts
}

// matchOf : le match joué à cette place, s'il en existe un qui compte encore.
func (s *State) matchOf(g *GMatch) *Match {
	if g.MatchID == "" {
		return nil
	}
	m := s.Matches[g.MatchID]
	if m == nil || m.Status == Cancelled {
		return nil
	}
	return m
}
