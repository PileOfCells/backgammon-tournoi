package rules

import (
	"math/rand"
	"sort"
)

// ---- Suisse à vies : appariement dans le groupe de même nombre de défaites ----

func (v *vue) swissDone() bool {
	if v.Running > 0 {
		return false
	}
	if v.Rules.Target > 0 && v.livesLeft() <= v.Rules.Target {
		return true
	}
	return len(v.alive()) <= 1
}

// SwissLength : la longueur des matchs à lancer maintenant dans un suisse. Elle passe à
// LengthLate quand il ne reste plus que LateThreshold joueurs en vie ou moins, sauf si le
// directeur a changé la longueur à la main (Length ≠ Rules.Length) : la sienne l'emporte.
func SwissLength(p Phase) int { return voir(&p).swissLength() }

func (v *vue) swissLength() int {
	if v.Rules.LengthLate > 0 && v.Length == v.Rules.Length && len(v.alive()) <= v.Rules.LateThreshold {
		return v.Rules.LengthLate
	}
	return v.Length
}

func (v *vue) met(a, b Ref) bool {
	for _, o := range v.entrant(a).Opponents {
		if o == b {
			return true
		}
	}
	return false
}

func (v *vue) sameClub(a, b Ref) bool {
	ca, cb := v.entrant(a).Club, v.entrant(b).Club
	return ca != "" && ca == cb
}

func sortedRefs(ids []Ref) []Ref {
	out := append([]Ref{}, ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// pairGroup apparie un groupe : glouton, dans un ordre tiré au sort (après le tri par
// identifiant), les plus victorieux d'abord en appariement « wins », ceux qui ont déjà eu un bye
// en premier ; rematchs évités (sauf permission), clubs évités quand c'est possible.
func (v *vue) pairGroup(g []Ref, rng *rand.Rand, allowRematch bool) (pairs [][2]Ref, rest []Ref) {
	g = sortedRefs(g)
	rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
	if v.Rules.Pairing == PairingWins {
		sort.SliceStable(g, func(i, j int) bool { return v.entrant(g[i]).Wins > v.entrant(g[j]).Wins })
	}
	sort.SliceStable(g, func(i, j int) bool { return v.entrant(g[i]).Byes > v.entrant(g[j]).Byes })
	libres := append([]Ref{}, g...)
	for len(libres) >= 2 {
		a := libres[0]
		libres = libres[1:]
		found := -1
		passes := []bool{false}
		if v.Rules.AvoidClubs {
			passes = []bool{true, false}
		}
		for _, exigerClub := range passes {
			for i, c := range libres {
				if !allowRematch && !v.Rules.AllowRematch && v.met(a, c) {
					continue
				}
				if exigerClub && v.sameClub(a, c) {
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
		pairs = append(pairs, [2]Ref{a, libres[found]})
		libres = append(libres[:found], libres[found+1:]...)
	}
	rest = append(rest, libres...)
	return pairs, rest
}

// groupe : les joueurs de players qui ont l perdu.
func (v *vue) groupe(players []Ref, l int) []Ref {
	var g []Ref
	for _, p := range players {
		if v.entrant(p).Losses == l {
			g = append(g, p)
		}
	}
	return g
}

func (v *vue) pairSwiss(seed Seed) []Pairing {
	if v.swissDone() {
		return nil
	}
	if v.Rules.Mode == ModeRounds {
		if v.Running > 0 {
			return nil
		}
		r := v.Round + 1
		acts, normal := v.pairRound(r, v.Free, generateur(seed.Round(r)))
		if !normal && len(v.Free) >= 2 {
			return v.rematchOrFinal(v.Free, generateur(seed.Draw()))
		}
		return acts
	}
	return v.pairContinuous(generateur(seed.Draw()))
}

// pairContinuous : le suisse continu. Chaque groupe de défaites est apparié ; la bascule
// (Target) borne le nombre de matchs lançables. Sans aucune paire et sans match en cours, le
// secours : un rematch dans un groupe, sinon le match croisé des deux joueurs qui ont le moins
// de défaites (une finale s'ils ne sont que deux).
func (v *vue) pairContinuous(rng *rand.Rand) []Pairing {
	L := v.Rules.Lives
	free := v.Free
	var acts []Pairing
	budget := 1 << 30
	if v.Rules.Target > 0 {
		budget = v.livesLeft() - v.Running - v.Rules.Target
		if budget <= 0 {
			return nil
		}
	}
	label := func(a Ref) Label {
		e := v.entrant(a)
		return Label{Kind: LabelSwissGroup, Losses: e.Losses, Match: e.Wins + e.Losses + 1}
	}
	for l := 0; l < L && budget > 0; l++ {
		pairs, _ := v.pairGroup(v.groupe(free, l), rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Pairing{Kind: PairMatch, Label: label(pr[0]), A: pr[0], B: pr[1], Length: v.swissLength()})
			budget--
		}
	}
	if len(acts) == 0 && v.Running == 0 && len(free) >= 2 {
		for l := 0; l < L; l++ {
			pairs, _ := v.pairGroup(v.groupe(free, l), rng, true)
			if len(pairs) > 0 {
				pr := pairs[0]
				return []Pairing{{Kind: PairMatch, Label: Label{Kind: LabelRematch}, A: pr[0], B: pr[1], Length: v.swissLength()}}
			}
		}
		fr := v.leastLosses(free)
		lbl := Label{Kind: LabelFinal}
		if len(fr) > 2 {
			lbl = Label{Kind: LabelCrossed}
		}
		return []Pairing{{Kind: PairMatch, Label: lbl, A: fr[0], B: fr[1], Length: v.swissLength()}}
	}
	return acts
}

// leastLosses : les joueurs triés par identifiant, puis (stable) par défaites croissantes.
func (v *vue) leastLosses(players []Ref) []Ref {
	fr := sortedRefs(players)
	sort.SliceStable(fr, func(i, j int) bool { return v.entrant(fr[i]).Losses < v.entrant(fr[j]).Losses })
	return fr
}

// PairSwissRound : le plan de la ronde r d'un suisse par rondes pour ces joueurs, sur
// l'instantané p — celui d'AVANT la ronde quand l'hôte recalcule une ronde ouverte. Les matchs
// de chaque groupe de défaites, puis un bye par joueur resté seul. La dernière ronde d'une phase
// à bascule est tronquée au nombre exact de matchs qui reste à jouer (Σ vies − Target) et ne
// donne aucun bye. normal est faux quand aucune paire n'a été trouvée : le secours
// (PairSwissFallback) est alors à l'appelant.
func PairSwissRound(p Phase, r int, players []Ref, seed Seed) (plan []Pairing, normal bool) {
	return voir(&p).pairRound(r, players, generateur(seed.Round(r)))
}

func (v *vue) pairRound(r int, players []Ref, rng *rand.Rand) (acts []Pairing, normal bool) {
	budget := 1 << 30
	if v.Rules.Target > 0 {
		budget = v.livesLeft() - v.Rules.Target
		if budget <= 0 {
			return nil, true
		}
	}
	lbl := Label{Kind: LabelRound, N: r}
	var restes []Ref
	for l := 0; l < v.Rules.Lives && budget > 0; l++ {
		pairs, rest := v.pairGroup(v.groupe(players, l), rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Pairing{Kind: PairMatch, Label: lbl, Round: r, A: pr[0], B: pr[1], Length: v.swissLength()})
			budget--
		}
		restes = append(restes, rest...)
	}
	if len(acts) == 0 {
		return nil, false
	}
	if budget <= 0 {
		return acts, true
	}
	for _, p := range restes {
		acts = append(acts, Pairing{Kind: PairBye, Label: lbl, Round: r, A: p})
	}
	return acts, true
}

// PairSwissFallback : le secours d'un suisse par rondes qui ne trouve aucune paire — un rematch
// dans le premier groupe qui en donne un, sinon la « finale » des deux joueurs qui ont le moins
// de défaites.
func PairSwissFallback(p Phase, free []Ref, seed Seed) []Pairing {
	return voir(&p).rematchOrFinal(free, generateur(seed.Draw()))
}

func (v *vue) rematchOrFinal(free []Ref, rng *rand.Rand) []Pairing {
	for l := 0; l < v.Rules.Lives; l++ {
		pairs, _ := v.pairGroup(v.groupe(free, l), rng, true)
		if len(pairs) > 0 {
			return []Pairing{{Kind: PairMatch, Label: Label{Kind: LabelRematch}, A: pairs[0][0], B: pairs[0][1], Length: v.swissLength()}}
		}
	}
	fr := v.leastLosses(free)
	return []Pairing{{Kind: PairMatch, Label: Label{Kind: LabelFinal}, A: fr[0], B: fr[1], Length: v.swissLength()}}
}

// PairSwissOrphans : les ORPHELINS d'une ronde ouverte — des joueurs prévus contre un joueur
// qui a joué autre chose ou qui est parti — appariés entre eux, groupe par groupe, sur
// l'instantané p d'avant la ronde, dans la limite de budget matchs. length est la longueur
// d'aujourd'hui. Un orphelin seul attend la ronde suivante.
func PairSwissOrphans(p Phase, r int, orphans []Ref, budget, length int, seed Seed) []Pairing {
	v := voir(&p)
	rng := generateur(seed.Round(r))
	var acts []Pairing
	for l := 0; l < v.Rules.Lives && budget > 0; l++ {
		pairs, _ := v.pairGroup(v.groupe(orphans, l), rng, false)
		for _, pr := range pairs {
			if budget <= 0 {
				break
			}
			acts = append(acts, Pairing{Kind: PairMatch, Label: Label{Kind: LabelRound, N: r}, Round: r, A: pr[0], B: pr[1], Length: length})
			budget--
		}
	}
	return acts
}
