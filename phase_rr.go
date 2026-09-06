package tournoi

import (
	"fmt"
	"sort"
)

// ---- Poules toutes rondes, qualification sans départage (barrage à 2 vies entre ex æquo) ----

func groupName(i int) string { return fmt.Sprintf("Poule %c", 'A'+i) }

func (s *State) proposeRR(ph *PhaseState) []Action {
	if !ph.Drawn {
		n := len(ph.Entrants)
		if n < 2 {
			return nil
		}
		rng := s.rng()
		ids := sortedIDs(ph.Entrants)
		rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		ng := (n + ph.Cfg.GroupSize - 1) / ph.Cfg.GroupSize
		if ng < 1 {
			ng = 1
		}
		groups := make([][]PlayerID, ng)
		for i, p := range ids {
			groups[i%ng] = append(groups[i%ng], p)
		}
		return []Action{{Kind: ActDraw, Phase: ph.Index, Label: fmt.Sprintf("%d poules", ng), Draw: &Draw{Groups: groups}}}
	}
	var acts []Action
	for _, r := range s.readyFree(ph) {
		acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Section: r.sec.Name, Key: r.g.Key,
			Label: fmt.Sprintf("%s, %s", r.sec.Name, r.g.Label), A: r.g.Players[0], B: r.g.Players[1], Length: r.g.Length})
	}
	// barrages
	for _, sec := range ph.Sections {
		if sec.Kind != "poule" || !sectionDone(sec) {
			continue
		}
		tied, spots := s.rrTie(ph, sec)
		if len(tied) == 0 {
			continue
		}
		bname := "Barrage " + sec.Name
		bs := ph.section(bname)
		if bs == nil {
			acts = append(acts, Action{Kind: ActDraw, Phase: ph.Index, Section: bname,
				Label: fmt.Sprintf("Barrage %s : %d joueurs pour %d place(s)", sec.Name, len(tied), spots), Draw: &Draw{Groups: [][]PlayerID{tied}}})
			continue
		}
		acts = append(acts, s.proposeBarrage(ph, bs)...)
	}
	return acts
}

func sectionDone(sec *Section) bool {
	for i := range sec.Matches {
		if !sec.Matches[i].Done {
			return false
		}
	}
	return true
}

// rrWins : victoires de chaque joueur dans une section.
func rrWins(sec *Section) map[PlayerID]int {
	w := map[PlayerID]int{}
	for i := range sec.Matches {
		g := sec.Matches[i]
		w[g.Src[0].Player] += 0
		w[g.Src[1].Player] += 0
		if g.Done && !g.Walkover {
			w[g.Winner]++
		}
	}
	return w
}

// rrTie : joueurs à départager par barrage et nombre de places restantes dans la poule.
func (s *State) rrTie(ph *PhaseState, sec *Section) (tied []PlayerID, spots int) {
	w := rrWins(sec)
	var ids []PlayerID
	for p := range w {
		ids = append(ids, p)
	}
	ids = sortedIDs(ids)
	sort.SliceStable(ids, func(i, j int) bool { return w[ids[i]] > w[ids[j]] })
	q := ph.Cfg.Qualifiers
	if q >= len(ids) {
		return nil, 0
	}
	// place q (index q-1) et q+1 (index q) : égalité ?
	if w[ids[q-1]] != w[ids[q]] {
		return nil, 0
	}
	v := w[ids[q-1]]
	spots = q
	for _, p := range ids {
		if w[p] > v {
			spots--
		} else if w[p] == v {
			tied = append(tied, p)
		}
	}
	return tied, spots
}

// barrageLosses : défaites de chaque joueur dans une section de barrage (matchs réels).
func (s *State) barrageLosses(bs *Section) (losses map[PlayerID]int, running int) {
	losses = map[PlayerID]int{}
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Section != bs.Name || m.Status == Cancelled {
			continue
		}
		if m.Status == Running {
			running++
		} else {
			losses[m.Loser()]++
		}
	}
	return losses, running
}

func (s *State) barrageAlive(bs *Section) []PlayerID {
	losses, _ := s.barrageLosses(bs)
	var out []PlayerID
	for _, p := range bs.Players {
		if losses[p] < 2 {
			out = append(out, p)
		}
	}
	return out
}

func (s *State) proposeBarrage(ph *PhaseState, bs *Section) []Action {
	losses, running := s.barrageLosses(bs)
	alive := s.barrageAlive(bs)
	budget := len(alive) - running - bs.Spots
	if budget <= 0 {
		return nil
	}
	rng := s.rng()
	var free []PlayerID
	for _, p := range alive {
		if !s.busy(p) {
			free = append(free, p)
		}
	}
	var acts []Action
	for l := 0; l < 2 && budget > 0; l++ {
		var g []PlayerID
		for _, p := range free {
			if losses[p] == l {
				g = append(g, p)
			}
		}
		g = sortedIDs(g)
		rng.Shuffle(len(g), func(i, j int) { g[i], g[j] = g[j], g[i] })
		for i := 0; i+1 < len(g) && budget > 0; i += 2 {
			acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Section: bs.Name, Label: bs.Name, A: g[i], B: g[i+1], Length: ph.Length})
			budget--
		}
	}
	if len(acts) == 0 && running == 0 && len(free) >= 2 { // croisé : les deux ayant le moins de défaites
		fr := sortedIDs(free)
		sort.SliceStable(fr, func(i, j int) bool { return losses[fr[i]] < losses[fr[j]] })
		acts = append(acts, Action{Kind: ActStartMatch, Phase: ph.Index, Section: bs.Name, Label: bs.Name + " (croisé)", A: fr[0], B: fr[1], Length: ph.Length})
	}
	return acts
}

func (s *State) rrBarragesDone(ph *PhaseState) bool {
	for _, sec := range ph.Sections {
		if sec.Kind != "poule" {
			continue
		}
		tied, _ := s.rrTie(ph, sec)
		if len(tied) == 0 {
			continue
		}
		bs := ph.section("Barrage " + sec.Name)
		if bs == nil || len(s.barrageAlive(bs)) > bs.Spots {
			return false
		}
	}
	return true
}

func (s *State) applyRRDraw(ph *PhaseState, section string, d *Draw) error {
	if section == "" {
		for i, g := range d.Groups {
			sec := rrSection(groupName(i), g, ph.Length)
			sec.Group = i + 1
			ph.Sections = append(ph.Sections, sec)
		}
		ph.Drawn = true
		ph.resolve()
		return nil
	}
	if len(d.Groups) != 1 {
		return fmt.Errorf("barrage : un seul groupe attendu")
	}
	// places : recalculées à partir de la poule
	poule := ph.section(section[len("Barrage "):])
	if poule == nil {
		return fmt.Errorf("barrage : poule %q inconnue", section)
	}
	_, spots := s.rrTie(ph, poule)
	ph.Sections = append(ph.Sections, &Section{Name: section, Kind: "barrage", Players: d.Groups[0], Spots: spots})
	return nil
}

// rrQualified : qualifiés de chaque poule (après barrages).
func (s *State) rrQualified(ph *PhaseState) []PlayerID {
	var out []PlayerID
	for _, sec := range ph.Sections {
		if sec.Kind != "poule" {
			continue
		}
		w := rrWins(sec)
		var ids []PlayerID
		for p := range w {
			ids = append(ids, p)
		}
		ids = sortedIDs(ids)
		sort.SliceStable(ids, func(i, j int) bool { return w[ids[i]] > w[ids[j]] })
		tied, spots := s.rrTie(ph, sec)
		if len(tied) == 0 {
			out = append(out, ids[:min(ph.Cfg.Qualifiers, len(ids))]...)
			continue
		}
		v := w[tied[0]]
		for _, p := range ids {
			if w[p] > v {
				out = append(out, p)
			}
		}
		if bs := ph.section("Barrage " + sec.Name); bs != nil {
			alive := s.barrageAlive(bs)
			if len(alive) <= spots {
				out = append(out, alive...)
			}
		}
	}
	return out
}

// rrRanking : qualifiés d'abord, puis par victoires ; ex æquo partagés.
func (s *State) rrRanking(ph *PhaseState) []Rank {
	qual := map[PlayerID]bool{}
	for _, p := range s.rrQualified(ph) {
		qual[p] = true
	}
	score := map[PlayerID]int{}
	note := map[PlayerID]string{}
	for _, sec := range ph.Sections {
		if sec.Kind != "poule" {
			continue
		}
		for p, w := range rrWins(sec) {
			score[p] = w
			note[p] = fmt.Sprintf("%s, %d victoires", sec.Name, w)
			if qual[p] {
				score[p] += 100
				note[p] += ", qualifié"
			}
		}
	}
	ids := sortedIDs(ph.Entrants)
	sort.SliceStable(ids, func(i, j int) bool { return score[ids[i]] > score[ids[j]] })
	out := make([]Rank, len(ids))
	rank := 1
	for i, p := range ids {
		if i > 0 && score[p] != score[ids[i-1]] {
			rank = i + 1
		}
		out[i] = Rank{Player: p, Rank: rank, Note: note[p]}
	}
	return out
}
