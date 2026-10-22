package rules

import "strings"

// Codes, jamais de phrases : un libellé de match (Label) et une note de classement (Note) sont
// un code et ses paramètres. Chaque hôte les traduit dans la langue de son utilisateur. Les
// valeurs sont celles du journal du moteur, qui les reprend telles quelles : les changer casse
// les journaux existants.

// LabelKind est le code d'un libellé de match ou de tirage.
type LabelKind string

const (
	LabelNone LabelKind = ""

	// Suisse
	LabelSwissGroup LabelKind = "swiss_group" // Losses, Match : groupe de défaites, n-ième match du joueur
	LabelRound      LabelKind = "round"       // N : ronde
	LabelRematch    LabelKind = "rematch"
	LabelCrossed    LabelKind = "crossed" // match croisé de secours

	// Tableaux
	LabelBracketRound       LabelKind = "bracket_round" // N : tour
	LabelFinal              LabelKind = "final"
	LabelSemiFinal          LabelKind = "semi_final"
	LabelQuarterFinal       LabelKind = "quarter_final"
	LabelMainDraw           LabelKind = "main_draw" // Sub : le tour dans le tableau principal
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
	LabelPoolRound    LabelKind = "pool_round" // N
	LabelInSection    LabelKind = "in_section" // Section, Sub
	LabelBarrage      LabelKind = "barrage"    // Section
	LabelBarrageCross LabelKind = "barrage_crossed"

	// Tirages
	LabelDrawPools   LabelKind = "draw_pools"   // N = nombre de poules
	LabelDrawBarrage LabelKind = "draw_barrage" // Section, Players, Spots
	LabelDrawBracket LabelKind = "draw_bracket" // N = places ; Sub = LabelPhase
	LabelDrawBlock   LabelKind = "draw_block"   // N = bloc, Players = nombre de groupes
	LabelPhase       LabelKind = "phase"        // Text : le nom de la phase, saisi par l'organisateur
)

// Label est un libellé structuré. Les champs inutiles pour un code restent vides ; Sub compose
// un libellé dans un autre (« bloc 2, groupe B : match décisif »).
type Label struct {
	Kind    LabelKind `json:"kind,omitempty"`
	N       int       `json:"n,omitempty"`       // ronde, tour, bloc, nombre de places, de poules
	Losses  int       `json:"losses,omitempty"`  // suisse continu : groupe de défaites
	Match   int       `json:"match,omitempty"`   // suisse continu : n-ième match du joueur
	Section string    `json:"section,omitempty"` // identifiant de la section concernée
	Text    string    `json:"text,omitempty"`    // seul texte admis : ce que l'organisateur a saisi
	Players int       `json:"players,omitempty"` // barrage : joueurs à départager ; bloc : groupes
	Spots   int       `json:"spots,omitempty"`   // barrage : places à attribuer
	Sub     *Label    `json:"sub,omitempty"`
}

// Empty indique un libellé absent.
func (l Label) Empty() bool { return l.Kind == LabelNone }

// With renvoie une copie de l portant sub comme sous-libellé (l inchangé si sub est vide).
func (l Label) With(sub Label) Label {
	if sub.Empty() {
		return l
	}
	c := sub
	l.Sub = &c
	return l
}

// labelPtr : copie adressable d'un libellé, nil s'il est vide.
func labelPtr(l Label) *Label {
	if l.Empty() {
		return nil
	}
	c := l
	return &c
}

// RoundLabel : le libellé du tour r (0 = premier) d'un tableau de `rounds` tours — finale,
// demi-finale, quart, puis « tour r+1 ».
func RoundLabel(r, rounds int) Label {
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

// NoteKind est le code d'une note de classement.
type NoteKind string

const (
	NoteNone          NoteKind = ""
	NoteWinner        NoteKind = "winner"
	NoteFinalist      NoteKind = "finalist"
	NoteAlive         NoteKind = "alive"          // Lives
	NoteRecord        NoteKind = "record"         // Wins, Losses
	NoteForfeit       NoteKind = "forfeit"        // match perdu par forfait (classement d'une section)
	NoteWithdrawn     NoteKind = "withdrawn"      // retiré, classé sur son parcours
	NoteRunning       NoteKind = "running"        // encore en course, tableau non terminé
	NoteAwaitingDraw  NoteKind = "awaiting_draw"  // tableau pas encore tiré
	NoteUnranked      NoteKind = "unranked"       // aucune sortie enregistrée
	NoteSectionExit   NoteKind = "section_exit"   // Section + Sub : sorti de cette section-là
	NoteSectionWinner NoteKind = "section_winner" // Section
	NotePoolRecord    NoteKind = "pool_record"    // Section, Wins, Qualified
)

// Note est la note de classement d'un joueur : un code et ses paramètres.
type Note struct {
	Kind      NoteKind `json:"kind,omitempty"`
	Wins      int      `json:"wins,omitempty"`
	Losses    int      `json:"losses,omitempty"`
	Lives     int      `json:"lives,omitempty"`
	Section   string   `json:"section,omitempty"`
	Qualified bool     `json:"qualified,omitempty"`
	Sub       *Label   `json:"sub,omitempty"`
}

// ---- Sections ----
//
// Un nom de section est un IDENTIFIANT, jamais affiché tel quel : il voyage dans le journal et
// dans la base de l'hôte. Le genre (Section.Kind) est la famille de graphe ; pour les sections
// d'un tableau, nom et genre coïncident.

const (
	SectionMain       = "main"  // tableau principal
	SectionConso      = "conso" // consolante progressive
	SectionLast       = "last"  // dernière chance
	SectionGrandFinal = "gf"    // grande finale de la double élimination

	SectionGSL     = "gsl"     // genre : groupe GSL de 4, 3 ou 2
	SectionSE      = "se"      // genre : mini-tableau à élimination simple (bloc GSL, une défaite)
	SectionPool    = "poule"   // genre : poule toutes rondes
	SectionBarrage = "barrage" // genre : barrage entre ex æquo d'une poule

	poolPrefix    = "poule:"
	barragePrefix = "barrage:"
)

// PoolSectionName : l'identifiant de la i-ième poule (0 → « poule:A »).
func PoolSectionName(i int) string { return poolPrefix + string(rune('A'+i)) }

// BarrageSectionName : l'identifiant du barrage de la poule donnée.
func BarrageSectionName(pool string) string {
	return barragePrefix + strings.TrimPrefix(pool, poolPrefix)
}

// PoolOfBarrage : la poule dont ce barrage départage les ex æquo.
func PoolOfBarrage(barrage string) string {
	return poolPrefix + strings.TrimPrefix(barrage, barragePrefix)
}
