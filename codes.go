package tournoi

import (
	"encoding/json"
	"fmt"
	"strings"
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
	LabelSwissGroup LabelKind = "swiss_group" // Losses, Match : « 1 défaite, match 3 »
	LabelRound      LabelKind = "round"       // N : « Ronde 3 »
	LabelRematch    LabelKind = "rematch"
	LabelCrossed    LabelKind = "crossed" // match croisé de secours

	// Tableaux
	LabelBracketRound       LabelKind = "bracket_round" // N : « Tour 2 »
	LabelFinal              LabelKind = "final"
	LabelSemiFinal          LabelKind = "semi_final"
	LabelQuarterFinal       LabelKind = "quarter_final"
	LabelMainDraw           LabelKind = "main_draw" // Sub : « Principal, quart de finale »
	LabelConsolationRound   LabelKind = "consolation_round"
	LabelConsolationFinal   LabelKind = "consolation_final"
	LabelLastChance         LabelKind = "last_chance" // Sub facultatif
	LabelGrandFinal         LabelKind = "grand_final"
	LabelGrandFinalRecharge LabelKind = "grand_final_recharge"

	// GSL
	LabelOpening      LabelKind = "opening"
	LabelWinnersMatch LabelKind = "winners_match"
	LabelLosersMatch  LabelKind = "losers_match"
	LabelDecider      LabelKind = "decider"
	LabelSingleMatch  LabelKind = "single_match"
	LabelSingleElim   LabelKind = "single_elimination"
	LabelInBlock      LabelKind = "in_block" // N = bloc, Section, Sub

	// Poules
	LabelPoolRound     LabelKind = "pool_round" // N
	LabelInSection     LabelKind = "in_section" // Section, Sub
	LabelBarrage       LabelKind = "barrage"    // Section
	LabelBarrageCross  LabelKind = "barrage_crossed"
	LabelDrawPools     LabelKind = "draw_pools"   // N = nombre de poules
	LabelDrawBarrage   LabelKind = "draw_barrage" // Section, Players, Spots
	LabelDrawBracket   LabelKind = "draw_bracket" // N = places
	LabelDrawBlock     LabelKind = "draw_block"   // N = bloc, Groups
	LabelPhase         LabelKind = "phase"        // Phase : le nom vient de la configuration
	LabelPoolName      LabelKind = "pool_name"    // Text : la lettre de la poule
	LabelBarrageName   LabelKind = "barrage_name" // Sub = LabelPoolName
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

// with renvoie une copie de l portant sub comme sous-libellé.
func (l Label) with(sub Label) Label {
	if sub.Empty() {
		return l
	}
	c := sub
	l.Sub = &c
	return l
}

// roundLabel : libellé du tour r d'un tableau de `rounds` tours (0 = premier tour).
func roundLabel(r, rounds int) Label {
	switch rounds - r {
	case 1:
		return Label{Kind: LabelFinal}
	case 2:
		return Label{Kind: LabelSemiFinal}
	case 3:
		return Label{Kind: LabelQuarterFinal}
	}
	return Label{Kind: LabelBracketRound, N: r + 1}
}

// ---- Notes de classement ----

// NoteKind est le code d'une note de classement.
type NoteKind string

const (
	NoteNone          NoteKind = ""
	NoteWinner        NoteKind = "winner"
	NoteFinalist      NoteKind = "finalist"
	NoteAlive         NoteKind = "alive"          // Lives
	NoteRecord        NoteKind = "record"         // Wins, Losses
	NoteForfeit       NoteKind = "forfeit"        // retiré du tournoi
	NoteRunning       NoteKind = "running"        // encore en course, tableau non terminé
	NoteAwaitingDraw  NoteKind = "awaiting_draw"  // tableau pas encore tiré
	NoteUnranked      NoteKind = "unranked"       // aucune sortie enregistrée
	NoteSectionExit   NoteKind = "section_exit"   // Section + Sub : éliminé là
	NoteSectionWinner NoteKind = "section_winner" // Section
	NotePoolRecord    NoteKind = "pool_record"    // Section, Wins, Qualified
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
)

// ---- Noms de section ----
//
// Un nom de section est un IDENTIFIANT : il voyage dans le journal et dans la base du logiciel
// hôte, et sert de clé à ph.section(). Il n'est jamais affiché tel quel — sectionName (fr.go)
// ou l'hôte le rend.

const (
	secMain       = "main"
	secConso      = "conso"
	secLast       = "last"
	secGrandFinal = "gf"

	poolPrefix    = "poule:"
	barragePrefix = "barrage:"

	// Kinds de section (Section.Kind) : la famille de graphe, indépendante du nom.
	secKindGSL     = "gsl"
	secKindSE      = "se"
	secKindPool    = "poule"
	secKindBarrage = "barrage"
)

// poolSectionName : nom de la i-ième poule (0 → "poule:A").
func poolSectionName(i int) string { return poolPrefix + string(rune('A'+i)) }

// barrageSectionName : nom du barrage de la poule donnée.
func barrageSectionName(pool string) string {
	return barragePrefix + strings.TrimPrefix(pool, poolPrefix)
}

// poolOfBarrage : la poule dont ce barrage départage les ex æquo.
func poolOfBarrage(barrage string) string {
	return poolPrefix + strings.TrimPrefix(barrage, barragePrefix)
}

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
