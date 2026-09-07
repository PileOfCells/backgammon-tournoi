package tournoi

import "fmt"

// Configuration modifiable en cours de tournoi, et réouverture d'un tournoi clos.
//
// La configuration était figée dans l'événement de création, et seule la longueur des matchs à
// venir d'une phase pouvait changer (EvLengthChanged). Un directeur réel décide à 22 h de
// baisser la bascule de 16 à 8 pour finir plus tôt, ajoute une consolante qu'il n'avait pas
// prévue, ou rouvre un tournoi clos parce qu'un résultat était faux.
//
// EvConfigChanged porte la configuration ENTIÈRE, et non un champ à changer. C'est le même
// choix que pour EvDraw : ce qui est écrit dans le journal est le RÉSULTAT, pas l'instruction
// qui y mène. Un journal se relit alors sans connaître la règle de composition des retouches
// successives, et l'hôte n'a qu'un formulaire à envoyer — celui qui a servi à créer le tournoi.
//
// Ce qui est refusé tient en une phrase : on ne change pas le format d'une phase qui a déjà
// commencé, et on ne retire pas une phase qui existe. Le reste est autorisé, y compris ce que
// le moteur ne peut pas juger (dotation, tables, pauses) : le TD dirige.

// acceptConfig dit si next peut remplacer la configuration courante, et pourquoi non.
//
// Le refus est nominatif : le TD doit savoir QUELLE phase bloque et à cause de quoi, sans quoi
// il ne peut que renoncer.
func (s *State) acceptConfig(next Config) error {
	if len(next.Phases) < len(s.Phases) {
		return fmt.Errorf("configuration : %d phases proposées alors que le tournoi en a déjà ouvert %d ; "+
			"une phase ouverte ne se retire pas (rouvrez-la ou clôturez le tournoi)", len(next.Phases), len(s.Phases))
	}
	for i, ph := range s.Phases {
		if next.Phases[i].Kind == ph.Cfg.Kind {
			continue
		}
		switch {
		case i < s.Current:
			return fmt.Errorf("phase %d : son format ne change plus de %q en %q, elle est terminée",
				i, ph.Cfg.Kind, next.Phases[i].Kind)
		case ph.Drawn:
			return fmt.Errorf("phase %d : son format ne change plus de %q en %q, son tirage est fait",
				i, ph.Cfg.Kind, next.Phases[i].Kind)
		case ph.Started:
			return fmt.Errorf("phase %d : son format ne change plus de %q en %q, des matchs y sont lancés",
				i, ph.Cfg.Kind, next.Phases[i].Kind)
		}
	}
	return nil
}

// setConfig installe la nouvelle configuration et la répercute sur les phases ouvertes.
//
// La longueur courante d'une phase (PhaseState.Length) ne suit la configuration que si la
// configuration l'a effectivement changée : sinon, une retouche qui ne touche qu'à la bascule
// effacerait le length_changed que le TD venait de saisir à la main.
func (s *State) setConfig(next Config) {
	s.Config = next
	for i, ph := range s.Phases {
		old := ph.Cfg
		ph.Cfg = next.Phases[i]
		if old.Length != ph.Cfg.Length {
			ph.Length = ph.Cfg.Length
		}
		if old.Kind != ph.Cfg.Kind {
			// Le format d'une phase non commencée change : ses entrants n'ont plus le bon
			// nombre de vies (un suisse à 3 vies n'entre pas dans un tableau à une vie).
			ph.Length = ph.Cfg.Length
			for _, p := range ph.Entrants {
				ph.Lives[p] = livesFor(ph.Cfg)
			}
		}
	}
}

// clone : copie profonde d'une configuration.
//
// Config porte des tranches ; sans copie, l'état et l'événement du journal partageraient le
// même tableau de phases, et Validate — qui remplit les défauts en place — écrirait dans le
// journal de l'hôte. Un journal ne se modifie jamais, pas même par mégarde.
func (c Config) clone() Config {
	out := c
	out.Phases = append([]PhaseConfig(nil), c.Phases...)
	for i := range out.Phases {
		out.Phases[i].Lengths = append([]int(nil), c.Phases[i].Lengths...)
	}
	out.Prizes = append([]float64(nil), c.Prizes...)
	out.Tables.Unavailable = append([]int(nil), c.Tables.Unavailable...)
	out.Tables.Reserved = append([]TableRule(nil), c.Tables.Reserved...)
	return out
}
