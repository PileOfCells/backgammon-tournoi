package tournoi

import (
	"encoding/json"
	"fmt"
	"time"
)

// JournalVersion est la version du format du journal écrite dans chaque nouvel événement.
// 0 (champ absent) désigne les journaux antérieurs aux codes structurés, toujours rejouables.
const JournalVersion = 1

// EventKind est le type d'un événement du journal.
type EventKind string

const (
	EvCreated         EventKind = "created"          // Config, Seed
	EvPlayerAdded     EventKind = "player_added"     // Player
	EvPlayerWithdrawn EventKind = "player_withdrawn" // Player.ID ; AfterCurrent = finit son match
	EvMatchStarted    EventKind = "match_started"    // MatchID, Phase, Section, Label, Key, A, B, Length, Table
	EvResult          EventKind = "result"           // MatchID, Winner, ScoreA, ScoreB, Forfeit
	EvResultCorrected EventKind = "result_corrected" // idem, remplace le résultat précédent
	EvMatchCancelled  EventKind = "match_cancelled"  // MatchID (match lancé par erreur)
	EvBye             EventKind = "bye"              // Phase, Player, Label
	EvDraw            EventKind = "draw"             // Phase, Section, Draw
	EvNextPhase       EventKind = "next_phase"       // passage à la phase suivante
	EvLengthChanged   EventKind = "length_changed"   // Phase, Length (matchs futurs de la phase)
	EvConfigChanged   EventKind = "config_changed"   // Config (la configuration ENTIÈRE, voir reconfig.go)
	EvReopened        EventKind = "reopened"         // le tournoi clos est rouvert ; Final sera recalculé
	EvTableChanged    EventKind = "table_changed"    // MatchID, Table (match en cours déplacé)
	EvFinished        EventKind = "finished"
	EvNote            EventKind = "note" // Text (annotation libre du TD)
)

// Event est une entrée du journal. Les champs inutiles pour un type restent vides.
type Event struct {
	Seq          int       `json:"seq"`
	Version      int       `json:"version,omitempty"` // format du journal ; 0 = avant les codes structurés
	Kind         EventKind `json:"kind"`
	Time         time.Time `json:"time"`
	Config       *Config   `json:"config,omitempty"`
	Seed         int64     `json:"seed,omitempty"`
	Player       *Player   `json:"player,omitempty"`
	ID           PlayerID  `json:"player_id,omitempty"`
	MatchID      MatchID   `json:"match_id,omitempty"`
	Phase        int       `json:"phase,omitempty"`
	Section      string    `json:"section,omitempty"`
	Label        Label     `json:"label,omitempty"`
	Round        int       `json:"round,omitempty"`
	Key          string    `json:"key,omitempty"`
	A            PlayerID  `json:"a,omitempty"`
	B            PlayerID  `json:"b,omitempty"`
	Length       int       `json:"length,omitempty"`
	Table        int       `json:"table,omitempty"`
	Winner       PlayerID  `json:"winner,omitempty"`
	ScoreA       int       `json:"score_a,omitempty"`
	ScoreB       int       `json:"score_b,omitempty"`
	Forfeit      bool      `json:"forfeit,omitempty"`
	AfterCurrent bool      `json:"after_current,omitempty"` // retrait différé : le joueur finit son match
	Draw         *Draw     `json:"draw,omitempty"`
	Text         string    `json:"text,omitempty"`
	// Slot : sur player_added, la clé de la place d'exemption qu'un retardataire vient prendre
	// dans un tableau déjà tiré (voir retardataire.go). Vide pour une inscription ordinaire.
	Slot string `json:"slot,omitempty"`
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
	ev := Event{Version: JournalVersion, Time: now, Phase: a.Phase, Section: a.Section, Label: a.Label, Round: a.Round, Key: a.Key}
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
	case ActCancelMatch:
		ev.Kind = EvMatchCancelled
		ev.MatchID = a.Match
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
	return Event{Version: JournalVersion, Kind: EvResult, Time: now, MatchID: id, Winner: winner, ScoreA: scoreA, ScoreB: scoreB}
}

// upgraded convertit un événement d'un journal antérieur aux codes structurés (Version 0) vers
// la forme courante : le libellé texte devient un code quand il est reconnaissable, et le
// numéro de ronde passe du texte au champ Round. Un événement déjà à jour est renvoyé tel quel.
// La conversion a lieu à la LECTURE du journal, si bien que le reste du moteur ne connaît que
// les codes.
func (e Event) upgraded() Event {
	if e.Version >= 1 || e.Label.Text == "" {
		return e
	}
	if r := legacyRound(e.Label.Text); r > 0 {
		if e.Round == 0 {
			e.Round = r
		}
		e.Label = Label{Kind: LabelRound, N: r}
	}
	return e
}

// ---- Constructeurs d'événements ----
//
// Tout événement porte la version du format. Ces constructeurs existent pour qu'on ne puisse pas
// l'oublier : un événement fabriqué à la main sans Version serait relu comme un journal ancien.

// PlayerAddedEvent : inscription d'un joueur.
func PlayerAddedEvent(p Player, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvPlayerAdded, Time: now, Player: &p}
}

// PlayerAddedAtSlotEvent : inscription d'un retardataire sur une place d'exemption libre d'un
// tableau déjà tiré (State.FreeSlots les énumère). Le tirage n'est pas refait : la place est
// occupée là où elle est. Une place déjà jouée est refusée par Apply.
func PlayerAddedAtSlotEvent(p Player, slot Slot, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvPlayerAdded, Time: now, Player: &p,
		Phase: slot.Phase, Section: slot.Section, Slot: slot.Key}
}

// PlayerWithdrawnEvent : retrait immédiat d'un joueur. Ses matchs en cours sont perdus par
// forfait, et ses matchs de graphe non lancés aussi.
func PlayerWithdrawnEvent(id PlayerID, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvPlayerWithdrawn, Time: now, ID: id}
}

// PlayerWithdrawnAfterCurrentEvent : retrait différé. Le joueur n'est plus apparié, mais le
// match qu'il joue va à son terme — le cas de qui doit partir à 18 h.
func PlayerWithdrawnAfterCurrentEvent(id PlayerID, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvPlayerWithdrawn, Time: now, ID: id, AfterCurrent: true}
}

// ForfeitEvent : un joueur ne se présente pas pour CE match, sans quitter le tournoi. Il suit
// ensuite le chemin d'un perdant ordinaire (la consolante, par exemple).
func ForfeitEvent(id MatchID, winner PlayerID, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvResult, Time: now, MatchID: id, Winner: winner, Forfeit: true}
}

// WithNote attache une remarque à un événement de résultat (« tombé au temps », « abandon :
// … »). Rare, et irremplaçable quand elle sert.
func (e Event) WithNote(text string) Event {
	e.Text = text
	return e
}

// CorrectionEvent : correction du résultat d'un match déjà terminé. Le journal n'est jamais
// modifié : la correction est un événement de plus, et l'état est recalculé.
func CorrectionEvent(id MatchID, winner PlayerID, scoreA, scoreB int, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvResultCorrected, Time: now, MatchID: id,
		Winner: winner, ScoreA: scoreA, ScoreB: scoreB}
}

// CancelEvent : annulation d'un match lancé par erreur.
func CancelEvent(id MatchID, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvMatchCancelled, Time: now, MatchID: id}
}

// LengthChangedEvent : nouvelle longueur pour les matchs à venir d'une phase.
func LengthChangedEvent(phase, length int, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvLengthChanged, Time: now, Phase: phase, Length: length}
}

// ConfigChangedEvent : nouvelle configuration du tournoi, ENTIÈRE. Le moteur la valide et
// refuse ce qui changerait le format d'une phase commencée ou terminée, ou retirerait une phase
// ouverte (reconfig.go). Tout le reste est admis : la bascule, les longueurs à venir, les
// tables, les pauses, la dotation, une phase ajoutée après la phase courante.
//
// EvLengthChanged reste lu pour les journaux existants ; il n'est plus le seul moyen de changer
// une longueur.
func ConfigChangedEvent(cfg Config, now time.Time) Event {
	c := cfg.clone()
	return Event{Version: JournalVersion, Kind: EvConfigChanged, Time: now, Config: &c}
}

// ReopenedEvent : un tournoi clos est rouvert, parce qu'un résultat était faux. Le classement
// final est effacé et sera recalculé à la clôture suivante — le journal, lui, garde tout.
func ReopenedEvent(now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvReopened, Time: now}
}

// TableChangedEvent : un match en cours change de table (bruit, lumière, retransmission).
func TableChangedEvent(id MatchID, table int, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvTableChanged, Time: now, MatchID: id, Table: table}
}

// NoteEvent : annotation libre du directeur de tournoi, horodatée.
func NoteEvent(text string, now time.Time) Event {
	return Event{Version: JournalVersion, Kind: EvNote, Time: now, Text: text}
}
