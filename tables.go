package tournoi

import "encoding/json"

// Les tables d'une salle.
//
// Un directeur réel a une table au plateau cassé, une table réservée à la retransmission pour
// la finale seulement, et des joueurs qui demandent à changer de table en cours de match.
// assignTables ne donne donc plus « la plus petite libre » : elle saute les tables
// indisponibles et les tables réservées hors de leur usage, et laisse la proposition SANS table
// quand il n'en reste aucune — en le disant (ReasonWaitingTable), au lieu de sortir muette.

// TableRule réserve une table à une section ou à une phase. Une réservation vide des deux côtés
// ne réserve rien.
type TableRule struct {
	Table     int    `json:"table"`
	Section   string `json:"section,omitempty"` // identifiant de section (main, conso, poule:A…)
	Phase     int    `json:"phase,omitempty"`   // index de phase ; 0 = toutes
	AllPhases bool   `json:"all_phases,omitempty"`
}

// Tables décrit les tables de la salle. Count = 0 signifie « illimité » : le moteur numérote
// alors sans borne, ce qui reste utile pour une simulation.
type Tables struct {
	Count       int         `json:"count,omitempty"`
	Unavailable []int       `json:"unavailable,omitempty"` // plateau cassé, table retirée
	Reserved    []TableRule `json:"reserved,omitempty"`    // retransmission, table d'honneur
}

// UnmarshalJSON accepte les deux formes : le nombre des configurations d'avant les exceptions,
// et la structure. Une configuration écrite `"tables": 12` reste donc lisible.
func (t *Tables) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] != '{' {
		var n int
		if err := json.Unmarshal(b, &n); err != nil {
			return err
		}
		*t = Tables{Count: n}
		return nil
	}
	type brut Tables
	var v brut
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*t = Tables(v)
	return nil
}

// unavailable : la table est hors service.
func (t Tables) unavailable(n int) bool {
	for _, u := range t.Unavailable {
		if u == n {
			return true
		}
	}
	return false
}

// AvailableFor : la table n peut accueillir un match de cette section et de cette phase. Une
// table réservée n'accueille que ce pour quoi elle est réservée ; les autres tables sont libres.
func (t Tables) AvailableFor(n int, section string, phase int) bool {
	if n <= 0 || t.unavailable(n) {
		return false
	}
	if t.Count > 0 && n > t.Count {
		return false
	}
	for _, r := range t.Reserved {
		if r.Table != n {
			continue
		}
		if r.Section != "" && r.Section != section {
			return false
		}
		if !r.AllPhases && r.Phase != 0 && r.Phase != phase {
			return false
		}
		return true
	}
	return true
}
