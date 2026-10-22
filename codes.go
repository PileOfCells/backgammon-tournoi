package tournoi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// Codes structurés : le moteur n'émet aucune phrase destinée à l'affichage.
//
// Un logiciel hôte affiche un tournoi dans la langue de son utilisateur ; blunderDB en parle
// neuf. Les libellés de match, les notes de classement, les avertissements et les raisons
// d'attente sont donc des données — un code et ses paramètres — et jamais du texte. Le rendu
// français vit dans fr.go et sert la console, la démo et le paquet render.
//
// Ces valeurs entrent dans le journal : les changer casse les journaux existants. La version
// du format est Event.Version (voir events.go).

// ---- Libellés ----

// LabelKind est le code d'un libellé de match, de tirage ou de phase.
type LabelKind string

const (
	LabelNone LabelKind = ""

	// Suisse
	LabelSwissGroup LabelKind = LabelKind(rules.LabelSwissGroup) // Losses, Match : « 1 défaite, match 3 »
	LabelRound      LabelKind = LabelKind(rules.LabelRound)      // N : « Ronde 3 »
	LabelRematch    LabelKind = LabelKind(rules.LabelRematch)
	LabelCrossed    LabelKind = LabelKind(rules.LabelCrossed) // match croisé de secours

	// Tableaux
	LabelBracketRound       LabelKind = LabelKind(rules.LabelBracketRound) // N : « Tour 2 »
	LabelFinal              LabelKind = LabelKind(rules.LabelFinal)
	LabelSemiFinal          LabelKind = LabelKind(rules.LabelSemiFinal)
	LabelQuarterFinal       LabelKind = LabelKind(rules.LabelQuarterFinal)
	LabelMainDraw           LabelKind = LabelKind(rules.LabelMainDraw) // Sub : « Principal, quart de finale »
	LabelConsolationRound   LabelKind = LabelKind(rules.LabelConsolationRound)
	LabelConsolationFinal   LabelKind = LabelKind(rules.LabelConsolationFinal)
	LabelLastChance         LabelKind = LabelKind(rules.LabelLastChance) // Sub facultatif
	LabelGrandFinal         LabelKind = LabelKind(rules.LabelGrandFinal)
	LabelGrandFinalRecharge LabelKind = LabelKind(rules.LabelGrandFinalRecharge)

	// GSL
	LabelOpening      LabelKind = LabelKind(rules.LabelOpening)
	LabelWinnersMatch LabelKind = LabelKind(rules.LabelWinnersMatch)
	LabelLosersMatch  LabelKind = LabelKind(rules.LabelLosersMatch)
	LabelDecider      LabelKind = LabelKind(rules.LabelDecider)
	LabelSingleMatch  LabelKind = LabelKind(rules.LabelSingleMatch)
	LabelSingleElim   LabelKind = LabelKind(rules.LabelSingleElim)
	LabelInBlock      LabelKind = LabelKind(rules.LabelInBlock) // N = bloc, Section, Sub

	// Poules
	LabelPoolRound    LabelKind = LabelKind(rules.LabelPoolRound) // N
	LabelInSection    LabelKind = LabelKind(rules.LabelInSection) // Section, Sub
	LabelBarrage      LabelKind = LabelKind(rules.LabelBarrage)   // Section
	LabelBarrageCross LabelKind = LabelKind(rules.LabelBarrageCross)
	LabelDrawPools    LabelKind = LabelKind(rules.LabelDrawPools)   // N = nombre de poules
	LabelDrawBarrage  LabelKind = LabelKind(rules.LabelDrawBarrage) // Section, Players, Spots
	// LabelRepechage : repêchage proposé dans la poule Section ; Players = nombre de candidats
	// ex æquo (1 : un seul suivant ; davantage : le TD choisit, voir repechage.go).
	LabelRepechage     LabelKind = "repechage"
	LabelDrawBracket   LabelKind = LabelKind(rules.LabelDrawBracket) // N = places
	LabelDrawBlock     LabelKind = LabelKind(rules.LabelDrawBlock)   // N = bloc, Groups
	LabelPhase         LabelKind = LabelKind(rules.LabelPhase)       // Phase : le nom vient de la configuration
	LabelPoolName      LabelKind = "pool_name"                       // Text : la lettre de la poule
	LabelBarrageName   LabelKind = "barrage_name"                    // Sub = LabelPoolName
	LabelSectionMain   LabelKind = "section_main"
	LabelSectionConso  LabelKind = "section_conso"
	LabelSectionLast   LabelKind = "section_last"
	LabelSectionFinals LabelKind = "section_finals"
)

// Label est un libellé structuré. Les champs inutiles pour un code restent vides ; Sub compose
// un libellé dans un autre (« Bloc 2, groupe B : match décisif »).
type Label struct {
	Kind    LabelKind `json:"kind,omitempty"`
	N       int       `json:"n,omitempty"`       // ronde, tour, bloc, nombre de places, de poules
	Losses  int       `json:"losses,omitempty"`  // suisse continu : groupe de défaites
	Match   int       `json:"match,omitempty"`   // suisse continu : n-ième match du joueur
	Section string    `json:"section,omitempty"` // nom interne de la section concernée
	Text    string    `json:"text,omitempty"`    // seul texte admis : ce que le TD a lui-même saisi
	Players int       `json:"players,omitempty"` // barrage : joueurs à départager
	Spots   int       `json:"spots,omitempty"`   // barrage : places à attribuer
	Sub     *Label    `json:"sub,omitempty"`
}

// Empty indique un libellé absent.
func (l Label) Empty() bool { return l.Kind == LabelNone }

// ---- Notes de classement ----

// NoteKind est le code d'une note de classement.
type NoteKind string

const (
	NoteNone          NoteKind = ""
	NoteWinner        NoteKind = NoteKind(rules.NoteWinner)
	NoteFinalist      NoteKind = NoteKind(rules.NoteFinalist)
	NoteAlive         NoteKind = NoteKind(rules.NoteAlive)         // Lives
	NoteRecord        NoteKind = NoteKind(rules.NoteRecord)        // Wins, Losses
	NoteForfeit       NoteKind = NoteKind(rules.NoteForfeit)       // match perdu par forfait (classement d'une section) ; journaux antérieurs : retiré
	NoteWithdrawn     NoteKind = NoteKind(rules.NoteWithdrawn)     // retiré, classé sur son parcours : Wins, Losses, Lives ; tableau : Section + Sub ; poule : Section + Wins
	NoteRunning       NoteKind = NoteKind(rules.NoteRunning)       // encore en course, tableau non terminé
	NoteAwaitingDraw  NoteKind = NoteKind(rules.NoteAwaitingDraw)  // tableau pas encore tiré
	NoteUnranked      NoteKind = NoteKind(rules.NoteUnranked)      // aucune sortie enregistrée
	NoteSectionExit   NoteKind = NoteKind(rules.NoteSectionExit)   // Section + Sub : sorti de cette section là — pas forcément éliminé (State.Statuses)
	NoteSectionWinner NoteKind = NoteKind(rules.NoteSectionWinner) // Section
	NotePoolRecord    NoteKind = NoteKind(rules.NotePoolRecord)    // Section, Wins, Qualified
)

// Note est la note de classement d'un joueur : un code et ses paramètres, jamais une phrase.
type Note struct {
	Kind      NoteKind `json:"kind,omitempty"`
	Wins      int      `json:"wins,omitempty"`
	Losses    int      `json:"losses,omitempty"`
	Lives     int      `json:"lives,omitempty"`
	Section   string   `json:"section,omitempty"`
	Qualified bool     `json:"qualified,omitempty"`
	Sub       *Label   `json:"sub,omitempty"`
}

// ---- Avertissements ----

// WarningCode est le code d'une incohérence signalée par le moteur.
type WarningCode string

const (
	// WarnBracketWrongPlayers : un match de graphe a été joué par d'autres joueurs que ceux
	// que la place attendait (typiquement après la correction d'un résultat antérieur).
	WarnBracketWrongPlayers WarningCode = "bracket_wrong_players"
	// WarnScoreOverLength : un score dépasse la longueur annoncée du match.
	WarnScoreOverLength WarningCode = "score_over_length"
	// WarnEndsInBreak : la fin attendue d'un match proposé tombe dans une pause.
	WarnEndsInBreak WarningCode = "ends_in_break"
	// WarnSlowMatch : un match en cours dépasse la durée attendue.
	WarnSlowMatch WarningCode = "slow_match"
	// WarnTableShared : deux matchs en cours sur la même table (Table), Match et Other. Le
	// déplacement qui l'a produit a été accepté : le moteur avertit, il ne bloque pas.
	// L'avertissement tombe dès que l'un des deux matchs se termine ou change de table.
	WarnTableShared WarningCode = "table_shared"
	// WarnCorrectionEliminatesRunning : dans un suisse, une correction (ou une annulation) a fait
	// tomber à 0 vie un joueur (Player) qui joue en ce moment le match Match. L'annulation de ce
	// match est proposée (cancel_match). Dérivé : tombe quand le match finit ou est annulé.
	WarnCorrectionEliminatesRunning WarningCode = "correction_eliminates_running"
	// WarnCorrectionRevives : dans un suisse, une correction (ou une annulation) a rendu une vie à
	// un joueur (Player) qu'elle avait éliminé. Il redevient appariable ; l'avertissement reste
	// jusqu'à ce qu'il rejoue, ou qu'il soit de nouveau éliminé.
	WarnCorrectionRevives WarningCode = "correction_revives"
)

// Warning est une incohérence : un code et de quoi la situer. Rien n'est bloqué par un
// avertissement — le TD décide (voir README, « le moteur propose, le TD décide »).
type Warning struct {
	Code      WarningCode `json:"code"`
	Match     MatchID     `json:"match,omitempty"`
	Section   string      `json:"section,omitempty"`
	Label     Label       `json:"label,omitempty"`
	A         PlayerID    `json:"a,omitempty"`
	B         PlayerID    `json:"b,omitempty"`
	ExpectedA PlayerID    `json:"expected_a,omitempty"`
	ExpectedB PlayerID    `json:"expected_b,omitempty"`
	Length    int         `json:"length,omitempty"`
	ScoreA    int         `json:"score_a,omitempty"`
	ScoreB    int         `json:"score_b,omitempty"`
	Player    PlayerID    `json:"player,omitempty"` // correction_* : le joueur concerné
	Table     int         `json:"table,omitempty"`  // table_shared : la table disputée
	Other     MatchID     `json:"other,omitempty"`  // table_shared : l'autre match sur la table
}

// ---- Informations ----

// InfoCode est le code d'une information : ni une incohérence à corriger, ni une action à
// confirmer, mais un fait que le TD doit voir — le sort d'un joueur inscrit qui ne joue encore
// nulle part.
type InfoCode string

const (
	// InfoEntersAt : le joueur entrera là. Phase et Section (avec le libellé) disent où.
	InfoEntersAt InfoCode = "enters_at"
	// InfoNoEntry : le joueur est inscrit mais rien ne l'admet — aucune place d'exemption
	// libre, aucune phase à venir ouverte à tous. Le dire vaut mieux que le laisser disparaître
	// de l'affichage.
	InfoNoEntry InfoCode = "no_entry"
)

// Info est une information destinée au TD : un code et de quoi la situer.
type Info struct {
	Code    InfoCode `json:"code"`
	Player  PlayerID `json:"player,omitempty"`
	Phase   int      `json:"phase,omitempty"`
	Section string   `json:"section,omitempty"`
	Label   Label    `json:"label,omitempty"`
}

// ---- Raisons d'attente ----

// ReasonCode dit pourquoi le moteur ne propose rien à faire.
type ReasonCode string

const (
	ReasonNone           ReasonCode = ""
	ReasonMatchesRunning ReasonCode = "matches_running"
	ReasonNoPairing      ReasonCode = "no_pairing"
	ReasonWaitingBatch   ReasonCode = "waiting_batch"
	ReasonWaitingTable   ReasonCode = "waiting_table"
	// ReasonPlayerUnavailable : un joueur est indisponible (player_unavailable). Sur une attente
	// (A = le joueur, Until ou Round = son retour) dans un suisse ; sur la proposition de match
	// d'un graphe, qui reste dans la file sans table.
	ReasonPlayerUnavailable ReasonCode = "player_unavailable"
	// ReasonPlayerBusy : un joueur joue en ce moment dans une autre épreuve de la salle
	// (External.BusyPlayers). Sur une attente (A = le joueur) dans un suisse ; sur la proposition
	// de match d'un graphe, qui reste dans la file sans table. Jamais écrit au journal.
	ReasonPlayerBusy ReasonCode = "player_busy"
)

// ---- Noms de section ----
//
// Un nom de section est un IDENTIFIANT : il voyage dans le journal et dans la base du logiciel
// hôte, et sert de clé à ph.section(). Il n'est jamais affiché tel quel — sectionName (fr.go)
// ou l'hôte le rend.

const (
	secMain       = rules.SectionMain
	secConso      = rules.SectionConso
	secLast       = rules.SectionLast
	secGrandFinal = rules.SectionGrandFinal

	poolPrefix    = "poule:"
	barragePrefix = "barrage:"

	// Kinds de section (Section.Kind) : la famille de graphe, indépendante du nom.
	secKindGSL     = rules.SectionGSL
	secKindSE      = rules.SectionSE
	secKindPool    = rules.SectionPool
	secKindBarrage = rules.SectionBarrage
)

// poolLetter : la lettre d'une section de poule.
func poolLetter(name string) (string, bool) {
	if strings.HasPrefix(name, poolPrefix) {
		return strings.TrimPrefix(name, poolPrefix), true
	}
	return "", false
}

// barragePoolLetter : la lettre de la poule d'une section de barrage.
func barragePoolLetter(name string) (string, bool) {
	if strings.HasPrefix(name, barragePrefix) {
		return strings.TrimPrefix(name, barragePrefix), true
	}
	return "", false
}

// UnmarshalJSON accepte les deux formes d'un libellé : la structure (journaux à partir de la
// version 1) et la chaîne des journaux antérieurs, conservée telle quelle dans Text pour que
// rien ne soit perdu. upgrade (events.go) la convertit en code quand elle est reconnaissable.
func (l *Label) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = Label{Text: s}
		return nil
	}
	type brut Label // évite la récursion
	var v brut
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*l = Label(v)
	return nil
}

// legacyRound lit le numéro dans un libellé texte « Ronde k » d'un journal antérieur aux codes
// structurés. C'est le SEUL endroit du moteur qui relit un libellé, et il ne sert qu'à la
// conversion des vieux journaux : les journaux courants portent le numéro dans Event.Round.
func legacyRound(text string) int {
	var r int
	if n, _ := fmt.Sscanf(text, "Ronde %d", &r); n == 1 {
		return r
	}
	return 0
}
