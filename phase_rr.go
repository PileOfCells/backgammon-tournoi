package tournoi

import (
	"fmt"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// ---- Poules toutes rondes, qualification sans départage (barrage à 2 vies entre ex æquo) ----
//
// Tirage, poules (table de Berger), égalités, barrages, qualifiés et classement sont dans rules
// (Pair, PoolSections, PoolTie, PoolQualified, Rank) ; ici, l'enveloppe.

// groupName : identifiant de la i-ième poule (voir codes.go, « Noms de section »).
func groupName(i int) string { return poolSectionName(i) }

func (s *State) proposeRR(ph *PhaseState) []Action {
	return s.actions(ph, rules.Pair(s.snapshot(ph, nil), s.seed()))
}

// rrWins : victoires de chaque joueur dans une section (rules.PoolWins).
func rrWins(sec *Section) map[PlayerID]int {
	w := map[PlayerID]int{}
	for p, n := range rules.PoolWins(toRulesSection(sec)) {
		w[PlayerID(p)] = n
	}
	return w
}

func (s *State) applyRRDraw(ph *PhaseState, section string, d *Draw) error {
	if section == "" {
		ph.Sections = append(ph.Sections, fromRulesSections(rules.PoolSections(groupIDsToRefs(d.Groups), ph.Length))...)
		ph.Drawn = true
		ph.resolve(s.Withdrawn)
		return nil
	}
	if len(d.Groups) != 1 {
		return fmt.Errorf("barrage : un seul groupe attendu")
	}
	// places : recalculées à partir de la poule
	poule := ph.section(poolOfBarrage(section))
	if poule == nil {
		return fmt.Errorf("barrage : poule %q inconnue", section)
	}
	b := rules.BarrageSection(toRulesSection(poule), refs(d.Groups[0]), ph.Cfg.Qualifiers)
	b.Name = section
	ph.Sections = append(ph.Sections, fromRulesSection(b))
	return nil
}

func groupIDsToRefs(gs [][]PlayerID) [][]rules.Ref {
	out := make([][]rules.Ref, len(gs))
	for i, g := range gs {
		out[i] = refs(g)
	}
	return out
}

// rrQualified : qualifiés de chaque poule (après barrages), repêchés compris (rules.PoolQualified).
func (s *State) rrQualified(ph *PhaseState) []PlayerID {
	return ids(rules.PoolQualified(s.bilan(ph)))
}
