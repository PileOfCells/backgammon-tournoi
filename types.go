// Package tournoi est un moteur de tournoi de backgammon fondé sur un journal d'événements.
//
// L'hôte (le logiciel web) conserve la liste ordonnée des événements ; Replay reconstruit l'état ;
// Propose indique au directeur de tournoi (TD) quoi faire maintenant ; chaque décision du TD
// devient un événement. Aucun départage : les formats reposent sur les vies, les tableaux et les
// poules, avec barrages si nécessaire.
package tournoi

import "time"

// PlayerID identifie un joueur ; fourni par l'hôte.
type PlayerID string

// BYE est l'identifiant réservé d'une place vide dans un tableau (exemption).
const BYE PlayerID = "BYE"

// Player décrit un joueur inscrit.
type Player struct {
	ID     PlayerID `json:"id"`
	Name   string   `json:"name"`
	Club   string   `json:"club,omitempty"`
	Rating float64  `json:"rating,omitempty"` // PR (plus bas = meilleur) ; 0 = inconnu
}

// MatchID identifie un match dans le tournoi.
type MatchID string

// MatchStatus est l'état d'un match.
type MatchStatus string

const (
	Running   MatchStatus = "running"
	Finished  MatchStatus = "finished"
	Cancelled MatchStatus = "cancelled"
)

// Match est un match lancé (ou terminé) entre deux joueurs.
type Match struct {
	ID      MatchID     `json:"id"`
	Phase   int         `json:"phase"`
	Section string      `json:"section,omitempty"` // identifiant : "main", "conso", "poule:A"…
	Label   Label       `json:"label,omitempty"`   // libellé structuré (codes.go)
	Key     string      `json:"key,omitempty"`     // clé interne (position dans un graphe de matchs)
	A       PlayerID    `json:"a"`
	B       PlayerID    `json:"b"`
	Length  int         `json:"length"`
	Table   int         `json:"table,omitempty"`
	Status  MatchStatus `json:"status"`
	Start   time.Time   `json:"start"`
	End     time.Time   `json:"end,omitempty"`
	Winner  PlayerID    `json:"winner,omitempty"`
	ScoreA  int         `json:"score_a,omitempty"`
	ScoreB  int         `json:"score_b,omitempty"`
	Forfeit bool        `json:"forfeit,omitempty"`
}

// Loser renvoie le perdant d'un match terminé.
func (m *Match) Loser() PlayerID {
	if m.Winner == m.A {
		return m.B
	}
	return m.A
}

// Has indique si p joue ce match.
func (m *Match) Has(p PlayerID) bool { return m.A == p || m.B == p }

// Rank est une place au classement ; les ex æquo partagent le même Rank.
type Rank struct {
	Player PlayerID `json:"player"`
	Rank   int      `json:"rank"`
	Note   Note     `json:"note,omitempty"` // note structurée (codes.go)
}

// ActionKind est le type d'une action proposée au TD.
type ActionKind string

const (
	ActStartMatch ActionKind = "start_match" // lancer un match
	ActBye        ActionKind = "bye"         // donner un bye (ronde synchrone)
	ActDraw       ActionKind = "draw"        // tirer un tableau / des groupes (le tirage est joint)
	ActNextPhase  ActionKind = "next_phase"  // passer à la phase suivante
	ActFinish     ActionKind = "finish"      // clore le tournoi
	ActWait       ActionKind = "wait"        // rien à faire : attendre la fin de matchs en cours
	// ActCancelMatch : annuler un match devenu incohérent après une correction (reparation.go).
	// Comme toute action, elle est PROPOSÉE : rien n'est annulé d'office.
	ActCancelMatch ActionKind = "cancel_match"
)

// Action est une proposition du moteur ; le TD la confirme en ajoutant l'événement correspondant
// (voir Action.Event).
type Action struct {
	Kind    ActionKind `json:"kind"`
	Phase   int        `json:"phase"`
	Section string     `json:"section,omitempty"`
	Label   Label      `json:"label,omitempty"`
	Round   int        `json:"round,omitempty"` // ronde suisse (0 = sans objet) ; remplace la relecture du libellé
	Key     string     `json:"key,omitempty"`
	Match   MatchID    `json:"match,omitempty"` // cancel_match : le match à annuler
	A       PlayerID   `json:"a,omitempty"`
	B       PlayerID   `json:"b,omitempty"`
	Length  int        `json:"length,omitempty"`
	Table   int        `json:"table,omitempty"`
	Draw    *Draw      `json:"draw,omitempty"`
	Reason  ReasonCode `json:"reason,omitempty"`
	Until   time.Time  `json:"until,omitempty"` // waiting_batch : échéance du prochain lot
	// Warn signale ce que le moteur a remarqué sur CETTE proposition sans rien bloquer — la
	// fin attendue tombe pendant une pause, par exemple. Le TD décide (voir horaires.go).
	Warn WarningCode `json:"warn,omitempty"`
}

// Draw est le résultat d'un tirage (placement dans un tableau ou composition de groupes),
// matérialisé dans le journal pour que le rejeu ne dépende pas de l'algorithme.
type Draw struct {
	Slots  []PlayerID       `json:"slots,omitempty"`  // tableau : places du premier tour (BYE = exemption)
	Groups [][]PlayerID     `json:"groups,omitempty"` // groupes (GSL, poules)
	Lives  map[PlayerID]int `json:"lives,omitempty"`  // vies à l'entrée (information)
}
