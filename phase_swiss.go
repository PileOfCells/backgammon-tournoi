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

// free : joueurs en vie sans match en cours, ni ici ni dans une autre épreuve de la salle, et
// disponibles pour la ronde r (0 = hors rondes ; voir absence.go).
func (s *State) free(ph *PhaseState, r int) []PlayerID {
	var out []PlayerID
	for _, p := range s.alive(ph) {
		if !s.busy(p) && !s.absent(p, r) && !s.ailleurs[p] {
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
	free := s.free(ph, 0)
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

// proposeSwissRound : mode par rondes (tous les matchs de la ronde en même temps, byes aux
// groupes impairs).
//
// UNE RONDE RESTE OUVERTE tant que l'un des joueurs appelés n'y est pas engagé. Une ronde de
// 12 matchs dans une salle de 6 tables se lance en deux vagues ; un bye se confirme avant les
// matchs. Le moteur ne proposait une ronde qu'avec zéro match en cours, puis appariait « la
// ronde suivante » : lancer la première vague faisait disparaître la seconde, et confirmer le
// bye seul faisait passer à la ronde N+1, l'exempté réapparié. Désormais, tant que la ronde
// Round est ouverte, ses appariements non lancés restent proposés, LES MÊMES ; la ronde suivante
// ne vient que quand la ronde est entièrement engagée et ses matchs finis.
//
// Les mêmes appariements, parce qu'ils sont RECALCULÉS à l'identique plutôt que stockés : le
// plan d'une ronde est fonction de l'état d'avant la ronde (beforeRound retire ce que la ronde a
// déjà produit), des joueurs appelés (PhaseState.Roster, relevé au premier événement de la
// ronde) et d'un générateur propre à la ronde (roundRng). Le journal ne porte que ce qui a été
// confirmé, comme toujours.
//
// La bascule vers un tableau (Target) demande que la somme des vies TOMBE JUSTE sur une
// puissance de 2. Chaque match la fait décroître d'une unité, mais une ronde en lance beaucoup
// d'un coup : sans garde-fou, la dernière ronde passe sous la cible et le tableau qui suit
// n'est plus complet. Mesuré avant correction sur 2 vies et une cible de 16 : la somme
// atterrissait entre 12 et 15 selon l'effectif, jamais sur 16, et le tableau se remplissait de
// byes qui n'avaient pas lieu d'être.
//
// Le mode continu avait déjà ce budget ; les rondes ne l'avaient pas. La DERNIÈRE ronde d'une
// phase à bascule est donc tronquée au nombre exact de matchs qui reste à jouer, et ne donne
// aucun bye : elle est la dernière, et un bye enregistré fausserait l'ordre d'appariement d'une
// ronde qui n'aura pas lieu.
func (s *State) proposeSwissRound(ph *PhaseState) []Action {
	if ph.Round > 0 {
		if rest := s.roundRest(ph, ph.Round); len(rest) > 0 {
			return rest
		}
	}
	if s.runningInPhase(ph) > 0 {
		return nil
	}
	r := ph.Round + 1
	free := s.free(ph, r)
	acts, normal := s.pairRound(ph, r, free, s.roundRng(ph, r))
	if !normal && len(free) >= 2 { // secours comme en continu
		return s.proposeSwissContinuousFallback(ph, free, s.rng())
	}
	return acts
}

// roundRng : le générateur d'une ronde. Il ne dépend que de la graine, de la phase et du numéro
// de ronde — pas du nombre d'événements, sans quoi confirmer le premier match d'une ronde
// redistribuerait les suivants.
func (s *State) roundRng(ph *PhaseState, r int) *rand.Rand {
	return rand.New(rand.NewSource(s.Seed*1000003 + int64(ph.Index)*7919 + int64(r)*104729))
}

// pairRound : le plan de la ronde r pour ces joueurs, sur l'état v. normal = faux quand aucun
// appariement n'a été trouvé (le secours est alors l'affaire de l'appelant).
func (s *State) pairRound(v *PhaseState, r int, players []PlayerID, rng *rand.Rand) (acts []Action, normal bool) {
	budget := 1 << 30
	if v.Cfg.Target > 0 {
		budget = s.sumLives(v) - v.Cfg.Target // matchs encore lançables avant la bascule
		if budget <= 0 {
			return nil, true
		}
	}
	lbl := Label{Kind: LabelRound, N: r}
	var restes []PlayerID
	for l := 0; l < v.Cfg.Lives && budget > 0; l++ {
		var g []PlayerID
		for _, p := range players {
			if v.Losses[p] == l {
				g = append(g, p)
			}
		}
		pairs, rest := s.pairGroup(v, g, rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Action{Kind: ActStartMatch, Phase: v.Index, Label: lbl, Round: r, A: pr[0], B: pr[1], Length: s.swissLength(v)})
			budget--
		}
		restes = append(restes, rest...)
	}
	if len(acts) == 0 {
		return nil, false
	}
	if budget <= 0 {
		return acts, true // ronde tronquée par la bascule : c'est la dernière, pas de bye
	}
	for _, p := range restes {
		acts = append(acts, Action{Kind: ActBye, Phase: v.Index, Label: lbl, Round: r, A: p})
	}
	return acts, true
}

// openRound : au premier événement d'une ronde (match ou bye), relever les joueurs appelés —
// ceux qu'un Propose de cet instant aurait appariés. Appelée par Apply AVANT d'appliquer
// l'événement, quand ses joueurs sont encore libres.
func (s *State) openRound(ph *PhaseState, r int) {
	if ph.Cfg.Kind != KindSwissLives || ph.Cfg.Mode != "rounds" || r <= ph.Round {
		return
	}
	ph.Roster = s.free(ph, r)
}

// engagedIn : les joueurs déjà engagés dans la ronde r de la phase — un match non annulé de
// cette ronde, ou un bye de cette ronde.
func (s *State) engagedIn(ph *PhaseState, r int) map[PlayerID]bool {
	out := map[PlayerID]bool{}
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Phase == ph.Index && m.Round == r && m.Status != Cancelled {
			out[m.A], out[m.B] = true, true
		}
	}
	for p, rs := range ph.ByeRounds {
		for _, x := range rs {
			if x == r {
				out[p] = true
			}
		}
	}
	return out
}

// beforeRound : la phase telle qu'elle était avant la ronde r — victoires, défaites,
// adversaires et byes de la ronde r retirés. C'est l'état sur lequel la ronde a été appariée.
func (s *State) beforeRound(ph *PhaseState, r int) *PhaseState {
	v := *ph
	v.Losses, v.Wins, v.Opponents = map[PlayerID]int{}, map[PlayerID]int{}, map[PlayerID][]PlayerID{}
	v.Byes = map[PlayerID]int{}
	for p, n := range ph.Byes {
		v.Byes[p] = n
	}
	for p, rs := range ph.ByeRounds {
		for _, x := range rs {
			if x == r {
				v.Byes[p]--
			}
		}
	}
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Phase != ph.Index || m.Status == Cancelled || m.Round == r {
			continue
		}
		v.Opponents[m.A] = append(v.Opponents[m.A], m.B)
		v.Opponents[m.B] = append(v.Opponents[m.B], m.A)
		if m.Status == Finished {
			v.Wins[m.Winner]++
			v.Losses[m.Loser()]++
		}
	}
	return &v
}

// roundRest : ce qui reste à confirmer de la ronde r, ouverte. Le plan est recalculé sur l'état
// d'avant la ronde, pour les joueurs appelés ; on garde ce dont aucun joueur n'est encore
// engagé, ni retiré, ni occupé. Les ORPHELINS — prévus contre un joueur qui a joué autre chose
// (un match lancé à la main) ou qui est parti — sont appariés entre eux, dans le budget de la
// bascule s'il y en a une. Un orphelin seul attend la ronde suivante.
func (s *State) roundRest(ph *PhaseState, r int) []Action {
	if len(ph.Roster) == 0 {
		return nil
	}
	engagé := s.engagedIn(ph, r)
	v := s.beforeRound(ph, r)
	plan, normal := s.pairRound(v, r, ph.Roster, s.roundRng(ph, r))
	if !normal {
		return nil
	}
	libre := func(p PlayerID) bool {
		return !engagé[p] && s.remainingLives(ph, p) > 0 && !s.busy(p) && !s.absent(p, r) && !s.ailleurs[p]
	}
	var acts []Action
	pris := map[PlayerID]bool{}
	for _, a := range plan {
		switch {
		case a.Kind == ActBye && libre(a.A):
			acts = append(acts, a)
			pris[a.A] = true
		case a.Kind == ActStartMatch && libre(a.A) && libre(a.B):
			a.Length = s.swissLength(ph) // la longueur est celle d'aujourd'hui (length_changed)
			acts = append(acts, a)
			pris[a.A], pris[a.B] = true, true
		}
	}
	var orphelins []PlayerID
	for _, a := range plan {
		for _, p := range []PlayerID{a.A, a.B} {
			if p != "" && !pris[p] && libre(p) {
				orphelins = append(orphelins, p)
				pris[p] = true
			}
		}
	}
	if len(orphelins) < 2 {
		return acts
	}
	budget := 1 << 30
	if ph.Cfg.Target > 0 {
		budget = s.sumLives(ph) - s.runningInPhase(ph) - ph.Cfg.Target
		for _, a := range acts {
			if a.Kind == ActStartMatch {
				budget--
			}
		}
	}
	rng := s.roundRng(ph, r)
	for l := 0; l < ph.Cfg.Lives && budget > 0; l++ {
		var g []PlayerID
		for _, p := range orphelins {
			if v.Losses[p] == l {
				g = append(g, p)
			}
		}
		pairs, _ := s.pairGroup(v, g, rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Label: Label{Kind: LabelRound, N: r}, Round: r,
				A: pr[0], B: pr[1], Length: s.swissLength(ph)})
			budget--
		}
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
