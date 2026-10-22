package rules

import "sort"

// ---- Classement : par phase, par section, général ----
//
// Aucun départage, par principe : à égalité, les joueurs restent ex æquo (même rang, le rang
// suivant saute d'autant) ou se départagent sur le tapis, par un barrage. L'ordre d'affichage
// entre ex æquo est celui des entrants (ou des identifiants pour les poules), qui n'est pas un
// départage.

// Standing est une ligne de classement : le rang (partagé par les ex æquo) et la note codée.
type Standing struct {
	Player Ref  `json:"player"`
	Rank   int  `json:"rank"`
	Note   Note `json:"note"`
}

// Rank : le classement interne d'une phase. Suisse et GSL : vainqueur, finaliste (dernier
// éliminé), joueurs en vie par vies restantes, puis par victoires au moment de l'élimination.
// Tableau : par section et tour atteints (grande finale, principal, consolante, dernière
// chance). Poules : qualifiés d'abord, puis par victoires de poule. Un retiré est classé sur son
// parcours, avec la note NoteWithdrawn.
func Rank(p Phase) []Standing {
	v := voir(&p)
	switch p.Format {
	case FormatBracket, FormatLivesBracket:
		return v.rankBracket()
	case FormatRoundRobin:
		return v.rankPools()
	}
	return v.rankLives()
}

// rangs numérote une liste triée : même score, même rang ; le rang suivant saute d'autant.
func rangs[T comparable](players []Ref, score []T, notes []Note) []Standing {
	out := make([]Standing, len(players))
	rank := 1
	for i := range players {
		if i > 0 && score[i] != score[i-1] {
			rank = i + 1
		}
		out[i] = Standing{Player: players[i], Rank: rank, Note: notes[i]}
	}
	return out
}

func (v *vue) rankLives() []Standing {
	type sc struct {
		p     Ref
		score float64
		note  Note
	}
	var list []sc
	alive := v.alive()
	for _, e := range v.Entrants {
		p := e.Player
		val := float64(e.Wins)
		note := Note{Kind: NoteRecord, Wins: e.Wins, Losses: e.Losses}
		if rest := v.remaining(p); rest > 0 {
			val += 1000 + float64(rest)
			note = Note{Kind: NoteAlive, Lives: rest}
			if len(alive) == 1 {
				note = Note{Kind: NoteWinner}
			}
		}
		if v.withdrawn[p] {
			// classé comme un joueur éliminé à cet instant, par ses victoires
			val = float64(e.Wins)
			restantes := e.Lives - e.Losses
			if restantes < 0 {
				restantes = 0
			}
			note = Note{Kind: NoteWithdrawn, Wins: e.Wins, Losses: e.Losses, Lives: restantes}
		}
		list = append(list, sc{p, val, note})
	}
	if len(alive) == 1 && len(v.Eliminated) > 0 { // finaliste = dernier éliminé
		last := v.Eliminated[len(v.Eliminated)-1]
		for i := range list {
			if list[i].p == last {
				list[i].score, list[i].note = 999, Note{Kind: NoteFinalist}
			}
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].score > list[b].score })
	ps, scores, notes := make([]Ref, len(list)), make([]float64, len(list)), make([]Note, len(list))
	for i, x := range list {
		ps[i], scores[i], notes[i] = x.p, x.score, x.note
	}
	return rangs(ps, scores, notes)
}

func (v *vue) rankBracket() []Standing {
	prio := map[string]int{SectionGrandFinal: 4, SectionMain: 3, SectionConso: 2, SectionLast: 1, SectionSE: 3}
	type sc struct {
		p     Ref
		score int
		note  Note
	}
	if !v.Drawn {
		var out []Standing
		for _, e := range v.Entrants {
			out = append(out, Standing{Player: e.Player, Rank: 1, Note: Note{Kind: NoteAwaitingDraw}})
		}
		return out
	}
	scores := map[Ref]sc{}
	for _, e := range v.Entrants {
		scores[e.Player] = sc{e.Player, -1, Note{Kind: NoteUnranked}}
	}
	// Pour chaque joueur, la « sortie » de plus faible priorité (la dernière section jouée). Un
	// match perdu par forfait par un retiré est une sortie comme une autre ; tant que son
	// adversaire n'est pas connu, le match qui l'attend en tient lieu.
	sortie := func(p Ref, sec *Section, depth int, g Node) {
		cur := scores[p]
		sc2 := prio[sec.Kind]*1000 + depth
		if cur.score < 0 || prio[sec.Kind] < cur.score/1000 || (prio[sec.Kind] == cur.score/1000 && sc2 > cur.score) {
			scores[p] = sc{p, sc2, Note{Kind: NoteSectionExit, Section: sec.Name, Sub: labelPtr(g.Label)}}
		}
	}
	for si := range v.Sections {
		sec := &v.Sections[si]
		for r, idx := range sec.Rounds {
			for _, i := range idx {
				g := sec.Nodes[i]
				if g.Skipped {
					continue
				}
				if !g.Done {
					for _, p := range g.Players {
						if p != "" && p != Bye && v.withdrawn[p] {
							sortie(p, sec, r, g)
						}
					}
					continue
				}
				if g.Walkover && !v.withdrawn[g.Loser] {
					continue
				}
				sortie(g.Loser, sec, r, g)
			}
		}
	}
	for si := range v.Sections {
		sec := &v.Sections[si]
		if len(sec.Nodes) == 0 {
			continue
		}
		last := sec.Nodes[len(sec.Nodes)-1]
		if sec.Kind == SectionGrandFinal {
			// vainqueur de la grande finale : le gagnant du dernier match joué de la section
			for i := len(sec.Nodes) - 1; i >= 0; i-- {
				if sec.Nodes[i].Done && !sec.Nodes[i].Skipped {
					last = sec.Nodes[i]
					break
				}
			}
		}
		if last.Done && !last.Skipped && last.Winner != Bye {
			w := last.Winner
			cur := scores[w]
			sc2 := prio[sec.Kind]*1000 + 999
			if cur.score < 0 || sc2 > cur.score || sec.Kind == SectionGrandFinal {
				scores[w] = sc{w, sc2, Note{Kind: NoteSectionWinner, Section: sec.Name}}
			}
			if sec.Kind == SectionGrandFinal { // le perdant de la grande finale est finaliste
				l := last.Loser
				scores[l] = sc{l, prio[SectionGrandFinal]*1000 + 998, Note{Kind: NoteFinalist}}
			}
		}
	}
	var list []sc
	for _, e := range v.Entrants {
		x := scores[e.Player]
		if v.withdrawn[e.Player] {
			x.note = Note{Kind: NoteWithdrawn, Section: x.note.Section, Sub: x.note.Sub}
		}
		if x.score < 0 {
			x.score = 0
			if x.note.Kind == NoteUnranked {
				x.note = Note{Kind: NoteRunning}
				x.score = 5000
			}
		}
		list = append(list, x)
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].score > list[b].score })
	ps, scs, notes := make([]Ref, len(list)), make([]int, len(list)), make([]Note, len(list))
	for i, x := range list {
		ps[i], scs[i], notes[i] = x.p, x.score, x.note
	}
	return rangs(ps, scs, notes)
}

func (v *vue) rankPools() []Standing {
	qual := refSet(v.repecher(v.qualifiedBase()))
	score := map[Ref]int{}
	note := map[Ref]Note{}
	for i := range v.Sections {
		sec := v.Sections[i]
		if sec.Kind != SectionPool {
			continue
		}
		for p, w := range PoolWins(sec) {
			score[p] = w
			note[p] = Note{Kind: NotePoolRecord, Section: sec.Name, Wins: w}
			if v.withdrawn[p] {
				// jamais parmi les qualifiés : classé sur ses victoires
				note[p] = Note{Kind: NoteWithdrawn, Section: sec.Name, Wins: w}
				continue
			}
			if qual[p] {
				score[p] += 100
				n := note[p]
				n.Qualified = true
				note[p] = n
			}
		}
	}
	ids := make([]Ref, len(v.Entrants))
	for i, e := range v.Entrants {
		ids[i] = e.Player
	}
	ids = sortedRefs(ids)
	sort.SliceStable(ids, func(i, j int) bool { return score[ids[i]] > score[ids[j]] })
	scs, notes := make([]int, len(ids)), make([]Note, len(ids))
	for i, p := range ids {
		scs[i], notes[i] = score[p], note[p]
	}
	return rangs(ids, scs, notes)
}

// RankSection : le classement PROPRE d'une section (principal, consolante, dernière chance,
// grande finale, groupe GSL, poule, barrage), par tour atteint dans cette section — une
// consolante a ses prix, donc son classement, où son vainqueur est premier. Poules et barrages :
// par victoires. withdrawn : les retirés du tournoi (un retiré qui attend son adversaire n'est
// plus « en course »).
func RankSection(sec Section, withdrawn []Ref) []Standing {
	retire := refSet(withdrawn)
	var ordre []Ref // ordre de rencontre dans le graphe : stable, sans carte
	vu := map[Ref]bool{}
	for i := range sec.Nodes {
		for _, p := range sec.Nodes[i].Players {
			if p == "" || p == Bye || vu[p] {
				continue
			}
			vu[p] = true
			ordre = append(ordre, p)
		}
	}
	profondeur := map[Ref]int{}
	notes := map[Ref]Note{}
	if sec.Kind == SectionPool || sec.Kind == SectionBarrage {
		for i := range sec.Nodes {
			g := sec.Nodes[i]
			if g.Done && !g.Skipped && g.Winner != Bye {
				profondeur[g.Winner]++
			}
		}
		for _, p := range ordre {
			notes[p] = Note{Kind: NotePoolRecord, Section: sec.Name, Wins: profondeur[p]}
		}
	} else {
		for r, idx := range sec.Rounds {
			for _, i := range idx {
				g := sec.Nodes[i]
				if !g.Done && !g.Skipped {
					for _, p := range g.Players {
						if p != "" && p != Bye && retire[p] {
							profondeur[p] = r
							notes[p] = Note{Kind: NoteWithdrawn, Section: sec.Name, Sub: labelPtr(g.Label)}
						}
					}
					continue
				}
				if g.Skipped || g.Loser == Bye || g.Loser == "" {
					continue
				}
				profondeur[g.Loser] = r
				notes[g.Loser] = Note{Kind: NoteSectionExit, Section: sec.Name, Sub: labelPtr(g.Label)}
				if g.Walkover {
					notes[g.Loser] = Note{Kind: NoteForfeit}
				}
			}
		}
		for i := len(sec.Nodes) - 1; i >= 0; i-- {
			g := sec.Nodes[i]
			if g.Done && !g.Skipped && g.Winner != Bye && g.Winner != "" {
				profondeur[g.Winner] = len(sec.Rounds)
				notes[g.Winner] = Note{Kind: NoteSectionWinner, Section: sec.Name}
				break
			}
		}
		for _, p := range ordre {
			if _, sorti := profondeur[p]; !sorti {
				profondeur[p] = len(sec.Rounds)
				notes[p] = Note{Kind: NoteRunning}
			}
		}
	}
	list := append([]Ref(nil), ordre...)
	sort.SliceStable(list, func(a, b int) bool { return profondeur[list[a]] > profondeur[list[b]] })
	scs, ns := make([]int, len(list)), make([]Note, len(list))
	for i, p := range list {
		scs[i], ns[i] = profondeur[p], notes[p]
	}
	return rangs(list, scs, ns)
}

// MergeRankings : le classement général à partir des classements de phase, donnés de la
// PREMIÈRE phase à la dernière. La dernière phase passe devant ; un joueur déjà classé dans une
// phase plus avancée n'est pas reclassé ; les rangs sont renumérotés, ex æquo conservés.
func MergeRankings(byPhase [][]Standing) []Standing {
	var out []Standing
	seen := map[Ref]bool{}
	offset := 0
	for i := len(byPhase) - 1; i >= 0; i-- {
		r := byPhase[i]
		for _, rk := range r {
			if seen[rk.Player] {
				continue
			}
			seen[rk.Player] = true
			out = append(out, Standing{Player: rk.Player, Rank: rk.Rank + offset, Note: rk.Note})
		}
		offset += len(r)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Rank < out[b].Rank })
	pos, prev, prevRank := 0, -1, 0
	for i := range out {
		if out[i].Rank != prev {
			prev = out[i].Rank
			prevRank = pos + 1
		}
		out[i].Rank = prevRank
		pos++
	}
	return out
}
