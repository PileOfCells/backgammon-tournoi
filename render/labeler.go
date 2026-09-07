// Package render produit des composants d'affichage (SVG, HTML) à partir d'un état de tournoi :
// arbres de tableaux, tableau des vies, appariements en cours, grille des tables, feuille
// d'appariements imprimable, classement, et une page autonome qui les rassemble.
//
// Rien n'y est écrit en dur dans une langue. Le moteur ne produit que des CODES (voir codes.go) ;
// ce paquet les traduit par un Labeler que l'hôte fournit, et traduit de la même façon ses
// propres mots — « Joueur », « Table », « libre » — qui sont eux aussi des codes (Term). Un
// logiciel hôte qui parle neuf langues branche son Labeler et obtient ses pages ; French() est
// fourni pour la console, la démo et les tests.
package render

import (
	"fmt"

	"github.com/PileOfCells/backgammon-tournoi"
)

// Term est un mot du rendu lui-même — un en-tête de colonne, un état de table — par opposition
// aux codes du moteur. Il en faut : une feuille d'appariements a des colonnes, et une colonne a
// un titre. Les mettre ici plutôt que dans le HTML est ce qui rend la page traduisible.
type Term string

const (
	TermTournament Term = "tournament"
	TermPhase      Term = "phase"
	TermTodo       Term = "todo"
	TermRunning    Term = "running_matches"
	TermLives      Term = "lives"
	TermStandings  Term = "standings"
	TermTables     Term = "tables"
	TermPairings   Term = "pairing_sheet"

	TermPlayer    Term = "player"
	TermPlayers   Term = "players"
	TermClub      Term = "club"
	TermTable     Term = "table"
	TermMatch     Term = "match"
	TermPoints    Term = "points"
	TermSince     Term = "since"
	TermScore     Term = "score"
	TermRank      Term = "rank"
	TermPrize     Term = "prize"
	TermWins      Term = "wins"
	TermOpponents Term = "opponents"
	TermWaiting   Term = "waiting"
	TermState     Term = "state"
	TermRound     Term = "round"

	TermFree        Term = "free"
	TermBusy        Term = "busy"
	TermForfeit     Term = "forfeit"
	TermEliminated  Term = "eliminated"
	TermBye         Term = "bye"
	TermUnknown     Term = "unknown"
	TermUnavailable Term = "unavailable"
	TermReserved    Term = "reserved"
	TermLosses      Term = "losses"
	TermPlayed      Term = "played"
	TermInProgress  Term = "in_progress"
	TermNoMatch     Term = "no_match"
	TermVersus      Term = "versus"
	TermCancel      Term = "cancel"
)

// Labeler traduit les codes du moteur, et les mots du rendu, dans la langue de l'utilisateur.
// Aucune méthode ne renvoie jamais de code brut : c'est le contrat.
type Labeler interface {
	Label(tournoi.Label) string
	Note(tournoi.Note) string
	Reason(tournoi.ReasonCode) string
	Warn(tournoi.WarningCode) string
	SectionName(string) string
	PhaseName(tournoi.PhaseConfig) string
	// Term rend un mot du rendu. Count sert aux formes qui en dépendent (« 1 joueur »,
	// « 3 joueurs ») ; les rendus qui n'en ont pas besoin l'ignorent.
	Term(t Term, count int) string
}

// French est le Labeler français : il délègue au rendu de fr.go pour les codes du moteur et
// porte ici les mots du rendu. C'est le seul endroit du paquet où une phrase est écrite.
func French() Labeler { return french{} }

type french struct{}

func (french) Label(l tournoi.Label) string           { return l.String() }
func (french) Note(n tournoi.Note) string             { return n.String() }
func (french) Reason(r tournoi.ReasonCode) string     { return r.String() }
func (french) Warn(w tournoi.WarningCode) string      { return tournoi.Warning{Code: w}.String() }
func (french) SectionName(s string) string            { return tournoi.SectionName(s) }
func (french) PhaseName(p tournoi.PhaseConfig) string { return tournoi.PhaseName(p) }

var motsFrançais = map[Term]string{
	TermTournament: "Tournoi", TermPhase: "Phase", TermTodo: "À faire",
	TermRunning: "Matchs en cours", TermLives: "Vies", TermStandings: "Classement",
	TermTables: "Tables", TermPairings: "Feuille d'appariements",
	TermPlayer: "Joueur", TermPlayers: "joueurs", TermClub: "Club", TermTable: "Table",
	TermMatch: "Match", TermPoints: "Pts", TermSince: "Depuis", TermScore: "Score",
	TermRank: "#", TermPrize: "Prix", TermWins: "V", TermOpponents: "Adversaires rencontrés",
	TermWaiting: "Attente", TermState: "État", TermRound: "Ronde",
	TermFree: "libre", TermBusy: "en match", TermForfeit: "forfait",
	TermEliminated: "Éliminés", TermBye: "exempt", TermUnknown: "…",
	TermUnavailable: "indisponible", TermReserved: "réservée", TermLosses: "défaite(s)",
	TermPlayed: "matchs joués", TermInProgress: "en cours", TermNoMatch: "aucun match",
	TermVersus: "contre", TermCancel: "Annuler",
}

func (french) Term(t Term, count int) string {
	s, ok := motsFrançais[t]
	if !ok {
		return string(t)
	}
	switch t {
	case TermPlayers, TermPlayed:
		return fmt.Sprintf("%d %s", count, s)
	case TermLosses:
		return fmt.Sprintf("%d %s", count, s)
	}
	return s
}
