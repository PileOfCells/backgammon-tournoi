package tournoi

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventKind est le type d'un événement du journal.
type EventKind string

const (
	EvCreated         EventKind = "created"          // Config, Seed
	EvPlayerAdded     EventKind = "player_added"     // Player
	EvPlayerWithdrawn EventKind = "player_withdrawn" // Player.ID (forfait général)
	EvMatchStarted    EventKind = "match_started"    // MatchID, Phase, Section, Label, Key, A, B, Length, Table
	EvResult          EventKind = "result"           // MatchID, Winner, ScoreA, ScoreB, Forfeit
	EvResultCorrected EventKind = "result_corrected" // idem, remplace le résultat précédent
	EvMatchCancelled  EventKind = "match_cancelled"  // MatchID (match lancé par erreur)
	EvBye             EventKind = "bye"              // Phase, Player, Label
	EvDraw            EventKind = "draw"             // Phase, Section, Draw
	EvNextPhase       EventKind = "next_phase"       // passage à la phase suivante
	EvLengthChanged   EventKind = "length_changed"   // Phase, Length (matchs futurs de la phase)
	EvFinished        EventKind = "finished"
	EvNote            EventKind = "note" // Text (annotation libre du TD)
)

// Event est une entrée du journal. Les champs inutiles pour un type restent vides.
type Event struct {
	Seq     int       `json:"seq"`
	Kind    EventKind `json:"kind"`
	Time    time.Time `json:"time"`
	Config  *Config   `json:"config,omitempty"`
	Seed    int64     `json:"seed,omitempty"`
	Player  *Player   `json:"player,omitempty"`
	ID      PlayerID  `json:"player_id,omitempty"`
	MatchID MatchID   `json:"match_id,omitempty"`
	Phase   int       `json:"phase,omitempty"`
	Section string    `json:"section,omitempty"`
	Label   string    `json:"label,omitempty"`
	Key     string    `json:"key,omitempty"`
	A       PlayerID  `json:"a,omitempty"`
	B       PlayerID  `json:"b,omitempty"`
	Length  int       `json:"length,omitempty"`
	Table   int       `json:"table,omitempty"`
	Winner  PlayerID  `json:"winner,omitempty"`
	ScoreA  int       `json:"score_a,omitempty"`
	ScoreB  int       `json:"score_b,omitempty"`
	Forfeit bool      `json:"forfeit,omitempty"`
	Draw    *Draw     `json:"draw,omitempty"`
	Text    string    `json:"text,omitempty"`
}

// Journal est la liste ordonnée des événements d'un tournoi.
type Journal []Event

// MarshalJSON / UnmarshalJSON : le journal est un simple tableau JSON.
func (j Journal) Bytes() ([]byte, error) { return json.MarshalIndent(j, "", " ") }

// ParseJournal lit un journal JSON.
func ParseJournal(b []byte) (Journal, error) {
	var j Journal
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	return j, nil
}

// EventFromAction construit l'événement qui confirme une action proposée.
// Pour ActStartMatch, l'identifiant de match est attribué par le moteur (prochain numéro).
func (s *State) EventFromAction(a Action, now time.Time) (Event, error) {
	ev := Event{Time: now, Phase: a.Phase, Section: a.Section, Label: a.Label, Key: a.Key}
	switch a.Kind {
	case ActStartMatch:
		ev.Kind = EvMatchStarted
		ev.MatchID = s.nextMatchID()
		ev.A, ev.B, ev.Length, ev.Table = a.A, a.B, a.Length, a.Table
	case ActBye:
		ev.Kind = EvBye
		ev.ID = a.A
	case ActDraw:
		ev.Kind = EvDraw
		ev.Draw = a.Draw
	case ActNextPhase:
		ev.Kind = EvNextPhase
	case ActFinish:
		ev.Kind = EvFinished
	default:
		return ev, fmt.Errorf("action %q sans événement associé", a.Kind)
	}
	return ev, nil
}

// ResultEvent construit l'événement de résultat d'un match.
func ResultEvent(id MatchID, winner PlayerID, scoreA, scoreB int, now time.Time) Event {
	return Event{Kind: EvResult, Time: now, MatchID: id, Winner: winner, ScoreA: scoreA, ScoreB: scoreB}
}
