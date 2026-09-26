package tournoi

import (
	"fmt"
	"time"
)

// Indisponibilités : un joueur qui n'est pas là pour un temps, sans quitter le tournoi.
//
// Il n'existait que trois chemins, tous faux : le retirer puis le réinscrire (acquis conservés,
// mais « forfait » au classement pendant l'absence), ne rien faire (son appariement glisse, ou
// « Tout lancer » le lance), ou un forfait (une vie perdue). Un festival à trois épreuves a dû
// retenir douze appariements à la main parce qu'un joueur jouait dans une autre épreuve.
//
// player_unavailable dit qu'il n'est pas appariable jusqu'à une heure, jusqu'à une ronde, ou
// jusqu'à nouvel ordre (player_available) ; il garde tout le reste — vies, victoires, rang,
// place dans un tableau. Le moteur n'apparie pas un absent au suisse, et le dit dans la file
// (ReasonPlayerUnavailable) ; dans un graphe, où la place est fixée, le match reste proposé,
// sans table, avec la même raison.

// Absence est une indisponibilité déclarée.
type Absence struct {
	Until time.Time `json:"until,omitempty"` // heure de retour ; zéro = pas d'échéance horaire
	Round int       `json:"round,omitempty"` // suisse par rondes : ronde de retour
	Phase int       `json:"phase"`           // phase où elle a été déclarée (Round n'y vaut que là)
}

// applyAbsence applique player_unavailable et player_available.
func (s *State) applyAbsence(ev Event) error {
	if _, ok := s.Players[ev.ID]; !ok {
		return fmt.Errorf("joueur %s inconnu", ev.ID)
	}
	if ev.Kind == EvPlayerAvailable {
		delete(s.Unavailable, ev.ID)
		return nil
	}
	a := Absence{Round: ev.Round, Phase: s.Current}
	if ev.Until != nil {
		a.Until = *ev.Until
	}
	if !a.Until.IsZero() && a.Round > 0 {
		return fmt.Errorf("indisponibilité de %s : une heure OU une ronde de retour, pas les deux", ev.ID)
	}
	if a.Round > 0 {
		if ph := s.phase(); ph == nil || ph.Cfg.Kind != KindSwissLives || ph.Cfg.Mode != "rounds" {
			return fmt.Errorf("indisponibilité de %s jusqu'à la ronde %d : la phase en cours n'a pas de rondes", ev.ID, a.Round)
		}
	}
	if s.Unavailable == nil {
		s.Unavailable = map[PlayerID]Absence{}
	}
	s.Unavailable[ev.ID] = a
	return nil
}

// absent : le joueur est indisponible à l'heure de l'état (s.clock), pour la ronde r (0 = hors
// rondes). Une absence « jusqu'à la ronde k » tombe dès que la ronde appariée atteint k, ou que
// la phase a changé ; une absence à échéance tombe à l'heure dite.
func (s *State) absent(p PlayerID, r int) bool {
	a, ok := s.Unavailable[p]
	if !ok {
		return false
	}
	switch {
	case a.Round > 0:
		return a.Phase == s.Current && (r == 0 || r < a.Round)
	case !a.Until.IsZero():
		return s.clock.Before(a.Until)
	}
	return true
}

// roundProposed : la ronde que le suisse par rondes propose en ce moment — la ronde ouverte si
// des appariements en restent, sinon la suivante. 0 hors rondes.
func (s *State) roundProposed(ph *PhaseState) int {
	if ph.Cfg.Kind != KindSwissLives || ph.Cfg.Mode != "rounds" {
		return 0
	}
	if ph.Round > 0 && len(s.roundRest(ph, ph.Round)) > 0 {
		return ph.Round
	}
	return ph.Round + 1
}

// withAbsences : dans un suisse, une attente par joueur en vie indisponible — la file dit
// pourquoi il n'est pas apparié, et quand il revient (Until ou Round) — puis une par joueur en
// vie qui joue dans une autre épreuve de la salle (player_busy). L'indisponibilité, écrite au
// journal, l'emporte : un joueur n'a qu'une attente.
func (s *State) withAbsences(ph *PhaseState, acts []Action) []Action {
	if ph.Cfg.Kind != KindSwissLives || (len(s.Unavailable) == 0 && len(s.ailleurs) == 0) {
		return acts
	}
	r := s.roundProposed(ph)
	for _, p := range s.alive(ph) {
		switch {
		case s.absent(p, r):
			a := s.Unavailable[p]
			acts = append(acts, Action{Kind: ActWait, Phase: ph.Index, Reason: ReasonPlayerUnavailable, A: p,
				Until: a.Until, Round: a.Round})
		case s.ailleurs[p] && !s.busy(p):
			acts = append(acts, Action{Kind: ActWait, Phase: ph.Index, Reason: ReasonPlayerBusy, A: p})
		}
	}
	return acts
}

// holdAbsent : dans un graphe, le match d'un absent reste proposé — sa place est fixée — mais
// retenu : raison player_unavailable, pas de table.
func (s *State) holdAbsent(acts []Action) {
	if len(s.Unavailable) == 0 {
		return
	}
	for i := range acts {
		if acts[i].Kind == ActStartMatch && acts[i].Reason == ReasonNone && (s.absent(acts[i].A, 0) || s.absent(acts[i].B, 0)) {
			acts[i].Reason = ReasonPlayerUnavailable
		}
	}
}

// holdElsewhere : dans un graphe, le match d'un joueur qui joue dans une autre épreuve de la
// salle reste proposé mais retenu : raison player_busy, pas de table. Passe après holdAbsent,
// dont la raison l'emporte.
func (s *State) holdElsewhere(acts []Action) {
	if len(s.ailleurs) == 0 {
		return
	}
	for i := range acts {
		if acts[i].Kind == ActStartMatch && acts[i].Reason == ReasonNone && (s.ailleurs[acts[i].A] || s.ailleurs[acts[i].B]) {
			acts[i].Reason = ReasonPlayerBusy
		}
	}
}
