package rules

import "time"

// MatchStatus est l'état d'un match lancé. Les valeurs sont celles du journal du moteur.
type MatchStatus string

const (
	Running   MatchStatus = "running"
	Finished  MatchStatus = "finished"
	Cancelled MatchStatus = "cancelled"
)

// Match est ce que les règles lisent d'un match LANCÉ (une proposition, elle, est un Pairing).
// Chaque règle n'en lit qu'une partie ; l'hôte peut laisser le reste à zéro.
type Match struct {
	ID      string      `json:"id"` // identifiant opaque, rendu tel quel dans les lots et la feuille
	Phase   int         `json:"phase"`
	Section string      `json:"section,omitempty"` // section du graphe ou du barrage
	Label   *Label      `json:"label,omitempty"`   // libellé du match : celui d'une ligne de feuille
	Status  MatchStatus `json:"status"`
	Start   time.Time   `json:"start"`
	End     time.Time   `json:"end"` // valeur nulle : pas encore terminé, ou fin inconnue
	Table   int         `json:"table,omitempty"`
	Length  int         `json:"length"`
	A       Ref         `json:"a"`
	B       Ref         `json:"b"`
	Winner  Ref         `json:"winner,omitempty"`
	Forfeit bool        `json:"forfeit,omitempty"`
}

// loser : le perdant d'un match terminé.
func (m Match) loser() Ref {
	if m.Winner == m.A {
		return m.B
	}
	return m.A
}

// Interval est un créneau [Start, End) : une pause déclarée, le repas, la nuit.
type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
