package tournoi

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// State est l'état complet d'un tournoi, reconstruit à partir du journal.
type State struct {
	Config    Config               `json:"config"`
	Seed      int64                `json:"seed"`
	Players   map[PlayerID]*Player `json:"players"`
	Order     []PlayerID           `json:"order"` // ordre d'inscription
	Withdrawn map[PlayerID]bool    `json:"withdrawn,omitempty"`
	// WithdrawAfter : retraits différés. Le joueur a demandé à partir mais finit le match qu'il
	// joue ; il n'est plus apparié (il est occupé), et le retrait devient effectif dès que ce
	// match est terminé. Un retrait différé sans match en cours est un retrait immédiat.
	WithdrawAfter map[PlayerID]bool  `json:"withdraw_after,omitempty"`
	Matches       map[MatchID]*Match `json:"matches"`
	MatchOrder    []MatchID          `json:"match_order"`
	Phases        []*PhaseState      `json:"phases"`
	Current       int                `json:"current"` // index de la phase en cours (-1 avant création)
	Finished      bool               `json:"finished"`
	Final         []Rank             `json:"final,omitempty"`
	NEvents       int                `json:"n_events"`
	Last          time.Time          `json:"last"`
	Warnings      []Warning          `json:"warnings,omitempty"`
	nextID        int
}

// PhaseState est l'état d'une phase.
type PhaseState struct {
	Index     int                     `json:"index"`
	Cfg       PhaseConfig             `json:"cfg"`
	Entrants  []PlayerID              `json:"entrants"`
	Lives     map[PlayerID]int        `json:"lives"` // vies à l'entrée de la phase
	Losses    map[PlayerID]int        `json:"losses"`
	Wins      map[PlayerID]int        `json:"wins"`
	Byes      map[PlayerID]int        `json:"byes"`
	Opponents map[PlayerID][]PlayerID `json:"opponents"`
	ElimOrder []PlayerID              `json:"elim_order"` // ordre d'élimination
	Round     int                     `json:"round"`      // rondes synchrones / blocs
	Drawn     bool                    `json:"drawn"`
	Done      bool                    `json:"done"`
	Sections  []*Section              `json:"sections,omitempty"`
	Length    int                     `json:"length"` // longueur courante des matchs
	Started   bool                    `json:"started"`
}

// Section est un graphe de matchs dont les places se remplissent par les résultats
// (tableau, groupe GSL, poule, barrage).
type Section struct {
	Name    string     `json:"name"`
	Kind    string     `json:"kind"` // secMain, secConso, secLast, secGrandFinal, secKindGSL, secKindSE, secKindPool, secKindBarrage
	Group   int        `json:"group,omitempty"`
	Block   int        `json:"block,omitempty"`
	Matches []GMatch   `json:"matches"`
	Rounds  [][]int    `json:"rounds,omitempty"`  // indices de matchs par tour (affichage)
	Players []PlayerID `json:"players,omitempty"` // barrage : joueurs concernés (appariement dynamique)
	Spots   int        `json:"spots,omitempty"`   // barrage : nombre de places à attribuer
}

// GMatch est un match d'un graphe : ses deux places viennent de joueurs fixés ou d'autres matchs.
type GMatch struct {
	Key      string      `json:"key"`
	Label    Label       `json:"label,omitempty"`
	Length   int         `json:"length"`
	Src      [2]Src      `json:"src"`
	Players  [2]PlayerID `json:"players"`
	MatchID  MatchID     `json:"match_id,omitempty"`
	Winner   PlayerID    `json:"winner,omitempty"`
	Loser    PlayerID    `json:"loser,omitempty"`
	Done     bool        `json:"done"`
	Walkover bool        `json:"walkover,omitempty"`
	Skipped  bool        `json:"skipped,omitempty"`   // match conditionnel non joué (recharge)
	CondFrom int         `json:"cond_from,omitempty"` // index du match dont dépend l'existence (recharge)
	CondSide int         `json:"cond_side,omitempty"` // le match existe si le vainqueur de CondFrom est son joueur CondSide
	Cond     bool        `json:"cond,omitempty"`
}

// Src est l'origine d'une place : un joueur, ou le vainqueur/perdant d'un match (index dans la
// section Section, ou la même section si vide).
type Src struct {
	Player  PlayerID `json:"player,omitempty"`
	From    int      `json:"from"`
	Section string   `json:"section,omitempty"`
	Loser   bool     `json:"loser,omitempty"`
}

func newPhaseState(i int, cfg PhaseConfig) *PhaseState {
	return &PhaseState{Index: i, Cfg: cfg, Lives: map[PlayerID]int{}, Losses: map[PlayerID]int{},
		Wins: map[PlayerID]int{}, Byes: map[PlayerID]int{}, Opponents: map[PlayerID][]PlayerID{}, Length: cfg.Length}
}

// Replay reconstruit l'état à partir du journal.
func Replay(j Journal) (*State, error) {
	s := newState()
	for i := range j {
		// Un journal antérieur aux codes structurés est converti à la lecture, pour que le
		// moteur ne voie jamais de libellé texte (voir upgrade dans events.go).
		if err := s.Apply(j[i].upgraded()); err != nil {
			return s, fmt.Errorf("événement %d (%s) : %w", i, j[i].Kind, err)
		}
	}
	return s, nil
}

// New crée l'état initial d'un tournoi et l'événement de création correspondant.
func New(cfg Config, seed int64, now time.Time) (*State, Event, error) {
	if err := cfg.Validate(); err != nil {
		return nil, Event{}, err
	}
	ev := Event{Version: JournalVersion, Kind: EvCreated, Time: now, Config: &cfg, Seed: seed}
	s := newState()
	if err := s.Apply(ev); err != nil {
		return nil, ev, err
	}
	return s, ev, nil
}

// newState : un état vide, toutes les cartes initialisées.
func newState() *State {
	return &State{Players: map[PlayerID]*Player{}, Withdrawn: map[PlayerID]bool{},
		WithdrawAfter: map[PlayerID]bool{}, Matches: map[MatchID]*Match{}, Current: -1}
}

// seen enregistre qu'un événement a été appliqué : compteur et horodatage. Les branches d'Apply
// qui sortent tôt passent par là pour ne pas fausser le générateur (rng dépend de NEvents).
func (s *State) seen(ev Event) error {
	s.NEvents++
	if ev.Time.After(s.Last) {
		s.Last = ev.Time
	}
	return nil
}

// promoteDeferred : un retrait différé devient effectif dès que le joueur n'a plus de match en
// cours. Appelé après chaque résultat.
func (s *State) promoteDeferred() {
	for p := range s.WithdrawAfter {
		if s.busy(p) {
			continue
		}
		delete(s.WithdrawAfter, p)
		s.Withdrawn[p] = true
	}
}

func (s *State) nextMatchID() MatchID { return MatchID(fmt.Sprintf("M%d", s.nextID+1)) }

// rng renvoie un générateur déterministe pour la proposition courante.
func (s *State) rng() *rand.Rand {
	return rand.New(rand.NewSource(s.Seed*1000003 + int64(s.NEvents)*7919 + int64(s.Current)))
}

func (s *State) phase() *PhaseState {
	if s.Current < 0 || s.Current >= len(s.Phases) {
		return nil
	}
	return s.Phases[s.Current]
}

// Apply applique un événement à l'état.
func (s *State) Apply(ev Event) error {
	if ev.Kind != EvCreated && s.Current < 0 {
		return fmt.Errorf("tournoi non créé")
	}
	switch ev.Kind {
	case EvCreated:
		if ev.Config == nil {
			return fmt.Errorf("created sans config")
		}
		cfg := *ev.Config
		if err := cfg.Validate(); err != nil {
			return err
		}
		s.Config, s.Seed = cfg, ev.Seed
		s.Phases = []*PhaseState{newPhaseState(0, cfg.Phases[0])}
		s.Current = 0
	case EvPlayerAdded:
		if ev.Player == nil || ev.Player.ID == "" {
			return fmt.Errorf("joueur sans identifiant")
		}
		if _, ok := s.Players[ev.Player.ID]; !ok {
			s.Order = append(s.Order, ev.Player.ID)
		}
		p := *ev.Player
		s.Players[p.ID] = &p
		delete(s.Withdrawn, p.ID)
		ph := s.phase()
		if ph.Index == 0 && !ph.Drawn && (ph.Cfg.Kind == KindSwissLives || !ph.Started) {
			s.enter(ph, p.ID, livesFor(ph.Cfg)) // retardataire admis avec toutes ses vies (suisse) ou avant le tirage
		}
	case EvPlayerWithdrawn:
		if _, ok := s.Players[ev.ID]; !ok {
			return fmt.Errorf("joueur %s inconnu", ev.ID)
		}
		if ev.AfterCurrent && s.busy(ev.ID) {
			// Il finit son match : rien n'est perdu par forfait, et il n'est plus apparié
			// puisqu'il est occupé. promoteDeferred le retirera quand le match sera fini.
			s.WithdrawAfter[ev.ID] = true
			return s.seen(ev)
		}
		delete(s.WithdrawAfter, ev.ID)
		s.Withdrawn[ev.ID] = true
		for _, id := range s.MatchOrder { // ses matchs en cours sont perdus par forfait
			m := s.Matches[id]
			if m.Status == Running && m.Has(ev.ID) {
				m.Status, m.Forfeit, m.End = Finished, true, ev.Time
				m.Winner = m.A
				if m.A == ev.ID {
					m.Winner = m.B
				}
				s.onResult(m)
			}
		}
		s.recompute()
	case EvMatchStarted:
		if _, dup := s.Matches[ev.MatchID]; dup || ev.MatchID == "" {
			return fmt.Errorf("identifiant de match invalide ou déjà utilisé : %q", ev.MatchID)
		}
		if ev.A == ev.B || ev.A == "" || ev.B == "" {
			return fmt.Errorf("match %s : joueurs invalides", ev.MatchID)
		}
		for _, p := range []PlayerID{ev.A, ev.B} {
			if _, ok := s.Players[p]; !ok {
				return fmt.Errorf("joueur %s inconnu", p)
			}
			if s.busy(p) {
				return fmt.Errorf("joueur %s a déjà un match en cours", p)
			}
		}
		m := &Match{ID: ev.MatchID, Phase: ev.Phase, Section: ev.Section, Label: ev.Label, Key: ev.Key,
			A: ev.A, B: ev.B, Length: ev.Length, Table: ev.Table, Status: Running, Start: ev.Time}
		s.Matches[m.ID] = m
		s.MatchOrder = append(s.MatchOrder, m.ID)
		s.nextID++
		if ph := s.phaseOf(ev.Phase); ph != nil {
			ph.Started = true
			if ev.Round > ph.Round && ph.Cfg.Kind == KindSwissLives {
				ph.Round = ev.Round
			}
			ph.Opponents[m.A] = append(ph.Opponents[m.A], m.B)
			ph.Opponents[m.B] = append(ph.Opponents[m.B], m.A)
			if g := ph.gmatch(m.Section, m.Key); g != nil {
				g.MatchID = m.ID
			}
		}
	case EvResult, EvResultCorrected:
		m, ok := s.Matches[ev.MatchID]
		if !ok {
			return fmt.Errorf("match %s inconnu", ev.MatchID)
		}
		if ev.Kind == EvResult && m.Status == Finished {
			return fmt.Errorf("match %s déjà terminé (utiliser result_corrected)", ev.MatchID)
		}
		if ev.Winner != m.A && ev.Winner != m.B {
			return fmt.Errorf("match %s : vainqueur %s absent du match", ev.MatchID, ev.Winner)
		}
		m.Status, m.Winner, m.ScoreA, m.ScoreB, m.Forfeit = Finished, ev.Winner, ev.ScoreA, ev.ScoreB, ev.Forfeit
		if m.End.IsZero() || ev.Kind == EvResult {
			m.End = ev.Time
		}
		if ev.Kind == EvResultCorrected {
			s.recompute()
		} else {
			s.onResult(m)
		}
		s.promoteDeferred()
		if s.Withdrawn[m.A] || s.Withdrawn[m.B] {
			s.recompute() // un retrait différé vient de prendre effet
		}
	case EvMatchCancelled:
		m, ok := s.Matches[ev.MatchID]
		if !ok {
			return fmt.Errorf("match %s inconnu", ev.MatchID)
		}
		m.Status, m.End = Cancelled, ev.Time
		s.recompute()
	case EvBye:
		ph := s.phaseOf(ev.Phase)
		if ph == nil {
			return fmt.Errorf("phase %d inconnue", ev.Phase)
		}
		ph.Byes[ev.ID]++
		if ev.Round > ph.Round && ph.Cfg.Kind == KindSwissLives {
			ph.Round = ev.Round
		}
	case EvDraw:
		ph := s.phaseOf(ev.Phase)
		if ph == nil {
			return fmt.Errorf("phase %d inconnue", ev.Phase)
		}
		if ev.Draw == nil {
			return fmt.Errorf("draw sans tirage")
		}
		if err := s.applyDraw(ph, ev.Section, ev.Draw); err != nil {
			return err
		}
	case EvNextPhase:
		ph := s.phase()
		if ph == nil || s.Current+1 >= len(s.Config.Phases) {
			return fmt.Errorf("pas de phase suivante")
		}
		ph.Done = true
		next := newPhaseState(s.Current+1, s.Config.Phases[s.Current+1])
		s.Phases = append(s.Phases, next)
		s.Current++
		s.enterFrom(next, ph)
	case EvTableChanged:
		m, ok := s.Matches[ev.MatchID]
		if !ok {
			return fmt.Errorf("match %s inconnu", ev.MatchID)
		}
		m.Table = ev.Table
	case EvLengthChanged:
		ph := s.phaseOf(ev.Phase)
		if ph == nil {
			return fmt.Errorf("phase %d inconnue", ev.Phase)
		}
		ph.Length = ev.Length
	case EvFinished:
		s.Finished = true
		s.Final = s.Ranking()
	case EvNote:
	default:
		return fmt.Errorf("événement %q inconnu", ev.Kind)
	}
	s.NEvents++
	if ev.Time.After(s.Last) {
		s.Last = ev.Time
	}
	return nil
}

func (s *State) phaseOf(i int) *PhaseState {
	if i < 0 || i >= len(s.Phases) {
		return nil
	}
	return s.Phases[i]
}

// busy indique si le joueur a un match en cours.
func (s *State) busy(p PlayerID) bool {
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Status == Running && m.Has(p) {
			return true
		}
	}
	return false
}

// enter inscrit un joueur dans une phase avec un nombre de vies.
func (s *State) enter(ph *PhaseState, p PlayerID, lives int) {
	for _, e := range ph.Entrants {
		if e == p {
			return
		}
	}
	ph.Entrants = append(ph.Entrants, p)
	ph.Lives[p] = lives
}

// enterFrom remplit les entrants d'une phase à partir de la précédente.
func (s *State) enterFrom(next, prev *PhaseState) {
	switch {
	case next.Cfg.Entry == "all":
		for _, p := range s.Order {
			if !s.Withdrawn[p] {
				s.enter(next, p, livesFor(next.Cfg))
			}
		}
	case len(next.Cfg.Entry) > 4 && next.Cfg.Entry[:4] == "top:":
		var n int
		fmt.Sscanf(next.Cfg.Entry[4:], "%d", &n)
		r := s.phaseRanking(prev)
		for _, rk := range r {
			if rk.Rank <= n && !s.Withdrawn[rk.Player] {
				s.enter(next, rk.Player, livesFor(next.Cfg))
			}
		}
	default: // survivants avec leurs vies restantes
		for _, p := range s.survivors(prev) {
			if !s.Withdrawn[p] {
				l := s.remainingLives(prev, p)
				if next.Cfg.Kind != KindLivesBracket && next.Cfg.Kind != KindGSL && next.Cfg.Kind != KindSwissLives {
					l = livesFor(next.Cfg)
				}
				s.enter(next, p, l)
			}
		}
	}
}

func livesFor(cfg PhaseConfig) int {
	switch cfg.Kind {
	case KindSwissLives:
		return cfg.Lives
	case KindGSL:
		return 2
	}
	return 1
}

// remainingLives : vies restantes d'un joueur dans une phase à vies.
func (s *State) remainingLives(ph *PhaseState, p PlayerID) int {
	if s.Withdrawn[p] {
		return 0
	}
	l := ph.Lives[p] - ph.Losses[p]
	if l < 0 {
		l = 0
	}
	return l
}

// alive : joueurs de la phase encore en vie.
func (s *State) alive(ph *PhaseState) []PlayerID {
	var out []PlayerID
	for _, p := range ph.Entrants {
		if s.remainingLives(ph, p) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// survivors : joueurs qui sortent vivants de la phase (pour la phase suivante).
func (s *State) survivors(ph *PhaseState) []PlayerID {
	switch ph.Cfg.Kind {
	case KindRoundRobin:
		return s.rrQualified(ph)
	case KindBracket, KindLivesBracket:
		// après un tableau, seuls les vainqueurs de sections « comptent » ; on renvoie les joueurs
		// dont le parcours n'est pas terminé (tableau interrompu) ou le vainqueur.
		return s.bracketSurvivors(ph)
	}
	return s.alive(ph)
}

// onResult met à jour la comptabilité de la phase du match après un résultat.
func (s *State) onResult(m *Match) {
	ph := s.phaseOf(m.Phase)
	if ph == nil {
		return
	}
	loser := m.Loser()
	ph.Wins[m.Winner]++
	ph.Losses[loser]++
	if s.remainingLives(ph, loser) == 0 && ph.Lives[loser] > 0 {
		ph.ElimOrder = append(ph.ElimOrder, loser)
	}
	if g := ph.gmatch(m.Section, m.Key); g != nil {
		g.Done, g.Winner, g.Loser = true, m.Winner, loser
		ph.resolve(s.Withdrawn)
	}
}

// recompute recalcule toute la comptabilité des phases à partir des matchs (corrections, annulations).
func (s *State) recompute() {
	for _, ph := range s.Phases {
		ph.Losses, ph.Wins, ph.Opponents = map[PlayerID]int{}, map[PlayerID]int{}, map[PlayerID][]PlayerID{}
		ph.ElimOrder = nil
		for _, sec := range ph.Sections {
			for i := range sec.Matches {
				g := &sec.Matches[i]
				g.Done, g.Winner, g.Loser, g.Walkover, g.Skipped = false, "", "", false, false
			}
		}
	}
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		ph := s.phaseOf(m.Phase)
		if ph == nil || m.Status == Cancelled {
			continue
		}
		ph.Opponents[m.A] = append(ph.Opponents[m.A], m.B)
		ph.Opponents[m.B] = append(ph.Opponents[m.B], m.A)
		if g := ph.gmatch(m.Section, m.Key); g != nil {
			g.MatchID = m.ID
		}
		if m.Status == Finished {
			s.onResult(m)
		}
	}
	for _, ph := range s.Phases {
		ph.resolve(s.Withdrawn)
	}
	s.Warnings = s.check()
}

// check signale les incohérences (après correction d'un résultat, par exemple).
func (s *State) check() []Warning {
	var w []Warning
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Status == Cancelled {
			continue
		}
		ph := s.phaseOf(m.Phase)
		if ph == nil {
			continue
		}
		if g := ph.gmatch(m.Section, m.Key); g != nil {
			if g.Players[0] != "" && g.Players[1] != "" && !((g.Players[0] == m.A && g.Players[1] == m.B) || (g.Players[0] == m.B && g.Players[1] == m.A)) {
				w = append(w, Warning{Code: WarnBracketWrongPlayers, Match: m.ID, Section: m.Section, Label: m.Label,
					A: m.A, B: m.B, ExpectedA: g.Players[0], ExpectedB: g.Players[1]})
			}
		}
		if m.Status == Finished && m.Length > 0 && (m.ScoreA > m.Length || m.ScoreB > m.Length) {
			w = append(w, Warning{Code: WarnScoreOverLength, Match: m.ID, Length: m.Length, ScoreA: m.ScoreA, ScoreB: m.ScoreB})
		}
	}
	return w
}

// Ranking renvoie le classement général courant : la phase en cours d'abord, puis les phases
// précédentes (joueurs éliminés plus tôt classés derrière).
func (s *State) Ranking() []Rank {
	var out []Rank
	seen := map[PlayerID]bool{}
	offset := 0
	for i := len(s.Phases) - 1; i >= 0; i-- {
		r := s.phaseRanking(s.Phases[i])
		max := 0
		for _, rk := range r {
			if seen[rk.Player] {
				continue
			}
			seen[rk.Player] = true
			out = append(out, Rank{Player: rk.Player, Rank: rk.Rank + offset, Note: rk.Note})
			if rk.Rank > max {
				max = rk.Rank
			}
		}
		offset += len(r)
	}
	// renumérotation dense avec ex æquo conservés
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

// phaseRanking : classement interne d'une phase.
func (s *State) phaseRanking(ph *PhaseState) []Rank {
	switch ph.Cfg.Kind {
	case KindBracket, KindLivesBracket:
		return s.bracketRanking(ph)
	case KindRoundRobin:
		return s.rrRanking(ph)
	}
	return s.livesRanking(ph)
}

// livesRanking : vainqueur, finaliste, puis par victoires au moment de l'élimination (ex æquo).
func (s *State) livesRanking(ph *PhaseState) []Rank {
	type sc struct {
		p     PlayerID
		score float64
		note  Note
	}
	var list []sc
	alive := s.alive(ph)
	for _, p := range ph.Entrants {
		v := float64(ph.Wins[p])
		note := Note{Kind: NoteRecord, Wins: ph.Wins[p], Losses: ph.Losses[p]}
		if s.remainingLives(ph, p) > 0 {
			v += 1000 + float64(s.remainingLives(ph, p))
			note = Note{Kind: NoteAlive, Lives: s.remainingLives(ph, p)}
			if len(alive) == 1 {
				note = Note{Kind: NoteWinner}
			}
		}
		if s.Withdrawn[p] {
			v = -1
			note = Note{Kind: NoteForfeit}
		}
		list = append(list, sc{p, v, note})
	}
	if len(alive) == 1 && len(ph.ElimOrder) > 0 { // finaliste = dernier éliminé
		last := ph.ElimOrder[len(ph.ElimOrder)-1]
		for i := range list {
			if list[i].p == last {
				list[i].score, list[i].note = 999, Note{Kind: NoteFinalist}
			}
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].score > list[b].score })
	out := make([]Rank, len(list))
	rank := 1
	for i := range list {
		if i > 0 && list[i].score != list[i-1].score {
			rank = i + 1
		}
		out[i] = Rank{Player: list[i].p, Rank: rank, Note: list[i].note}
	}
	return out
}

// labelPtr : copie adressable d'un libellé, pour les champs Sub.
func labelPtr(l Label) *Label {
	if l.Empty() {
		return nil
	}
	c := l
	return &c
}
