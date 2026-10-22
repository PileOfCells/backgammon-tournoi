package tournoi

import (
	"sort"
	"strings"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// ---- Passage vers rules : instantanés, sections, libellés ----
//
// Les appariements et les classements sont des fonctions pures du paquet rules (Pair, Rank,
// RankSection, MergeRankings, Resolve…). Le moteur n'en est que l'ENVELOPPE : il construit
// l'instantané d'une phase depuis son état, appelle la règle, et retraduit la sortie dans ses
// types (Action, Rank, Section). Ces types restent les siens : ils voyagent dans le journal et
// dans l'état sérialisé, que rules ne connaît pas.

func ref(p PlayerID) rules.Ref { return rules.Ref(p) }

func refs(ps []PlayerID) []rules.Ref {
	if ps == nil {
		return nil
	}
	out := make([]rules.Ref, len(ps))
	for i, p := range ps {
		out[i] = rules.Ref(p)
	}
	return out
}

func ids(rs []rules.Ref) []PlayerID {
	if rs == nil {
		return nil
	}
	out := make([]PlayerID, len(rs))
	for i, r := range rs {
		out[i] = PlayerID(r)
	}
	return out
}

// ---- Libellés et notes ----

func toRulesLabel(l Label) rules.Label {
	out := rules.Label{Kind: rules.LabelKind(l.Kind), N: l.N, Losses: l.Losses, Match: l.Match, Section: l.Section,
		Text: l.Text, Players: l.Players, Spots: l.Spots}
	if l.Sub != nil {
		sub := toRulesLabel(*l.Sub)
		out.Sub = &sub
	}
	return out
}

func fromRulesLabel(l rules.Label) Label {
	out := Label{Kind: LabelKind(l.Kind), N: l.N, Losses: l.Losses, Match: l.Match, Section: l.Section,
		Text: l.Text, Players: l.Players, Spots: l.Spots}
	if l.Sub != nil {
		sub := fromRulesLabel(*l.Sub)
		out.Sub = &sub
	}
	return out
}

func fromRulesNote(n rules.Note) Note {
	out := Note{Kind: NoteKind(n.Kind), Wins: n.Wins, Losses: n.Losses, Lives: n.Lives, Section: n.Section, Qualified: n.Qualified}
	if n.Sub != nil {
		sub := fromRulesLabel(*n.Sub)
		out.Sub = &sub
	}
	return out
}

func fromStandings(st []rules.Standing) []Rank {
	if st == nil {
		return nil
	}
	out := make([]Rank, len(st))
	for i, x := range st {
		out[i] = Rank{Player: PlayerID(x.Player), Rank: x.Rank, Note: fromRulesNote(x.Note)}
	}
	return out
}

// ---- Sections ----

func toRulesSection(sec *Section) rules.Section {
	out := rules.Section{Name: sec.Name, Kind: sec.Kind, Group: sec.Group, Block: sec.Block, Rounds: sec.Rounds,
		Players: refs(sec.Players), Spots: sec.Spots}
	if len(sec.Matches) > 0 {
		out.Nodes = make([]rules.Node, len(sec.Matches))
		for i := range sec.Matches {
			g := &sec.Matches[i]
			out.Nodes[i] = rules.Node{Key: g.Key, Label: toRulesLabel(g.Label), Length: g.Length,
				Src:     [2]rules.Src{toRulesSrc(g.Src[0]), toRulesSrc(g.Src[1])},
				Players: [2]rules.Ref{ref(g.Players[0]), ref(g.Players[1])}, Match: string(g.MatchID),
				Winner: ref(g.Winner), Loser: ref(g.Loser), Done: g.Done, Walkover: g.Walkover, Skipped: g.Skipped,
				Cond: g.Cond, CondFrom: g.CondFrom, CondSide: g.CondSide}
		}
	}
	return out
}

func toRulesSrc(s Src) rules.Src {
	return rules.Src{Player: ref(s.Player), From: s.From, Section: s.Section, Loser: s.Loser}
}

func toRulesSections(secs []*Section) []rules.Section {
	out := make([]rules.Section, len(secs))
	for i, sec := range secs {
		out[i] = toRulesSection(sec)
	}
	return out
}

func fromRulesSection(sec rules.Section) *Section {
	out := &Section{Name: sec.Name, Kind: sec.Kind, Group: sec.Group, Block: sec.Block, Rounds: sec.Rounds,
		Players: ids(sec.Players), Spots: sec.Spots}
	for _, g := range sec.Nodes {
		out.Matches = append(out.Matches, GMatch{Key: g.Key, Label: fromRulesLabel(g.Label), Length: g.Length,
			Src: [2]Src{
				{Player: PlayerID(g.Src[0].Player), From: g.Src[0].From, Section: g.Src[0].Section, Loser: g.Src[0].Loser},
				{Player: PlayerID(g.Src[1].Player), From: g.Src[1].From, Section: g.Src[1].Section, Loser: g.Src[1].Loser}},
			Players: [2]PlayerID{PlayerID(g.Players[0]), PlayerID(g.Players[1])}, MatchID: MatchID(g.Match),
			Winner: PlayerID(g.Winner), Loser: PlayerID(g.Loser), Done: g.Done, Walkover: g.Walkover, Skipped: g.Skipped,
			Cond: g.Cond, CondFrom: g.CondFrom, CondSide: g.CondSide})
	}
	return out
}

func fromRulesSections(secs []rules.Section) []*Section {
	out := make([]*Section, len(secs))
	for i := range secs {
		out[i] = fromRulesSection(secs[i])
	}
	return out
}

// resolve remplit les places des graphes de la phase (rules.Resolve) et recopie l'état résolu
// dans ses sections, en place : les pointeurs que tiennent les appelants restent valides.
func (ph *PhaseState) resolve(withdrawn map[PlayerID]bool) {
	if len(ph.Sections) == 0 {
		return
	}
	res := rules.Resolve(toRulesSections(ph.Sections), withdrawnRefs(withdrawn))
	for i, sec := range ph.Sections {
		for j := range sec.Matches {
			g, r := &sec.Matches[j], &res[i].Nodes[j]
			g.Players = [2]PlayerID{PlayerID(r.Players[0]), PlayerID(r.Players[1])}
			g.Winner, g.Loser = PlayerID(r.Winner), PlayerID(r.Loser)
			g.Done, g.Walkover, g.Skipped = r.Done, r.Walkover, r.Skipped
		}
	}
}

// withdrawnRefs : les retirés, triés (l'instantané ne dépend d'aucun ordre de carte).
func withdrawnRefs(w map[PlayerID]bool) []rules.Ref {
	var out []rules.Ref
	for p, ok := range w {
		if ok {
			out = append(out, ref(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ---- Instantané d'une phase ----

func phaseRules(c PhaseConfig) rules.PhaseRules {
	return rules.PhaseRules{Lives: c.Lives, Length: c.Length, FinalLength: c.FinalLength, Lengths: c.Lengths,
		LengthLate: c.LengthLate, LateThreshold: c.LateThreshold, Mode: c.Mode, Pairing: c.Pairing,
		AvoidClubs: c.AvoidClubs, AllowRematch: c.AllowRematch, Target: c.Target, Consolation: c.Consolation,
		LastChance: c.LastChance, Reconciliation: c.Reconciliation, Recharge: c.Recharge, GroupSize: c.GroupSize,
		Qualifiers: c.Qualifiers, Seeding: c.Seeding}
}

// snapshot : l'instantané de la phase ph pour rules.Pair. free : les joueurs appariables d'un
// suisse (nil ailleurs).
func (s *State) snapshot(ph *PhaseState, free []PlayerID) rules.Phase {
	return s.instantané(ph, free, true)
}

// bilan : l'instantané allégé que lisent PhaseDone, SwissLength, Rank et PoolQualified — sans les
// adversaires ni les joueurs occupés, qui ne servent qu'à apparier. Il est construit à chaque
// proposition et à chaque classement : ce qu'aucune de ces règles ne lit n'y est pas copié.
func (s *State) bilan(ph *PhaseState) rules.Phase { return s.instantané(ph, nil, false) }

func (s *State) instantané(ph *PhaseState, free []PlayerID, apparier bool) rules.Phase {
	p := rules.Phase{Format: rules.Format(ph.Cfg.Kind), Name: PhaseName(ph.Cfg), Rules: phaseRules(ph.Cfg),
		Eliminated: refs(ph.ElimOrder), Withdrawn: withdrawnRefs(s.Withdrawn), Drawn: ph.Drawn, Round: ph.Round,
		Length: ph.Length, Running: s.runningInPhase(ph), Free: refs(free)}
	// Seul un suisse lit les adversaires ; seules les poules lisent les barrages.
	suisse, poules := apparier && ph.Cfg.Kind == KindSwissLives, ph.Cfg.Kind == KindRoundRobin
	p.Entrants = make([]rules.Entrant, len(ph.Entrants))
	for i, id := range ph.Entrants {
		e := rules.Entrant{Player: ref(id), Lives: ph.Lives[id], Wins: ph.Wins[id], Losses: ph.Losses[id],
			Byes: ph.Byes[id]}
		if suisse {
			e.Opponents = refs(ph.Opponents[id])
		}
		if pl := s.Players[id]; pl != nil {
			e.Club, e.Rating = pl.Club, pl.Rating
		}
		p.Entrants[i] = e
	}
	p.Sections = toRulesSections(ph.Sections)
	for _, id := range s.MatchOrder {
		if !apparier && !poules {
			break
		}
		m := s.Matches[id]
		if apparier && m.Status == Running {
			p.Busy = append(p.Busy, ref(m.A), ref(m.B))
		}
		if poules && m.Status != Cancelled && strings.HasPrefix(m.Section, barragePrefix) {
			p.Barrages = append(p.Barrages, m.rulesMatch())
		}
	}
	if apparier && len(s.ailleurs) > 0 {
		var ail []PlayerID
		for q := range s.ailleurs {
			ail = append(ail, q)
		}
		p.Elsewhere = refs(sortedIDs(ail))
	}
	if len(ph.Repechages) > 0 {
		p.Repechages = make(map[rules.Ref]rules.Ref, len(ph.Repechages))
		for a, b := range ph.Repechages {
			p.Repechages[ref(a)] = ref(b)
		}
	}
	return p
}

// seed : la graine des propositions à l'état courant.
func (s *State) seed() rules.Seed {
	return rules.Seed{Base: s.Seed, Events: s.NEvents, Phase: s.Current}
}

// actions : les propositions de rules, en actions du moteur. Le tirage d'un tableau ou d'un bloc
// GSL porte les vies d'entrée (information) ; celui de poules ou d'un barrage, non.
func (s *State) actions(ph *PhaseState, prs []rules.Pairing) []Action {
	if len(prs) == 0 {
		return nil
	}
	out := make([]Action, 0, len(prs))
	for _, p := range prs {
		a := Action{Phase: ph.Index, Section: p.Section, Key: p.Key, Label: fromRulesLabel(p.Label), Round: p.Round,
			A: PlayerID(p.A), B: PlayerID(p.B), Length: p.Length}
		switch p.Kind {
		case rules.PairMatch:
			a.Kind = ActStartMatch
		case rules.PairBye:
			a.Kind = ActBye
		case rules.PairDraw:
			a.Kind = ActDraw
			a.Draw = &Draw{Slots: ids(p.Slots), Groups: groupIDs(p.Groups)}
			if ph.Cfg.Kind != KindRoundRobin {
				a.Draw.Lives = copyLives(ph)
			}
		}
		out = append(out, a)
	}
	return out
}

func groupIDs(gs [][]rules.Ref) [][]PlayerID {
	if gs == nil {
		return nil
	}
	out := make([][]PlayerID, len(gs))
	for i, g := range gs {
		out[i] = ids(g)
	}
	return out
}

func copyLives(ph *PhaseState) map[PlayerID]int {
	out := map[PlayerID]int{}
	for _, p := range ph.Entrants {
		out[p] = ph.Lives[p]
	}
	return out
}

// RulesPhase : l'instantané de la phase i tel que rules le lit (rules.Pair, rules.Rank,
// rules.RemainingMatches), pour un hôte qui tient un State et veut appeler les règles lui-même.
// nil si la phase n'existe pas. Les joueurs appariables d'un suisse (Free) sont ceux de l'instant
// du dernier événement.
func (s *State) RulesPhase(i int) *rules.Phase {
	ph := s.phaseOf(i)
	if ph == nil {
		return nil
	}
	var free []PlayerID
	if ph.Cfg.Kind == KindSwissLives {
		v := *s
		v.clock = s.Last
		r := 0
		if ph.Cfg.Mode == "rounds" {
			r = ph.Round + 1
		}
		free = v.free(ph, r)
	}
	p := s.snapshot(ph, free)
	return &p
}
