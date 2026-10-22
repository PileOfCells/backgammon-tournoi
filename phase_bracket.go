package tournoi

import (
	"fmt"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// ---- Tableaux : élimination simple, tableau à vies (exemptions), consolante, dernière chance,
// double élimination (réconciliation, recharge) ----
//
// Le tirage (rules.DrawSlots, têtes de série comprises), la construction des sections
// (rules.BracketSections), les appariements (rules.Pair) et le classement (rules.Rank,
// rules.RankSection) sont dans rules ; ici, l'enveloppe.

// buildBracket crée les sections d'un tableau à partir des places tirées.
func (s *State) buildBracket(ph *PhaseState, slots []PlayerID) {
	ph.Sections = fromRulesSections(rules.BracketSections(refs(slots), ph.Length, phaseRules(ph.Cfg)))
	ph.Drawn = true
	ph.resolve(s.Withdrawn)
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
	return s.actions(ph, rules.Pair(s.snapshot(ph, nil), s.seed()))
}

// bracketSurvivors : vainqueur final si le tableau est terminé, sinon joueurs encore en course.
func (s *State) bracketSurvivors(ph *PhaseState) []PlayerID {
	if !ph.Drawn {
		return ph.Entrants
	}
	snap := s.bilan(ph)
	r := rules.Rank(snap)
	if rules.AllDone(snap.Sections) && len(r) > 0 {
		return []PlayerID{PlayerID(r[0].Player)}
	}
	var out []PlayerID
	for _, rk := range r {
		if rk.Note.Kind == rules.NoteRunning {
			out = append(out, PlayerID(rk.Player))
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
// Enveloppe de rules.RankSection. La section est cherchée de la dernière phase vers la
// première : c'est la plus récente qui porte ce nom qui compte, un tournoi pouvant enchaîner
// deux tableaux.
func (s *State) SectionRanking(name string) []Rank {
	for i := len(s.Phases) - 1; i >= 0; i-- {
		if sec := s.Phases[i].section(name); sec != nil {
			return fromStandings(rules.RankSection(toRulesSection(sec), withdrawnRefs(s.Withdrawn)))
		}
	}
	return nil
}
