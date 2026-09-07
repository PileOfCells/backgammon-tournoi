package tournoi

import (
	"encoding/json"
	"fmt"
)

// Kinds de phases.
const (
	KindSwissLives   = "swiss_lives"   // suisse à vies (continu ou par rondes)
	KindLivesBracket = "lives_bracket" // tableau à exemptions : 2 vies = exempt du 1er tour
	KindGSL          = "gsl"           // blocs de groupes GSL (2 vies)
	KindBracket      = "bracket"       // élimination simple, consolante, dernière chance, double élimination
	KindRoundRobin   = "round_robin"   // poules toutes rondes puis qualification (barrage si égalité)
)

// Config décrit un tournoi : une suite de phases.
type Config struct {
	Name        string        `json:"name"`
	Phases      []PhaseConfig `json:"phases"`
	MinPerPoint float64       `json:"min_per_point,omitempty"` // durée moyenne d'un point (minutes), défaut 8
	Tables      Tables        `json:"tables,omitempty"`        // les tables de la salle (tables.go)
	Prizes      []float64     `json:"prizes,omitempty"`        // dotation par place (fractions ou montants)
}

// PhaseConfig paramètre une phase. Les champs inutiles pour un type sont ignorés.
type PhaseConfig struct {
	Kind        string `json:"kind"`
	Name        string `json:"name,omitempty"`
	Lives       int    `json:"lives,omitempty"`        // swiss_lives : nombre de vies (défaut 2)
	Length      int    `json:"length"`                 // longueur des matchs (points)
	FinalLength int    `json:"final_length,omitempty"` // longueur de la finale (tableaux), 0 = Length
	// Lengths donne la longueur tour par tour d'un tableau, DU DERNIER TOUR VERS LE PREMIER
	// (« 15, 13, 11, 9 » = finale en 15, demies en 13, quarts en 11, reste en 9). L'ordre est
	// celui dans lequel un organisateur annonce son tournoi, et il ne dépend pas de la taille
	// du tableau : la même liste sert un tableau de 16 et un tableau de 64, où elle allonge les
	// quatre derniers tours. Une liste plus courte que le tableau retombe sur Length.
	Lengths []int `json:"lengths,omitempty"`
	// LengthLate allonge les matchs de la fin d'un suisse, quand il ne reste plus que
	// LateThreshold joueurs en vie ou moins. Les deux vont ensemble : l'un sans l'autre ne fait
	// rien. Un changement de longueur décidé à la main par le TD (EvLengthChanged) l'emporte.
	LengthLate     int    `json:"length_late,omitempty"`
	LateThreshold  int    `json:"late_threshold,omitempty"`
	Mode           string `json:"mode,omitempty"`        // swiss_lives : "continuous" (défaut) ou "rounds"
	Pairing        string `json:"pairing,omitempty"`     // swiss_lives : "random" (défaut) ou "wins"
	AvoidClubs     bool   `json:"avoid_clubs,omitempty"` // éviter les joueurs du même club quand c'est possible
	AllowRematch   bool   `json:"allow_rematch,omitempty"`
	Target         int    `json:"target,omitempty"`         // swiss_lives / gsl : figer quand la somme des vies vaut Target (puissance de 2)
	Consolation    bool   `json:"consolation,omitempty"`    // bracket : consolante progressive (perdants du tableau principal)
	LastChance     bool   `json:"last_chance,omitempty"`    // bracket : dernière chance (perdants de la consolante)
	Reconciliation bool   `json:"reconciliation,omitempty"` // bracket : le vainqueur de la consolante joue le vainqueur du principal (double élimination)
	Recharge       bool   `json:"recharge,omitempty"`       // bracket : en double élimination, le vainqueur du principal doit être battu deux fois
	GroupSize      int    `json:"group_size,omitempty"`     // round_robin : taille des poules (défaut 4)
	Qualifiers     int    `json:"qualifiers,omitempty"`     // round_robin : qualifiés par poule (défaut 2)
	Entry          string `json:"entry,omitempty"`          // "survivors" (défaut : joueurs encore en vie de la phase précédente), "all", "top:N"
	// Seeding place les joueurs d'un tableau par cote au lieu de les tirer au hasard :
	// "" (défaut) ou SeedingRating. Le défaut VIDE est un choix de conception et non un oubli —
	// l'étude conclut « pas de têtes de série protégées », c'est la culture actuelle du
	// backgammon. Voir seeding.go.
	Seeding string `json:"seeding,omitempty"`
}

// Validate vérifie la cohérence d'une configuration et remplit les défauts.
func (c *Config) Validate() error {
	if len(c.Phases) == 0 {
		return fmt.Errorf("config : au moins une phase")
	}
	if c.MinPerPoint <= 0 {
		c.MinPerPoint = 8
	}
	for i := range c.Phases {
		p := &c.Phases[i]
		if p.Length <= 0 {
			return fmt.Errorf("phase %d : longueur de match manquante", i)
		}
		for k, l := range p.Lengths {
			if l <= 0 {
				return fmt.Errorf("phase %d : lengths[%d] = %d, une longueur de match est positive", i, k, l)
			}
		}
		if p.LengthLate > 0 && p.LateThreshold <= 0 {
			return fmt.Errorf("phase %d : length_late sans late_threshold — à partir de combien de joueurs ?", i)
		}
		switch p.Kind {
		case KindSwissLives:
			if p.Lives <= 0 {
				p.Lives = 2
			}
			if p.Mode == "" {
				p.Mode = "continuous"
			}
			if p.Mode != "continuous" && p.Mode != "rounds" {
				return fmt.Errorf("phase %d : mode %q inconnu", i, p.Mode)
			}
			if p.Pairing == "" {
				p.Pairing = "random"
			}
			if p.Target != 0 && (p.Target&(p.Target-1)) != 0 {
				return fmt.Errorf("phase %d : target doit être une puissance de 2", i)
			}
			if p.Target != 0 && p.Lives != 2 {
				return fmt.Errorf("phase %d : la bascule vers un tableau à vies suppose 2 vies", i)
			}
		case KindLivesBracket, KindBracket:
			if p.Seeding != "" && p.Seeding != SeedingRating {
				return fmt.Errorf("phase %d : seeding %q inconnu (vide ou %q)", i, p.Seeding, SeedingRating)
			}
			if p.Recharge && !p.Reconciliation {
				return fmt.Errorf("phase %d : recharge sans reconciliation", i)
			}
			if p.Reconciliation && !p.Consolation {
				return fmt.Errorf("phase %d : reconciliation sans consolante", i)
			}
		case KindGSL:
			p.Lives = 2
			if p.Target != 0 && (p.Target&(p.Target-1)) != 0 {
				return fmt.Errorf("phase %d : target doit être une puissance de 2", i)
			}
		case KindRoundRobin:
			if p.GroupSize <= 0 {
				p.GroupSize = 4
			}
			if p.Qualifiers <= 0 {
				p.Qualifiers = 2
			}
			if p.Qualifiers >= p.GroupSize {
				return fmt.Errorf("phase %d : qualifiers doit être < group_size", i)
			}
		default:
			return fmt.Errorf("phase %d : type %q inconnu", i, p.Kind)
		}
		if p.Entry == "" {
			p.Entry = "survivors"
		}
		if p.Seeding != "" && p.Kind != KindBracket && p.Kind != KindLivesBracket {
			return fmt.Errorf("phase %d : seeding ne s'applique qu'aux tableaux, pas à %q", i, p.Kind)
		}
	}
	return nil
}

// ParseConfig lit une configuration JSON et la valide.
func ParseConfig(b []byte) (*Config, error) {
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
