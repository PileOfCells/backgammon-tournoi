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
func (s *State) ProposeAt(now time.Time) []Action { return s.ProposeWith(now, External{}) }

// External décrit ce que l'hôte sait de la salle et que le journal de CE tournoi ignore : les
// tables où joue, en ce moment, une autre épreuve dirigée à côté, et les joueurs de ce tournoi
// qui y jouent. Ce n'est pas un événement : l'occupation par l'extérieur change à chaque match
// de l'autre épreuve, elle n'appartient pas à l'histoire de celle-ci, et le rejeu n'en a pas
// besoin — le match_started confirmé porte, lui, la table et les joueurs effectivement pris.
type External struct {
	// BusyTables : tables occupées hors de ce tournoi. L'attribution les saute ; une proposition
	// qui ne trouve plus de table attend (ReasonWaitingTable) au lieu de collisionner.
	BusyTables []int `json:"busy_tables,omitempty"`
	// BusyPlayers : joueurs de CE tournoi (ses PlayerID ; l'hôte traduit ceux de l'autre
	// épreuve) qui jouent en ce moment ailleurs. Au suisse et en barrage ils ne sont pas
	// appariés, et la file porte une attente ReasonPlayerBusy par joueur ; dans un graphe, où la
	// place est fixée, leur match reste proposé, sans table, avec cette raison. Un identifiant
	// inconnu est ignoré.
	BusyPlayers []PlayerID `json:"busy_players,omitempty"`
}

// ProposeWith est ProposeAt avec ce que l'hôte sait de l'extérieur (External). L'attribution des
// tables et la disponibilité des joueurs en dépendent ; les tirages et passages de phase restent
// fonction du seul journal.
func (s *State) ProposeWith(now time.Time, ext External) []Action {
	if s.Current < 0 || s.Finished {
		return nil
	}
	// Une copie superficielle, à l'heure de la proposition : les indisponibilités à échéance en
	// dépendent, et Propose ne modifie pas l'état (un hôte peut l'appeler sous verrou de lecture).
	v := *s
	v.clock = now
	if len(ext.BusyPlayers) > 0 {
		v.ailleurs = make(map[PlayerID]bool, len(ext.BusyPlayers))
		for _, p := range ext.BusyPlayers {
			if _, ok := s.Players[p]; ok {
				v.ailleurs[p] = true
			}
		}
	}
	return v.propose(now, ext)
}

func (s *State) propose(now time.Time, ext External) []Action {
	ph := s.phase()
	var acts []Action
	switch ph.Cfg.Kind {
	case KindSwissLives:
		if éch, lot := s.batchDeadline(ph); lot && now.Before(éch) && !s.swissDone(ph) {
			// Micro-rondes : les joueurs libres attendent l'échéance. L'action porte
			// l'échéance pour que l'hôte affiche un compte à rebours.
			return s.withAbsences(ph, []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonWaitingBatch, Until: éch}})
		}
		acts = s.proposeSwiss(ph)
	case KindGSL:
		acts = s.proposeGSL(ph)
	case KindLivesBracket, KindBracket:
		acts = s.proposeBracket(ph)
	case KindRoundRobin:
		acts = s.proposeRR(ph)
	}
	// La réparation d'un graphe désaccordé passe DEVANT : on annule avant de relancer
	// (reparation.go). Elle est vide dans un tournoi mené normalement.
	rep := s.proposeRepair(ph)
	rep = append(rep, s.proposeSwissRepair(ph)...)
	if len(rep) > 0 {
		acts = append(rep, acts...)
	}
	if len(acts) == 0 {
		if s.runningInPhase(ph) > 0 {
			return s.withAbsences(ph, []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonMatchesRunning}})
		}
		if s.phaseDone(ph) {
			if s.Current+1 < len(s.Config.Phases) {
				// Un qualifié de poule retiré : le repêchage est proposé AVANT le passage, qui
				// reste proposé — le confirmer sans repêcher laisse une exemption (N26).
				return append(s.proposeRepechages(ph), Action{Kind: ActNextPhase, Phase: ph.Index, Label: Label{Kind: LabelPhase, Text: PhaseName(s.Config.Phases[s.Current+1])}})
			}
			return []Action{{Kind: ActFinish, Phase: ph.Index}}
		}
		return s.withAbsences(ph, []Action{{Kind: ActWait, Phase: ph.Index, Reason: ReasonNoPairing}})
	}
	if ph.Index > 0 && !ph.Drawn && !ph.Started {
		// Passage confirmé, tirage pas encore fait : le repêchage reste possible (N26).
		acts = append(s.proposeRepechages(s.phaseOf(ph.Index-1)), acts...)
	}
	if ph.Cfg.Kind != KindSwissLives {
		s.holdAbsent(acts) // un suisse n'apparie pas les absents ; un graphe les retient
		s.holdElsewhere(acts)
	}
	s.assignTables(acts, ext.BusyTables)
	s.flagBreaks(acts, now)
	return s.withAbsences(ph, acts)
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

// assignTables attribue une table à chaque match proposé : la plus petite qui soit libre,
// disponible, non réservée à autre chose (tables.go) et non occupée par une autre épreuve de la
// salle (dehors, voir External). Une proposition qui n'en trouve pas reste dans la file avec
// ReasonWaitingTable — le TD peut la lancer avec un numéro saisi.
func (s *State) assignTables(acts []Action, dehors []int) {
	used := map[int]bool{}
	for _, m := range s.Running() {
		if m.Table > 0 {
			used[m.Table] = true
		}
	}
	for _, t := range dehors {
		if t > 0 {
			used[t] = true
		}
	}
	max := s.Config.Tables.Count
	for i := range acts {
		if acts[i].Kind != ActStartMatch || acts[i].Table > 0 || acts[i].Reason != ReasonNone {
			continue // déjà placée, ou retenue (player_unavailable) : pas de table
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
	case ActRepechage:
		return fmt.Sprintf("%s : %s à la place de %s, retiré", a.Label, a.B, a.A)
	case ActCancelMatch:
		return fmt.Sprintf("Annuler %s : %s contre %s (%s), devenu incohérent", a.Match, a.A, a.B, a.Label)
	case ActFinish:
		return "Clore le tournoi"
	}
	if a.Reason == ReasonWaitingBatch && !a.Until.IsZero() {
		return fmt.Sprintf("Attendre : %s (jusqu'à %s)", a.Reason, a.Until.Format("15:04"))
	}
	return fmt.Sprintf("Attendre : %s", a.Reason)
}
