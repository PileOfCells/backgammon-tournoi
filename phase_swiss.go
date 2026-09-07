package tournoi

import (
	"math/rand"
	"sort"
)

// ---- Suisse à vies : appariement dans le groupe de même nombre de défaites ----

// sumLives : somme des vies restantes des joueurs en vie.
func (s *State) sumLives(ph *PhaseState) int {
	n := 0
	for _, p := range ph.Entrants {
		n += s.remainingLives(ph, p)
	}
	return n
}

// swissFrozen : la bascule vers le tableau est atteinte (somme des vies ≤ Target, plus de match en cours).
func (s *State) swissFrozen(ph *PhaseState) bool {
	return ph.Cfg.Target > 0 && s.sumLives(ph)-s.runningInPhase(ph) <= ph.Cfg.Target
}

func (s *State) swissDone(ph *PhaseState) bool {
	if s.runningInPhase(ph) > 0 {
		return false
	}
	if ph.Cfg.Target > 0 && s.sumLives(ph) <= ph.Cfg.Target {
		return true
	}
	return len(s.alive(ph)) <= 1
}

// free : joueurs en vie sans match en cours.
func (s *State) free(ph *PhaseState) []PlayerID {
	var out []PlayerID
	for _, p := range s.alive(ph) {
		if !s.busy(p) {
			out = append(out, p)
		}
	}
	return out
}

func (s *State) met(ph *PhaseState, a, b PlayerID) bool {
	for _, o := range ph.Opponents[a] {
		if o == b {
			return true
		}
	}
	return false
}

func (s *State) sameClub(a, b PlayerID) bool {
	pa, pb := s.Players[a], s.Players[b]
	return pa != nil && pb != nil && pa.Club != "" && pa.Club == pb.Club
}

// pairGroup apparie un groupe (glouton, ordre tiré au sort) en évitant rematchs et clubs.
func (s *State) pairGroup(ph *PhaseState, g []PlayerID, rng *rand.Rand, allowRematch bool) (pairs [][2]PlayerID, rest []PlayerID) {
	g = sortedIDs(g)
	rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
	if ph.Cfg.Pairing == "wins" {
		sort.SliceStable(g, func(i, j int) bool { return ph.Wins[g[i]] > ph.Wins[g[j]] })
	}
	// ceux qui ont déjà eu un bye sont appariés en premier
	sort.SliceStable(g, func(i, j int) bool { return ph.Byes[g[i]] > ph.Byes[g[j]] })
	libres := append([]PlayerID{}, g...)
	for len(libres) >= 2 {
		a := libres[0]
		libres = libres[1:]
		found := -1
		passes := []bool{false}
		if ph.Cfg.AvoidClubs {
			passes = []bool{true, false}
		}
		for _, exigerClub := range passes {
			for i, c := range libres {
				if !allowRematch && !ph.Cfg.AllowRematch && s.met(ph, a, c) {
					continue
				}
				if exigerClub && s.sameClub(a, c) {
					continue
				}
				found = i
				break
			}
			if found >= 0 {
				break
			}
		}
		if found < 0 {
			rest = append(rest, a)
			continue
		}
		pairs = append(pairs, [2]PlayerID{a, libres[found]})
		libres = append(libres[:found], libres[found+1:]...)
	}
	rest = append(rest, libres...)
	return pairs, rest
}

// swissLength : longueur des matchs à lancer maintenant dans un suisse.
//
// Un suisse s'allonge quand il ne reste qu'une poignée de joueurs : les derniers matchs
// décident du tournoi et méritent d'être plus longs. La règle est automatique, mais un
// changement décidé à la main par le TD (EvLengthChanged) l'emporte — c'est lui qui dirige,
// et il a pu allonger ou raccourcir pour une raison que le moteur ignore (l'horaire de la salle).
func (s *State) swissLength(ph *PhaseState) int {
	if ph.Cfg.LengthLate > 0 && ph.Length == ph.Cfg.Length && len(s.alive(ph)) <= ph.Cfg.LateThreshold {
		return ph.Cfg.LengthLate
	}
	return ph.Length
}

func (s *State) proposeSwiss(ph *PhaseState) []Action {
	if s.swissDone(ph) {
		return nil
	}
	if ph.Cfg.Mode == "rounds" {
		return s.proposeSwissRound(ph)
	}
	rng := s.rng()
	L := ph.Cfg.Lives
	free := s.free(ph)
	var acts []Action
	budget := 1 << 30
	if ph.Cfg.Target > 0 {
		budget = s.sumLives(ph) - s.runningInPhase(ph) - ph.Cfg.Target // matchs encore lançables
		if budget <= 0 {
			return nil
		}
	}
	label := func(a PlayerID) Label {
		return Label{Kind: LabelSwissGroup, Losses: ph.Losses[a], Match: ph.Wins[a] + ph.Losses[a] + 1}
	}
	for l := 0; l < L && budget > 0; l++ {
		var g []PlayerID
		for _, p := range free {
			if ph.Losses[p] == l {
				g = append(g, p)
			}
		}
		pairs, _ := s.pairGroup(ph, g, rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Label: label(pr[0]), A: pr[0], B: pr[1], Length: s.swissLength(ph)})
			budget--
		}
	}
	if len(acts) == 0 && s.runningInPhase(ph) == 0 && len(free) >= 2 {
		// secours : rematch dans le groupe, sinon match croisé entre les deux joueurs ayant le moins de défaites
		for l := 0; l < L; l++ {
			var g []PlayerID
			for _, p := range free {
				if ph.Losses[p] == l {
					g = append(g, p)
				}
			}
			pairs, _ := s.pairGroup(ph, g, rng, true)
			if len(pairs) > 0 {
				pr := pairs[0]
				return []Action{{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelRematch}, A: pr[0], B: pr[1], Length: s.swissLength(ph)}}
			}
		}
		fr := sortedIDs(free)
		sort.SliceStable(fr, func(i, j int) bool { return ph.Losses[fr[i]] < ph.Losses[fr[j]] })
		lbl := Label{Kind: LabelFinal}
		if len(fr) > 2 {
			lbl = Label{Kind: LabelCrossed}
		}
		return []Action{{Kind: ActStartMatch, Phase: ph.Index, Label: lbl, A: fr[0], B: fr[1], Length: s.swissLength(ph)}}
	}
	return acts
}

// proposeSwissRound : mode par rondes (tous les matchs de la ronde en même temps, byes aux groupes impairs).
func (s *State) proposeSwissRound(ph *PhaseState) []Action {
	if s.runningInPhase(ph) > 0 {
		return nil
	}
	rng := s.rng()
	free := s.free(ph)
	var acts []Action
	var restes []PlayerID
	for l := 0; l < ph.Cfg.Lives; l++ {
		var g []PlayerID
		for _, p := range free {
			if ph.Losses[p] == l {
				g = append(g, p)
			}
		}
		pairs, rest := s.pairGroup(ph, g, rng, false)
		for _, pr := range pairs {
			acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelRound, N: ph.Round + 1}, Round: ph.Round + 1, A: pr[0], B: pr[1], Length: s.swissLength(ph)})
		}
		restes = append(restes, rest...)
	}
	if len(acts) == 0 && len(free) >= 2 { // secours comme en continu
		return s.proposeSwissContinuousFallback(ph, free, rng)
	}
	for _, p := range restes {
		acts = append(acts, Action{Kind: ActBye, Phase: ph.Index, Label: Label{Kind: LabelRound, N: ph.Round + 1}, Round: ph.Round + 1, A: p})
	}
	return acts
}

func (s *State) proposeSwissContinuousFallback(ph *PhaseState, free []PlayerID, rng *rand.Rand) []Action {
	for l := 0; l < ph.Cfg.Lives; l++ {
		var g []PlayerID
		for _, p := range free {
			if ph.Losses[p] == l {
				g = append(g, p)
			}
		}
		pairs, _ := s.pairGroup(ph, g, rng, true)
		if len(pairs) > 0 {
			return []Action{{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelRematch}, A: pairs[0][0], B: pairs[0][1], Length: s.swissLength(ph)}}
		}
	}
	fr := sortedIDs(free)
	sort.SliceStable(fr, func(i, j int) bool { return ph.Losses[fr[i]] < ph.Losses[fr[j]] })
	return []Action{{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelFinal}, A: fr[0], B: fr[1], Length: s.swissLength(ph)}}
}
