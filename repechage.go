package tournoi

import (
	"fmt"
	"sort"
)

// ---- Repêchage d'un qualifié de poule retiré avant le tirage (règle N26) ----
//
// Un qualifié de poule qui se retire avant que la phase suivante soit tirée laisse une place.
// Le moteur PROPOSE de la donner au suivant de sa poule (ActRepechage) ; le TD confirme
// (EvRepechage) ou passe outre, et la place devient alors une exemption du tableau, comme
// avant que la règle soit tranchée. Rien n'est fait d'office : un journal sans repechage se
// rejoue exactement comme il a été joué.
//
// Le suivant est le joueur de la poule ni qualifié ni retiré qui a le plus de victoires de
// poule. Le moteur ne départage pas : entre ex æquo, une proposition par candidat, et le TD
// choisit. Après le tirage (ou le premier match) de la phase suivante, plus de repêchage : le
// retiré perd son match par forfait, comme tout retrait de tableau.

// repechageQualified remplace, dans la liste des qualifiés d'une poule, chaque retiré repêché
// par son remplaçant — en chaîne, si le remplaçant s'est retiré à son tour et a été remplacé.
// Un qualifié réintégré (plus retiré) retrouve sa place : le repêchage ne tient qu'autant que
// le retrait.
func (s *State) repechageQualified(ph *PhaseState, base []PlayerID) []PlayerID {
	if len(ph.Repechages) == 0 {
		return base
	}
	seen := map[PlayerID]bool{}
	out := make([]PlayerID, 0, len(base))
	for _, p := range base {
		for i := 0; s.Withdrawn[p] && i <= len(ph.Repechages); i++ {
			r, ok := ph.Repechages[p]
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

// repechageOpen : la phase de poules ph peut encore repêcher — terminée, suivie d'une phase qui
// sélectionne ses entrants (pas « all ») et qui n'est ni tirée ni commencée.
func (s *State) repechageOpen(ph *PhaseState) error {
	if ph.Cfg.Kind != KindRoundRobin {
		return fmt.Errorf("repêchage : la phase %d n'est pas une phase de poules", ph.Index)
	}
	if ph.Index+1 >= len(s.Config.Phases) {
		return fmt.Errorf("repêchage : pas de phase après les poules")
	}
	if s.Config.Phases[ph.Index+1].Entry == "all" {
		return fmt.Errorf("repêchage : la phase suivante prend tous les joueurs")
	}
	if !s.phaseDone(ph) {
		return fmt.Errorf("repêchage : les poules ne sont pas terminées")
	}
	if next := s.phaseOf(ph.Index + 1); next != nil && (next.Drawn || next.Started) {
		return fmt.Errorf("repêchage : la phase suivante est déjà tirée ; le retiré perd son match par forfait")
	}
	return nil
}

// poolMembers : les joueurs d'une poule, dans l'ordre des identifiants.
func poolMembers(sec *Section) []PlayerID {
	var out []PlayerID
	for p := range rrWins(sec) {
		out = append(out, p)
	}
	return sortedIDs(out)
}

// repechageCandidates : les suivants de la poule sec — non qualifiés, non retirés, au plus
// grand nombre de victoires de poule. Plusieurs quand ils sont ex æquo ; aucun quand la poule
// est épuisée.
func (s *State) repechageCandidates(sec *Section, qualified map[PlayerID]bool) []PlayerID {
	w := rrWins(sec)
	var cands []PlayerID
	for _, p := range poolMembers(sec) {
		if !qualified[p] && !s.Withdrawn[p] {
			cands = append(cands, p)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.SliceStable(cands, func(i, j int) bool { return w[cands[i]] > w[cands[j]] })
	best := w[cands[0]]
	n := 0
	for n < len(cands) && w[cands[n]] == best {
		n++
	}
	return cands[:n]
}

// proposeRepechages : une ActRepechage par candidat, pour le premier qualifié retiré non
// remplacé de chaque poule. Un seul retiré à la fois par poule : les candidats du suivant
// dépendent du choix fait pour le premier.
func (s *State) proposeRepechages(ph *PhaseState) []Action {
	if ph == nil || s.repechageOpen(ph) != nil {
		return nil
	}
	qualified := map[PlayerID]bool{}
	for _, p := range s.rrQualified(ph) {
		qualified[p] = true
	}
	var acts []Action
	for _, sec := range ph.Sections {
		if sec.Kind != secKindPool {
			continue
		}
		var gone PlayerID
		for _, p := range poolMembers(sec) {
			if qualified[p] && s.Withdrawn[p] {
				gone = p
				break
			}
		}
		if gone == "" {
			continue
		}
		cands := s.repechageCandidates(sec, qualified)
		for _, c := range cands {
			acts = append(acts, Action{Kind: ActRepechage, Phase: ph.Index, Section: sec.Name, A: gone, B: c,
				Label: Label{Kind: LabelRepechage, Section: sec.Name, Players: len(cands)}})
		}
	}
	return acts
}

// applyRepechage applique un EvRepechage : B prend la place de A parmi les qualifiés de la
// poule, et, si la phase suivante est déjà ouverte (pas encore tirée), parmi ses entrants.
func (s *State) applyRepechage(ev Event) error {
	ph := s.phaseOf(ev.Phase)
	if ph == nil {
		return fmt.Errorf("phase %d inconnue", ev.Phase)
	}
	if err := s.repechageOpen(ph); err != nil {
		return err
	}
	sec := ph.section(ev.Section)
	if sec == nil || sec.Kind != secKindPool {
		return fmt.Errorf("repêchage : poule %q inconnue", ev.Section)
	}
	member := map[PlayerID]bool{}
	for _, p := range poolMembers(sec) {
		member[p] = true
	}
	qualified := map[PlayerID]bool{}
	for _, p := range s.rrQualified(ph) {
		qualified[p] = true
	}
	if !member[ev.A] || !qualified[ev.A] || !s.Withdrawn[ev.A] {
		return fmt.Errorf("repêchage : %s n'est pas un qualifié retiré de %s", ev.A, ev.Section)
	}
	if !member[ev.B] || qualified[ev.B] || s.Withdrawn[ev.B] {
		return fmt.Errorf("repêchage : %s n'est pas un joueur non qualifié et présent de %s", ev.B, ev.Section)
	}
	if ph.Repechages == nil {
		ph.Repechages = map[PlayerID]PlayerID{}
	}
	ph.Repechages[ev.A] = ev.B
	if next := s.phaseOf(ph.Index + 1); next != nil {
		// Le passage de phase a déjà eu lieu : le retiré avait pris sa place parmi les entrants
		// (enterFrom ne l'écarte que s'il était déjà retiré). Le repêché la reprend.
		for i, p := range next.Entrants {
			if p == ev.A {
				next.Entrants = append(next.Entrants[:i:i], next.Entrants[i+1:]...)
				delete(next.Lives, ev.A)
				break
			}
		}
		s.enter(next, ev.B, s.entryLives(next, ph, ev.B))
	}
	return nil
}
