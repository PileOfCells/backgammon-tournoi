package tournoi

import "fmt"

// Rendu français des codes (codes.go). C'est le seul endroit du moteur où une phrase destinée
// à un humain est écrite. Il sert la console tournoi-td, la démo et le paquet render ; un
// logiciel hôte multilingue ignore ce fichier et traduit les codes lui-même.

// String rend un libellé en français.
func (l Label) String() string {
	sub := ""
	if l.Sub != nil {
		sub = l.Sub.String()
	}
	switch l.Kind {
	case LabelNone:
		return ""
	case LabelSwissGroup:
		return fmt.Sprintf("%d défaite(s), match %d", l.Losses, l.Match)
	case LabelRound:
		return fmt.Sprintf("Ronde %d", l.N)
	case LabelRematch:
		return "Rematch"
	case LabelCrossed:
		return "Match croisé"
	case LabelBracketRound:
		return fmt.Sprintf("Tour %d", l.N)
	case LabelFinal:
		return "Finale"
	case LabelSemiFinal:
		return "Demi-finale"
	case LabelQuarterFinal:
		return "Quart de finale"
	case LabelMainDraw:
		return "Principal, " + sub
	case LabelConsolationRound:
		return fmt.Sprintf("Consolante tour %d", l.N)
	case LabelConsolationFinal:
		return "Finale consolante"
	case LabelLastChance:
		if sub == "" {
			return "Dernière chance, tour 1"
		}
		return "Dernière chance, " + sub
	case LabelGrandFinal:
		return "Grande finale"
	case LabelGrandFinalRecharge:
		return "Grande finale (recharge)"
	case LabelOpening:
		return "Ouverture"
	case LabelWinnersMatch:
		return "Match des gagnants"
	case LabelLosersMatch:
		return "Match des perdants"
	case LabelDecider:
		return "Match décisif"
	case LabelSingleMatch:
		return "Match"
	case LabelSingleElim:
		return "Élimination directe"
	case LabelInBlock:
		return fmt.Sprintf("Bloc %d, %s : %s", l.N, sectionName(l.Section), sub)
	case LabelPoolRound:
		return fmt.Sprintf("Poule, ronde %d", l.N)
	case LabelInSection:
		return fmt.Sprintf("%s, %s", sectionName(l.Section), sub)
	case LabelBarrage:
		return "Barrage " + sectionName(l.Section)
	case LabelBarrageCross:
		return "Barrage " + sectionName(l.Section) + " (croisé)"
	case LabelDrawPools:
		return fmt.Sprintf("%d poules", l.N)
	case LabelDrawBarrage:
		return fmt.Sprintf("Barrage %s : %d joueurs pour %d place(s)", sectionName(l.Section), l.Players, l.Spots)
	case LabelDrawBracket:
		if sub != "" {
			return fmt.Sprintf("%s : tableau de %d places", sub, l.N)
		}
		return fmt.Sprintf("Tableau de %d places", l.N)
	case LabelDrawBlock:
		return fmt.Sprintf("Bloc %d : %d groupes", l.N, l.Players)
	case LabelPhase:
		return l.Text
	case LabelPoolName:
		return "poule " + l.Text
	case LabelBarrageName:
		return "Barrage " + sub
	case LabelSectionMain:
		return "Principal"
	case LabelSectionConso:
		return "Consolante"
	case LabelSectionLast:
		return "Dernière chance"
	case LabelSectionFinals:
		return "Grande finale"
	}
	return string(l.Kind)
}

// sectionName rend le nom interne d'une section en français. Les noms de tableau sont des
// identifiants ASCII ; les poules portent leur lettre.
func sectionName(name string) string {
	switch name {
	case "":
		return ""
	case secMain:
		return "principal"
	case secConso:
		return "consolante"
	case secLast:
		return "dernière chance"
	case secGrandFinal:
		return "grande finale"
	}
	if letter, ok := poolLetter(name); ok {
		return "poule " + letter
	}
	if letter, ok := barragePoolLetter(name); ok {
		return "barrage poule " + letter
	}
	return name
}

// String rend une note de classement en français.
func (n Note) String() string {
	sub := ""
	if n.Sub != nil {
		sub = n.Sub.String()
	}
	switch n.Kind {
	case NoteNone:
		return ""
	case NoteWinner:
		return "vainqueur"
	case NoteFinalist:
		return "finaliste"
	case NoteAlive:
		return fmt.Sprintf("en vie (%d vies)", n.Lives)
	case NoteRecord:
		return fmt.Sprintf("%d victoires, %d défaites", n.Wins, n.Losses)
	case NoteForfeit:
		return "forfait"
	case NoteRunning:
		return "en cours"
	case NoteAwaitingDraw:
		return "en attente du tirage"
	case NoteUnranked:
		return "non classé"
	case NoteSectionExit:
		return fmt.Sprintf("%s, %s", sectionName(n.Section), sub)
	case NoteSectionWinner:
		return "vainqueur " + sectionName(n.Section)
	case NotePoolRecord:
		s := fmt.Sprintf("%s, %d victoires", sectionName(n.Section), n.Wins)
		if n.Qualified {
			s += ", qualifié"
		}
		return s
	}
	return string(n.Kind)
}

// String rend une raison d'attente en français.
func (r ReasonCode) String() string {
	switch r {
	case ReasonNone:
		return ""
	case ReasonMatchesRunning:
		return "matchs en cours"
	case ReasonNoPairing:
		return "aucun appariement possible"
	case ReasonWaitingBatch:
		return "appariement au prochain lot"
	case ReasonWaitingTable:
		return "aucune table libre"
	}
	return string(r)
}

// String rend un avertissement en français.
func (w Warning) String() string {
	switch w.Code {
	case WarnBracketWrongPlayers:
		return fmt.Sprintf("match %s (%s %s) : joueurs %s/%s mais le tableau attend %s/%s",
			w.Match, sectionName(w.Section), w.Label.String(), w.A, w.B, w.ExpectedA, w.ExpectedB)
	case WarnScoreOverLength:
		return fmt.Sprintf("match %s : score %d-%d au-delà de la longueur %d", w.Match, w.ScoreA, w.ScoreB, w.Length)
	case WarnEndsInBreak:
		return fmt.Sprintf("match %s : la fin attendue tombe pendant une pause", w.Match)
	case WarnSlowMatch:
		return fmt.Sprintf("match %s : durée au-delà de l'attendu", w.Match)
	}
	return string(w.Code)
}

// PhaseName rend le nom d'une phase : celui que le TD a saisi, sinon le défaut du format.
// Validate ne remplit plus Name, pour qu'un hôte multilingue puisse fabriquer le sien.
func PhaseName(p PhaseConfig) string {
	if p.Name != "" {
		return p.Name
	}
	switch p.Kind {
	case KindSwissLives:
		return fmt.Sprintf("Suisse %d vies", p.Lives)
	case KindLivesBracket:
		return "Tableau final"
	case KindGSL:
		return "Blocs GSL"
	case KindBracket:
		if p.Reconciliation {
			return "Double élimination"
		}
		return "Tableau"
	case KindRoundRobin:
		return "Poules"
	}
	return p.Kind
}
