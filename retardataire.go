package tournoi

import "fmt"

// Retardataires : un joueur qui arrive après le tirage.
//
// Il en arrive un à chaque tournoi, et il reste presque toujours une place d'exemption libre au
// premier tour. Le moteur ne refait JAMAIS un tirage déjà fait — un tirage est un événement du
// journal, et le refaire déplacerait des joueurs qui ont déjà lu leur nom sur le mur. Le
// retardataire prend donc une place vide, ou il entre plus tard, et le moteur dit lequel des
// deux (Info, codes.go) au lieu de le laisser inscrit nulle part.

// Slot est une place d'exemption libre : un côté d'un match de graphe dont la source est un BYE
// fixe, dans un tour que personne n'a commencé à jouer. C'est ce qu'un hôte propose au TD quand
// un joueur se présente en retard.
type Slot struct {
	Phase   int    `json:"phase"`
	Section string `json:"section"`
	Key     string `json:"key"`
	Label   Label  `json:"label,omitempty"`
}

// roundStarted : un match de ce tour de la section a déjà été lancé.
//
// Un tour dont un seul match a commencé est un tour en cours : y glisser un joueur ferait jouer
// à ses adversaires un tableau différent de celui qu'ils ont sous les yeux. Une exemption
// résolue au tirage (Walkover) ne compte pas : personne n'a joué.
func roundStarted(sec *Section, r int) bool {
	if r < 0 || r >= len(sec.Rounds) {
		return true
	}
	for _, i := range sec.Rounds[r] {
		if sec.Matches[i].MatchID != "" {
			return true
		}
	}
	return false
}

// roundOf : le tour d'un match dans sa section (-1 s'il n'y figure pas).
func roundOf(sec *Section, idx int) int {
	for r, ids := range sec.Rounds {
		for _, i := range ids {
			if i == idx {
				return r
			}
		}
	}
	return -1
}

// FreeSlots énumère les places d'exemption qu'un retardataire peut encore prendre, dans l'ordre
// du tableau. L'hôte les propose au TD ; la place choisie voyage dans l'événement d'inscription
// (PlayerAddedAtSlotEvent).
func (s *State) FreeSlots() []Slot {
	ph := s.phase()
	if ph == nil {
		return nil
	}
	var out []Slot
	for _, sec := range ph.Sections {
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.MatchID != "" {
				continue
			}
			r := roundOf(sec, i)
			if r < 0 || roundStarted(sec, r) {
				continue
			}
			for k := 0; k < 2; k++ {
				if g.Src[k].Player == BYE {
					out = append(out, Slot{Phase: ph.Index, Section: sec.Name, Key: g.Key, Label: g.Label})
					break // une place par match suffit : la seconde se proposera après celle-ci
				}
			}
		}
	}
	return out
}

// takeSlot installe un joueur sur une place d'exemption. Une place déjà jouée, inconnue, ou
// prise est refusée : le moteur ne devine pas ce que le TD a voulu dire.
func (s *State) takeSlot(ph *PhaseState, section, key string, p PlayerID) error {
	if ph == nil {
		return fmt.Errorf("place %q : aucune phase en cours", key)
	}
	for _, sec := range ph.Sections {
		if section != "" && sec.Name != section {
			continue
		}
		for i := range sec.Matches {
			g := &sec.Matches[i]
			if g.Key != key {
				continue
			}
			if g.MatchID != "" {
				return fmt.Errorf("place %q : le match a déjà été lancé", key)
			}
			r := roundOf(sec, i)
			if roundStarted(sec, r) {
				return fmt.Errorf("place %q : le tour %d de la section %s a déjà commencé", key, r+1, sec.Name)
			}
			for k := 0; k < 2; k++ {
				if g.Src[k].Player != BYE {
					continue
				}
				g.Src[k].Player = p
				s.enter(ph, p, 1) // il entre avec une vie : il ne bénéficie pas de l'exemption qu'il occupe
				s.recompute()
				return nil
			}
			return fmt.Errorf("place %q : aucune exemption libre dans ce match", key)
		}
	}
	return fmt.Errorf("place %q inconnue dans la phase %d", key, ph.Index)
}

// entersAt dit où un joueur inscrit mais engagé dans aucune phase entrera.
//
// Trois cas, dans l'ordre où ils servent le joueur : une place d'exemption libre dans la phase
// en cours (il joue tout de suite, dès que le TD la lui donne), une phase à venir dont l'entrée
// est ouverte à tous, ou rien — et « rien » se dit, plutôt que de laisser un inscrit disparaître
// de l'affichage.
func (s *State) entersAt(p PlayerID) Info {
	if slots := s.FreeSlots(); len(slots) > 0 {
		return Info{Code: InfoEntersAt, Player: p, Phase: slots[0].Phase, Section: slots[0].Section, Label: slots[0].Label}
	}
	for i := s.Current + 1; i < len(s.Config.Phases); i++ {
		if s.Config.Phases[i].Entry == "all" {
			return Info{Code: InfoEntersAt, Player: p, Phase: i,
				Label: Label{Kind: LabelPhase, Text: PhaseName(s.Config.Phases[i])}}
		}
	}
	return Info{Code: InfoNoEntry, Player: p}
}

// refreshInfos recalcule les informations sur les inscrits qui ne jouent encore nulle part.
//
// Elles sont DÉRIVÉES de l'état, jamais accumulées : un joueur à qui le TD finit par donner une
// place disparaît de la liste au même instant, sans qu'aucun événement n'ait à la corriger.
func (s *State) refreshInfos() {
	s.Infos = nil
	if s.Current < 0 {
		return
	}
	engagés := map[PlayerID]bool{}
	for _, ph := range s.Phases {
		for _, e := range ph.Entrants {
			engagés[e] = true
		}
	}
	for _, p := range s.Order {
		if engagés[p] || s.Withdrawn[p] {
			continue
		}
		s.Infos = append(s.Infos, s.entersAt(p))
	}
}
