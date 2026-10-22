package rules

import "time"

// MatchStatus est l'état d'un match lancé. Les valeurs sont celles du journal du moteur.
type MatchStatus string

const (
	Running   MatchStatus = "running"
	Finished  MatchStatus = "finished"
	Cancelled MatchStatus = "cancelled"
)

// Match est ce que les règles de fin et la feuille lisent d'un match lancé. Chaque règle n'en lit
// qu'une partie ; l'hôte peut laisser le reste à zéro.
type Match struct {
	ID      string      `json:"id"` // identifiant opaque, rendu tel quel dans les lots et la feuille
	Phase   int         `json:"phase"`
	Status  MatchStatus `json:"status"`
	Start   time.Time   `json:"start"`
	End     time.Time   `json:"end,omitempty"`
	Table   int         `json:"table,omitempty"`
	Length  int         `json:"length"`
	A       Ref         `json:"a"`
	B       Ref         `json:"b"`
	Forfeit bool        `json:"forfeit,omitempty"`
}

// Interval est un créneau [Start, End) : une pause déclarée, le repas, la nuit.
type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
