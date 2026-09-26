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
// Ce qui est refusé tient en deux phrases : on ne change pas le format d'une phase qui a déjà
// commencé, et on ne retire pas une phase qui existe ; on ne change pas, après le tirage, ce qui
// ne sert qu'à construire un tableau (la consolante d'un tableau tiré n'aurait aucun effet). Le
// reste est autorisé, y compris ce que le moteur ne peut pas juger (dotation, tables, pauses) :
// le TD dirige.

// ConfigRefusal est le refus d'une configuration : la phase, le champ, et la raison — un CODE,
// comme tout ce qui sort du moteur. Error() en donne le rendu français, pour la console.
type ConfigRefusal struct {
	Phase  int    `json:"phase"`
	Field  string `json:"field"`            // champ JSON de PhaseConfig ("kind", "consolation"…) ; "phases" pour un retrait
	Reason string `json:"reason"`           // RefusalRemoved, RefusalFinished, RefusalDrawn, RefusalStarted
	From   string `json:"from,omitempty"`   // kind : l'ancien format
	To     string `json:"to,omitempty"`     // kind : le nouveau format
	Opened int    `json:"opened,omitempty"` // removed : le nombre de phases déjà ouvertes
}

// Raisons d'un ConfigRefusal.
const (
	RefusalRemoved  = "removed"  // la configuration retire une phase déjà ouverte
	RefusalFinished = "finished" // la phase est terminée
	RefusalDrawn    = "drawn"    // le tirage de la phase est fait
	RefusalStarted  = "started"  // des matchs de la phase sont lancés
)

func (r *ConfigRefusal) Error() string {
	pourquoi := map[string]string{RefusalFinished: "elle est terminée", RefusalDrawn: "son tirage est fait",
		RefusalStarted: "des matchs y sont lancés"}[r.Reason]
	switch {
	case r.Reason == RefusalRemoved:
		return fmt.Sprintf("configuration : %d phases proposées alors que le tournoi en a déjà ouvert %d ; "+
			"une phase ouverte ne se retire pas (rouvrez-la ou clôturez le tournoi)", r.Phase, r.Opened)
	case r.Field == "kind":
		return fmt.Sprintf("phase %d : son format ne change plus de %q en %q, %s", r.Phase, r.From, r.To, pourquoi)
	}
	return fmt.Sprintf("phase %d : %s ne change plus, %s — le tableau est construit", r.Phase, r.Field, pourquoi)
}

// CheckConfig dit si next pourrait remplacer la configuration courante, sans rien appliquer :
// la prévisualisation d'un formulaire de réglages. Le refus est un *ConfigRefusal (ou l'erreur
// de Validate). Il applique les règles d'aujourd'hui, celles d'un config_changed de la version
// courante du journal.
func (s *State) CheckConfig(next Config) error {
	c := next.clone()
	if err := c.Validate(); err != nil {
		return err
	}
	return s.acceptConfig(c, JournalVersion)
}

// champsDuTirage : les options d'un tableau qui ne servent qu'à CONSTRUIRE le graphe, au
// tirage. Une fois le tirage fait, les changer ne peut plus rien produire : cocher la
// consolante d'un tableau tiré était accepté, écrit dans la configuration, et sans effet — le
// TD croyait avoir une consolante. Elles sont figées au tirage, à partir de la version 2 du
// journal (voir acceptConfig).
func champsDuTirage(a, b PhaseConfig) string {
	switch {
	case a.Consolation != b.Consolation:
		return "consolation"
	case a.LastChance != b.LastChance:
		return "last_chance"
	case a.Reconciliation != b.Reconciliation:
		return "reconciliation"
	case a.Recharge != b.Recharge:
		return "recharge"
	case a.Seeding != b.Seeding:
		return "seeding"
	}
	return ""
}

// acceptConfig dit si next peut remplacer la configuration courante, et pourquoi non.
//
// Le refus est nominatif : le TD doit savoir QUELLE phase bloque et à cause de quoi, sans quoi
// il ne peut que renoncer.
//
// version est celle de l'événement config_changed. Les options de construction d'un tableau
// (champsDuTirage) ne sont figées qu'à partir de la version 2 : un journal plus ancien qui les
// changeait après le tirage a été joué ainsi — accepté, sans effet — et se rejoue ainsi.
func (s *State) acceptConfig(next Config, version int) error {
	if len(next.Phases) < len(s.Phases) {
		return &ConfigRefusal{Phase: len(next.Phases), Field: "phases", Reason: RefusalRemoved, Opened: len(s.Phases)}
	}
	for i, ph := range s.Phases {
		if next.Phases[i].Kind != ph.Cfg.Kind {
			r := &ConfigRefusal{Phase: i, Field: "kind", From: ph.Cfg.Kind, To: next.Phases[i].Kind}
			switch {
			case i < s.Current:
				r.Reason = RefusalFinished
			case ph.Drawn:
				r.Reason = RefusalDrawn
			case ph.Started:
				r.Reason = RefusalStarted
			default:
				continue
			}
			return r
		}
		if version < 2 || !ph.Drawn || (ph.Cfg.Kind != KindBracket && ph.Cfg.Kind != KindLivesBracket) {
			continue
		}
		if champ := champsDuTirage(ph.Cfg, next.Phases[i]); champ != "" {
			return &ConfigRefusal{Phase: i, Field: champ, Reason: RefusalDrawn}
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
	out.Prizes = c.Prizes.clone()
	out.Tables.Unavailable = append([]int(nil), c.Tables.Unavailable...)
	out.Tables.Reserved = append([]TableRule(nil), c.Tables.Reserved...)
	return out
}
