package tournoi

import "github.com/PileOfCells/backgammon-tournoi/rules"

// ---- Blocs GSL : suisse à 2 vies dont l'appariement est figé par blocs de petits groupes ----
//
// Les règles — groupes GSL pour les joueurs sans défaite, mini-tableaux pour ceux qui en ont une,
// finale entre les deux derniers — sont dans rules (Pair, GSLBlockSections) ; ici, l'enveloppe.

func (s *State) gslDone(ph *PhaseState) bool { return rules.PhaseDone(s.bilan(ph)) }

func (s *State) proposeGSL(ph *PhaseState) []Action {
	return s.actions(ph, rules.Pair(s.snapshot(ph, nil), s.seed()))
}

func (s *State) applyGSLDraw(ph *PhaseState, d *Draw) error {
	ph.Round++
	ph.Drawn = true
	losses := make(map[rules.Ref]int, len(ph.Losses))
	for p, n := range ph.Losses {
		losses[ref(p)] = n
	}
	groups := make([][]rules.Ref, len(d.Groups))
	for i, g := range d.Groups {
		groups[i] = refs(g)
	}
	ph.Sections = append(ph.Sections, fromRulesSections(rules.GSLBlockSections(ph.Round, groups, losses, ph.Length))...)
	ph.resolve(s.Withdrawn)
	return nil
}
