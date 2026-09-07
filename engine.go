package tournoi

import (
	"fmt"
	"sort"
	"time"
)

// Propose renvoie les actions à effectuer maintenant. Le TD en confirme tout ou partie ; chaque
// confirmation devient un événement (voir State.EventFromAction). Déterministe pour un journal donné.
//
// « Maintenant » est ici l'horodatage du dernier événement du journal : le moteur n'a pas
// d'horloge, et ne va pas s'en inventer une. Un hôte qui affiche un compte à rebours ou qui
// veut les avertissements de pause à sa propre heure appelle ProposeAt.
func (s *State) Propose() []Action { return s.ProposeAt(s.Last) }

// ProposeAt est Propose à une heure donnée. Elle ne change que ce qui dépend du temps : les
// micro-rondes (l'échéance du prochain lot) et les pauses (la fin attendue d'un match proposé).
// Le reste — appariements, tirages, passage de phase — ne dépend que du journal.
//
// Le rejeu n'en est pas affecté : ce qui est rejoué, ce sont les événements, pas les
// propositions. Deux hôtes dont les horloges diffèrent proposent les mêmes matchs, à des
// instants différents.
func (s *State) ProposeAt(now time.Time) []Action {
	if s.Current < 0 || s.Finished {
		return nil
	}
	ph := s.phase()
	var acts []Action
	switch ph.Cfg.Kind {
	case KindSwissLives:
		if éch, lot := s.batchDeadline(ph); lot && now.Before(éch) && !s.swissDone(ph) {
			// Micro-rondes : les joueurs libres attendent l'échéance. L'action porte
			// l'échéance pour que l'hôte affiche un compte à rebours.
			return []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonWaitingBatch, Until: éch}}
		}
		acts = s.proposeSwiss(ph)
	case KindGSL:
		acts = s.proposeGSL(ph)
	case KindLivesBracket, KindBracket:
		acts = s.proposeBracket(ph)
	case KindRoundRobin:
		acts = s.proposeRR(ph)
	}
	if len(acts) == 0 {
		if s.runningInPhase(ph) > 0 {
			return []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonMatchesRunning}}
		}
		if s.phaseDone(ph) {
			if s.Current+1 < len(s.Config.Phases) {
				return []Action{{Kind: ActNextPhase, Phase: ph.Index, Label: Label{Kind: LabelPhase, Text: PhaseName(s.Config.Phases[s.Current+1])}}}
			}
			return []Action{{Kind: ActFinish, Phase: ph.Index}}
		}
		return []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonNoPairing}}
	}
	s.assignTables(acts)
	s.flagBreaks(acts, now)
	return acts
}

// phaseDone : la phase n'a plus rien à jouer.
func (s *State) phaseDone(ph *PhaseState) bool {
	switch ph.Cfg.Kind {
	case KindSwissLives:
		return s.swissDone(ph)
	case KindGSL:
		return s.gslDone(ph)
	case KindRoundRobin:
		return ph.Drawn && ph.allDone() && s.rrBarragesDone(ph)
	}
	return ph.Drawn && ph.allDone()
}

func (s *State) runningInPhase(ph *PhaseState) int {
	n := 0
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Status == Running && m.Phase == ph.Index {
			n++
		}
	}
	return n
}

// Running renvoie les matchs en cours.
func (s *State) Running() []*Match {
	var out []*Match
	for _, id := range s.MatchOrder {
		if m := s.Matches[id]; m.Status == Running {
			out = append(out, m)
		}
	}
	return out
}

// assignTables attribue les plus petites tables libres aux matchs proposés.
// assignTables attribue une table à chaque match proposé : la plus petite qui soit libre,
// disponible, et non réservée à autre chose (tables.go). Une proposition qui n'en trouve pas
// reste dans la file avec ReasonWaitingTable — le TD peut la lancer avec un numéro saisi.
func (s *State) assignTables(acts []Action) {
	used := map[int]bool{}
	for _, m := range s.Running() {
		if m.Table > 0 {
			used[m.Table] = true
		}
	}
	max := s.Config.Tables.Count
	for i := range acts {
		if acts[i].Kind != ActStartMatch || acts[i].Table > 0 {
			continue
		}
		found := 0
		for t := 1; max == 0 || t <= max; t++ {
			if max == 0 && t > len(used)+len(s.Config.Tables.Unavailable)+len(s.Config.Tables.Reserved)+1 {
				break // salle illimitée : inutile de chercher au-delà
			}
			if used[t] || !s.Config.Tables.AvailableFor(t, acts[i].Section, acts[i].Phase) {
				continue
			}
			found = t
			break
		}
		if found == 0 {
			acts[i].Reason = ReasonWaitingTable
			continue
		}
		acts[i].Table = found
		used[found] = true
	}
}

// Apply-and-propose helpers for hosts: Step applique une liste d'événements puis propose.
func (s *State) Step(evs ...Event) ([]Action, error) {
	for _, ev := range evs {
		if err := s.Apply(ev); err != nil {
			return nil, err
		}
	}
	return s.Propose(), nil
}

// sortedIDs renvoie une copie triée (ordre stable indépendant des maps).
func sortedIDs(ids []PlayerID) []PlayerID {
	out := append([]PlayerID{}, ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// String décrit une action pour le TD.
func (a Action) String() string {
	switch a.Kind {
	case ActStartMatch:
		s := fmt.Sprintf("Lancer %s : %s contre %s en %d points, table %d", a.Label, a.A, a.B, a.Length, a.Table)
		if a.Warn != "" {
			s += " — " + Warning{Code: a.Warn}.String()
		}
		return s
	case ActBye:
		return fmt.Sprintf("Bye pour %s (%s)", a.A, a.Label)
	case ActDraw:
		return fmt.Sprintf("Tirage : %s", a.Label)
	case ActNextPhase:
		return fmt.Sprintf("Passer à la phase suivante : %s", a.Label)
	case ActFinish:
		return "Clore le tournoi"
	}
	if a.Reason == ReasonWaitingBatch && !a.Until.IsZero() {
		return fmt.Sprintf("Attendre : %s (jusqu'à %s)", a.Reason, a.Until.Format("15:04"))
	}
	return fmt.Sprintf("Attendre : %s", a.Reason)
}
