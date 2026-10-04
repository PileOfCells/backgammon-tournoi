package tournoi

import "strings"

// ---- Statut d'un joueur : « est-ce que je joue ? » ----
//
// Le classement ne répond pas à cette question. Une note de classement situe un joueur ; elle
// ne dit pas s'il a encore un match : un perdant du principal reversé en consolante porte la
// note section_exit du principal tant que sa consolante n'est pas jouée, et un survivant du
// suisse qui manquera la coupe y est encore « en vie ». La réponse se lit dans les phases
// elles-mêmes — vies restantes, places de graphe encore à jouer, entrants de la phase suivante —
// et c'est le moteur qui la donne, parce que ce sont ses règles : un hôte qui la recalculerait
// recoderait le passage de phase, les exemptions et le repêchage (N26).
//
// Le statut est DÉRIVÉ de l'état, comme State.Infos : il ne dépend que du journal, jamais de
// l'heure ni d'une proposition non confirmée. Le seul cas où une proposition compte est le
// repêchage : un candidat proposé n'est ni qualifié ni éliminé tant que le TD n'a pas tranché.

// StatusKind est le code du statut d'un joueur.
type StatusKind string

const (
	// StatusPlaying : encore en course dans la phase en cours. Dans un tableau, Section, Round
	// et Label situent son prochain match.
	StatusPlaying StatusKind = "playing"
	// StatusBye : exempté — ne joue pas maintenant et entre plus tard. Tableau : il n'a encore
	// joué aucun match de la phase et le tirage (ou un forfait) l'a avancé jusqu'au match situé
	// par Section, Round (tour de la section, à partir de 1) et Label. Suisse par rondes : bye de
	// la ronde en cours ; il rejoue à la ronde Round (Label = LabelRound).
	StatusBye StatusKind = "bye"
	// StatusQualified : entre dans la phase Phase — la phase en cours est terminée et il fait
	// partie de ceux qui passent, ou le passage est fait et la phase n'est ni tirée ni commencée.
	// Un repêché (N26) est qualifié ; le retiré qu'il remplace ne l'est pas.
	StatusQualified StatusKind = "qualified"
	// StatusUndecided : son sort dans la phase Phase n'est pas encore fixé. Soit son parcours
	// dans la phase en cours est terminé mais Phase prend les N premiers (entrée top:N), connus à
	// la fin seulement ; soit il est candidat à un repêchage proposé que le TD n'a pas tranché.
	StatusUndecided StatusKind = "undecided"
	// StatusEliminated : plus aucun match à jouer dans le tournoi. Phase : la dernière phase
	// où il est entré.
	StatusEliminated StatusKind = "eliminated"
	// StatusWinner : la dernière phase est terminée (ou le tournoi clos) et il est premier.
	StatusWinner StatusKind = "winner"
	// StatusWithdrawn : retiré. Phase : la dernière phase où il est entré. Prime sur tout le
	// reste : un qualifié retiré n'est pas qualifié.
	StatusWithdrawn StatusKind = "withdrawn"
	// StatusNotEntered : inscrit, engagé dans aucune phase (un retardataire) ; State.Infos dit
	// où il entrera.
	StatusNotEntered StatusKind = "not_entered"
)

// PlayerStatus est le statut d'un joueur : un code et de quoi le situer, jamais une phrase.
type PlayerStatus struct {
	Player PlayerID   `json:"player"`
	Kind   StatusKind `json:"kind"`
	// Phase : la phase dont parle le statut — en cours (en jeu, exempté), à venir (qualifié,
	// indécis), dernière jouée (éliminé, retiré, vainqueur).
	Phase   int    `json:"phase,omitempty"`
	Section string `json:"section,omitempty"`
	Round   int    `json:"round,omitempty"`
	Label   Label  `json:"label,omitempty"`
}

// Statuses donne le statut de chaque inscrit, dans l'ordre d'inscription. Rien avant la
// création du tournoi.
func (s *State) Statuses() []PlayerStatus {
	v := s.statusView()
	if v == nil {
		return nil
	}
	out := make([]PlayerStatus, 0, len(s.Order))
	for _, p := range s.Order {
		out = append(out, v.of(p))
	}
	return out
}

// StatusOf donne le statut d'un joueur. Un identifiant inconnu est « non engagé ».
func (s *State) StatusOf(p PlayerID) PlayerStatus {
	v := s.statusView()
	if v == nil {
		return PlayerStatus{Player: p, Kind: StatusNotEntered}
	}
	return v.of(p)
}

// statusView : ce que tous les statuts partagent, calculé une fois.
type statusView struct {
	s       *State
	cur     *PhaseState
	done    bool // la phase en cours n'a plus rien à jouer
	hasNext bool
	// last : la dernière phase où chaque joueur est entré.
	last map[PlayerID]int
	// next : les entrants qu'aurait la phase suivante si on y passait maintenant (phase finie).
	next map[PlayerID]bool
	// undecided : les candidats d'un repêchage proposé, et la phase où ils entreraient.
	undecided map[PlayerID]int
	// winners : premiers du classement, quand la dernière phase est finie ou le tournoi clos.
	winners map[PlayerID]bool
	// champion : le vainqueur du tableau en cours, s'il est connu avant la fin de la phase
	// (consolante encore en cours).
	champion PlayerID
}

func (s *State) statusView() *statusView {
	cur := s.phase()
	if cur == nil {
		return nil
	}
	v := &statusView{s: s, cur: cur, last: map[PlayerID]int{}, next: map[PlayerID]bool{},
		undecided: map[PlayerID]int{}, winners: map[PlayerID]bool{}}
	for _, ph := range s.Phases {
		for _, p := range ph.Entrants {
			v.last[p] = ph.Index
		}
	}
	v.hasNext = s.Current+1 < len(s.Config.Phases)
	v.done = s.Finished || s.phaseDone(cur)
	repechage := func(ph *PhaseState, into int) {
		for _, a := range s.proposeRepechages(ph) {
			v.undecided[a.B] = into
		}
	}
	switch {
	case s.Finished || (v.done && !v.hasNext):
		rk := s.Final
		if rk == nil {
			rk = s.Ranking()
		}
		for _, r := range rk {
			if r.Rank == 1 {
				v.winners[r.Player] = true
			}
		}
	case v.done:
		// Le passage de phase lui-même (EvNextPhase) : enterFrom sur une phase neuve, sans
		// toucher à l'état.
		next := newPhaseState(s.Current+1, s.Config.Phases[s.Current+1])
		s.enterFrom(next, cur)
		for _, p := range next.Entrants {
			v.next[p] = true
		}
		repechage(cur, s.Current+1)
	default:
		if cur.Index > 0 && !cur.Drawn && !cur.Started {
			repechage(s.phaseOf(cur.Index-1), cur.Index)
		}
		if cur.Cfg.Kind == KindBracket || cur.Cfg.Kind == KindLivesBracket {
			v.champion = bracketChampion(cur)
		}
	}
	return v
}

func (v *statusView) of(p PlayerID) PlayerStatus {
	s, cur := v.s, v.cur
	last, entered := v.last[p]
	switch {
	case s.Withdrawn[p]:
		return PlayerStatus{Player: p, Kind: StatusWithdrawn, Phase: last}
	case v.winners[p]:
		return PlayerStatus{Player: p, Kind: StatusWinner, Phase: last}
	case s.Finished:
		if !entered {
			return PlayerStatus{Player: p, Kind: StatusNotEntered}
		}
		return PlayerStatus{Player: p, Kind: StatusEliminated, Phase: last}
	case v.next[p]:
		return PlayerStatus{Player: p, Kind: StatusQualified, Phase: s.Current + 1}
	}
	if ph, ok := v.undecided[p]; ok {
		return PlayerStatus{Player: p, Kind: StatusUndecided, Phase: ph}
	}
	if !entered {
		return PlayerStatus{Player: p, Kind: StatusNotEntered}
	}
	if !contains(cur.Entrants, p) {
		// Sorti à une phase précédente : il ne revient que par une phase ouverte à tous.
		return v.eliminated(p, s.Current+1, last)
	}
	if v.done {
		if v.hasNext {
			return v.eliminated(p, s.Current+2, s.Current)
		}
		return PlayerStatus{Player: p, Kind: StatusEliminated, Phase: s.Current}
	}
	if cur.Index > 0 && !cur.Drawn && !cur.Started {
		return PlayerStatus{Player: p, Kind: StatusQualified, Phase: cur.Index}
	}
	switch cur.Cfg.Kind {
	case KindSwissLives, KindGSL:
		if s.remainingLives(cur, p) == 0 {
			return v.phaseOver(p)
		}
		if cur.Cfg.Mode == "rounds" && cur.Round > 0 && !s.busy(p) && containsRound(cur.ByeRounds[p], cur.Round) {
			r := cur.Round + 1
			return PlayerStatus{Player: p, Kind: StatusBye, Phase: cur.Index, Round: r, Label: Label{Kind: LabelRound, N: r}}
		}
	case KindBracket, KindLivesBracket:
		if !cur.Drawn {
			break
		}
		sec, idx := pendingMatch(cur, p)
		if sec == nil {
			if p == v.champion {
				if v.hasNext {
					return PlayerStatus{Player: p, Kind: StatusQualified, Phase: s.Current + 1}
				}
				return PlayerStatus{Player: p, Kind: StatusWinner, Phase: s.Current}
			}
			return v.phaseOver(p)
		}
		g := sec.Matches[idx]
		kind := StatusPlaying
		if g.MatchID == "" && cur.Wins[p]+cur.Losses[p] == 0 && advancedUnplayed(cur, p) {
			kind = StatusBye
		}
		return PlayerStatus{Player: p, Kind: kind, Phase: cur.Index, Section: sec.Name,
			Round: roundOf(sec, idx) + 1, Label: g.Label}
	}
	return PlayerStatus{Player: p, Kind: StatusPlaying, Phase: cur.Index}
}

// phaseOver : le joueur n'a plus de match dans la phase en cours, qui n'est pas finie. La
// phase suivante dit ce que cela vaut : rien n'est joué si elle prend les N premiers, tout est
// acquis si elle prend tout le monde, et c'est la fin sinon.
func (v *statusView) phaseOver(p PlayerID) PlayerStatus {
	s := v.s
	if !v.hasNext {
		return PlayerStatus{Player: p, Kind: StatusEliminated, Phase: s.Current}
	}
	if strings.HasPrefix(s.Config.Phases[s.Current+1].Entry, "top:") {
		return PlayerStatus{Player: p, Kind: StatusUndecided, Phase: s.Current + 1}
	}
	return v.eliminated(p, s.Current+1, s.Current)
}

// eliminated : éliminé à la phase last, sauf si une phase à partir de from prend tout le monde
// (entrée « all ») — il y entrera.
func (v *statusView) eliminated(p PlayerID, from, last int) PlayerStatus {
	for i := from; i < len(v.s.Config.Phases); i++ {
		if v.s.Config.Phases[i].Entry == "all" {
			return PlayerStatus{Player: p, Kind: StatusQualified, Phase: i}
		}
	}
	return PlayerStatus{Player: p, Kind: StatusEliminated, Phase: last}
}

// pendingMatch : le premier match de graphe encore à jouer qui attend p (le plus petit tour,
// dans l'ordre des sections), lancé ou non.
func pendingMatch(ph *PhaseState, p PlayerID) (*Section, int) {
	var best *Section
	bi, br := -1, 0
	for _, sec := range ph.Sections {
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.Done || g.Skipped || (g.Players[0] != p && g.Players[1] != p) {
				continue
			}
			if r := roundOf(sec, i); best == nil || r < br {
				best, bi, br = sec, i, r
			}
		}
	}
	return best, bi
}

// advancedUnplayed : p a franchi au moins un match du graphe sans le jouer — une exemption du
// tirage, ou le forfait d'un retiré.
func advancedUnplayed(ph *PhaseState, p PlayerID) bool {
	for _, sec := range ph.Sections {
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.Done && g.Walkover && !g.Skipped && g.MatchID == "" && g.Winner == p {
				return true
			}
		}
	}
	return false
}

// bracketChampion : le vainqueur du tableau, quand il est connu — la grande finale jouée
// jusqu'au bout, ou, sans grande finale, la finale du principal.
func bracketChampion(ph *PhaseState) PlayerID {
	var main, gf *Section
	for _, sec := range ph.Sections {
		switch sec.Kind {
		case secMain:
			main = sec
		case secGrandFinal:
			gf = sec
		}
	}
	if gf != nil {
		var w PlayerID
		for i := range gf.Matches {
			g := gf.Matches[i]
			if !g.Done {
				return ""
			}
			if !g.Skipped && g.Winner != BYE {
				w = g.Winner
			}
		}
		return w
	}
	if main == nil || len(main.Matches) == 0 {
		return ""
	}
	if g := main.Matches[len(main.Matches)-1]; g.Done && !g.Skipped && g.Winner != BYE {
		return g.Winner
	}
	return ""
}

func contains(ids []PlayerID, p PlayerID) bool {
	for _, x := range ids {
		if x == p {
			return true
		}
	}
	return false
}

func containsRound(rs []int, r int) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
