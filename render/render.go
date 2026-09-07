package render

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
)

// Renderer produit les composants d'affichage. Tout ce qui est traduisible passe par L ; la
// feuille de style et le crédit sont injectés pour que l'hôte retrouve sa charte et sa mention.
type Renderer struct {
	L      Labeler
	Style  string // vide = DefaultStyle
	Credit string // vide = DefaultCredit
	Lang   string // code de langue de la page (attribut lang) ; vide = "fr"
}

// New : un rendu avec le Labeler donné, la feuille et le crédit par défaut.
func New(l Labeler) *Renderer { return &Renderer{L: l} }

func (r *Renderer) style() string {
	if r.Style != "" {
		return r.Style
	}
	return DefaultStyle
}

func (r *Renderer) credit() string {
	if r.Credit != "" {
		return r.Credit
	}
	return DefaultCredit
}

func (r *Renderer) lang() string {
	if r.Lang != "" {
		return r.Lang
	}
	return "fr"
}

func (r *Renderer) t(x Term, n int) string { return r.L.Term(x, n) }

func esc(s string) string { return html.EscapeString(s) }

// name : le nom d'un joueur. Un nom de joueur est saisi par le TD : il ne se traduit pas. Une
// place vide et une exemption, si.
func (r *Renderer) name(st *tournoi.State, id tournoi.PlayerID) string {
	switch id {
	case tournoi.BYE:
		return r.t(TermBye, 0)
	case "":
		return r.t(TermUnknown, 0)
	}
	if p := st.Players[id]; p != nil && p.Name != "" {
		return p.Name
	}
	return string(id)
}

// ---- Arbres ----

const (
	boxW, boxH   = 190.0, 44.0
	gapX, gapY   = 60.0, 12.0
	sectionGapX  = 40.0
	headerHeight = 30.0
)

// BracketBoardSVG dessine TOUTES les sections en graphe d'une phase dans un seul SVG, côte à
// côte : le principal, puis la consolante, puis la dernière chance, puis la grande finale.
//
// Chaque section occupe sa bande de colonnes, si bien que deux arbres ne peuvent pas se
// chevaucher — c'était le défaut du rendu par section, où chaque SVG partait de zéro et se
// superposait au précédent dans la page. Les DESCENTES (les perdants du principal qui
// alimentent la consolante) sont tracées d'une section à l'autre, en pointillé, sous les
// boîtes : ce sont elles qui font comprendre l'objet, et elles n'existaient pas.
func (r *Renderer) BracketBoardSVG(st *tournoi.State, ph *tournoi.PhaseState) string {
	var secs []*tournoi.Section
	for _, sec := range ph.Sections {
		if len(sec.Rounds) > 0 {
			secs = append(secs, sec)
		}
	}
	if len(secs) == 0 {
		return ""
	}
	type clé struct {
		sec string
		i   int
	}
	pos := map[clé][2]float64{}
	décalage := map[string]float64{}
	x0, hauteur := 10.0, 0.0
	for _, sec := range secs {
		décalage[sec.Name] = x0
		ys := placeSection(sec)
		for i, y := range ys {
			if y < 0 {
				continue
			}
			col := colonneDe(sec, i)
			pos[clé{sec.Name, i}] = [2]float64{x0 + float64(col)*(boxW+gapX), y}
			if y+boxH+20 > hauteur {
				hauteur = y + boxH + 20
			}
		}
		x0 += float64(len(sec.Rounds))*(boxW+gapX) + sectionGapX
	}
	largeur := x0 + 10
	var b strings.Builder
	// 1. les liaisons, sous les boîtes
	for _, sec := range secs {
		for i := range sec.Matches {
			ici, ok := pos[clé{sec.Name, i}]
			if !ok {
				continue
			}
			for _, src := range sec.Matches[i].Src {
				if src.From < 0 || src.Player != "" {
					continue
				}
				nom := sec.Name
				if src.Section != "" {
					nom = src.Section
				}
				amont, ok := pos[clé{nom, src.From}]
				if !ok {
					continue
				}
				trait, style := "#aaa", ""
				if nom != sec.Name {
					// une descente : elle traverse la page, on la distingue
					trait, style = "#c3a", ` stroke-dasharray="4 3"`
				}
				fmt.Fprintf(&b, `<path d="M%.0f %.0f H%.0f V%.0f H%.0f" fill="none" stroke="%s"%s/>`,
					amont[0]+boxW, amont[1]+boxH/2, ici[0]-gapX/2, ici[1]+boxH/2, ici[0], trait, style)
			}
		}
	}
	// 2. les boîtes
	for _, sec := range secs {
		x := décalage[sec.Name]
		fmt.Fprintf(&b, `<text x="%.0f" y="18" font-size="14" font-weight="bold">%s</text>`, x, esc(r.L.SectionName(sec.Name)))
		for i := range sec.Matches {
			p, ok := pos[clé{sec.Name, i}]
			if !ok {
				continue
			}
			r.boîte(&b, st, sec, i, p[0], p[1])
		}
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" font-family="system-ui, sans-serif" font-size="12">`,
		largeur, hauteur, largeur, hauteur) + b.String() + `</svg>`
}

// colonneDe : le tour auquel appartient le match i.
func colonneDe(sec *tournoi.Section, i int) int {
	for r, idx := range sec.Rounds {
		for _, j := range idx {
			if j == i {
				return r
			}
		}
	}
	return 0
}

// placeSection calcule l'ordonnée de chaque match d'une section (-1 = non dessiné : une paire
// de deux exemptions n'a rien à montrer).
func placeSection(sec *tournoi.Section) []float64 {
	ys := make([]float64, len(sec.Matches))
	for i := range ys {
		ys[i] = -1
	}
	rows := len(sec.Rounds[0])
	for r, idx := range sec.Rounds {
		colonne := make([]float64, len(idx))
		for k, i := range idx {
			if r == 0 {
				colonne[k] = headerHeight + float64(k)*(boxH+gapY)
				continue
			}
			var amont []float64
			for _, src := range sec.Matches[i].Src {
				if src.From >= 0 && src.Section == "" && ys[src.From] >= 0 {
					amont = append(amont, ys[src.From])
				}
			}
			if len(amont) == 0 {
				colonne[k] = headerHeight + float64(k)*(boxH+gapY)*float64(rows)/float64(len(idx))
				continue
			}
			somme := 0.0
			for _, v := range amont {
				somme += v
			}
			colonne[k] = somme / float64(len(amont))
		}
		// écarte les boîtes qui se chevaucheraient (matchs des gagnants et des perdants d'un
		// groupe GSL, par exemple)
		for k := 1; k < len(colonne); k++ {
			if colonne[k] < colonne[k-1]+boxH+gapY {
				colonne[k] = colonne[k-1] + boxH + gapY
			}
		}
		for k, i := range idx {
			g := sec.Matches[i]
			if g.Players[0] == tournoi.BYE && g.Players[1] == tournoi.BYE {
				continue
			}
			ys[i] = colonne[k]
		}
	}
	return ys
}

func (r *Renderer) boîte(b *strings.Builder, st *tournoi.State, sec *tournoi.Section, i int, x, y float64) {
	g := sec.Matches[i]
	fond := "#fff"
	if g.Done {
		fond = "#eef5ee"
	} else if g.MatchID != "" {
		fond = "#fff7dd"
	}
	fmt.Fprintf(b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="4" fill="%s" stroke="#888"/>`, x, y, boxW, boxH, fond)
	for c := 0; c < 2; c++ {
		p := g.Players[c]
		graisse := "normal"
		if g.Done && p == g.Winner && p != tournoi.BYE {
			graisse = "bold"
		}
		score := ""
		if m := st.Matches[g.MatchID]; g.MatchID != "" && m != nil && m.Status == tournoi.Finished {
			sc := m.ScoreA
			if p == m.B {
				sc = m.ScoreB
			}
			score = fmt.Sprintf("%d", sc)
		}
		fmt.Fprintf(b, `<text x="%.0f" y="%.0f" font-weight="%s">%s</text><text x="%.0f" y="%.0f" text-anchor="end">%s</text>`,
			x+6, y+17+float64(c)*20, graisse, esc(r.name(st, p)), x+boxW-6, y+17+float64(c)*20, score)
	}
	fmt.Fprintf(b, `<text x="%.0f" y="%.0f" font-size="9" fill="#666">%s · %d %s</text>`,
		x+6, y-2, esc(r.L.Label(g.Label)), g.Length, esc(r.t(TermPoints, g.Length)))
}

// ---- Vies ----

// LivesBoard : les joueurs d'une phase à vies par nombre de défaites, avec les adversaires déjà
// rencontrés et le temps d'attente.
//
// Les deux colonnes ajoutées sont celles que le TD lisait sur son cahier : qui a déjà joué qui
// (pour comprendre un appariement, ou refuser un rematch), et depuis combien de temps un joueur
// attend (pour savoir qui envoyer à table d'abord).
func (r *Renderer) LivesBoard(st *tournoi.State, ph *tournoi.PhaseState, now time.Time) string {
	L := ph.Cfg.Lives
	if L == 0 {
		L = 2
	}
	occupé := map[tournoi.PlayerID]*tournoi.Match{}
	for _, m := range st.Running() {
		occupé[m.A], occupé[m.B] = m, m
	}
	// fin du dernier match de chaque joueur dans la phase : l'attente part de là
	depuis := map[tournoi.PlayerID]time.Time{}
	for _, id := range st.MatchOrder {
		m := st.Matches[id]
		if m.Phase != ph.Index || m.Status != tournoi.Finished || m.End.IsZero() {
			continue
		}
		for _, p := range []tournoi.PlayerID{m.A, m.B} {
			if t, ok := depuis[p]; !ok || m.End.After(t) {
				depuis[p] = m.End
			}
		}
	}
	var b strings.Builder
	b.WriteString(`<div class="vies">`)
	for l := 0; l <= L; l++ {
		var ids []tournoi.PlayerID
		for _, p := range ph.Entrants {
			if ph.Losses[p] == l || (l == L && ph.Losses[p] >= L) {
				ids = append(ids, p)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			if ph.Wins[ids[i]] != ph.Wins[ids[j]] {
				return ph.Wins[ids[i]] > ph.Wins[ids[j]]
			}
			return ids[i] < ids[j]
		})
		titre := fmt.Sprintf("%s — %s", r.t(TermLosses, l), r.t(TermPlayers, len(ids)))
		if l == L {
			titre = fmt.Sprintf("%s — %d", r.t(TermEliminated, 0), len(ids))
		}
		fmt.Fprintf(&b, `<div><h3>%s</h3><table><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>`,
			esc(titre), esc(r.t(TermPlayer, 0)), esc(r.t(TermWins, 0)), esc(r.t(TermState, 0)),
			esc(r.t(TermWaiting, 0)), esc(r.t(TermOpponents, 0)))
		for _, p := range ids {
			état, attente := r.t(TermFree, 0), ""
			if m, ok := occupé[p]; ok {
				état = fmt.Sprintf("%s %d %s %s", r.t(TermTable, 0), m.Table, r.t(TermVersus, 0), r.name(st, autre(m, p)))
			} else if t, ok := depuis[p]; ok && now.After(t) {
				attente = durée(now.Sub(t))
			}
			if st.Withdrawn[p] {
				état, attente = r.t(TermForfeit, 0), ""
			}
			if l == L {
				état, attente = "", ""
			}
			var advs []string
			for _, o := range ph.Opponents[p] {
				advs = append(advs, r.name(st, o))
			}
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				esc(r.name(st, p)), ph.Wins[p], esc(état), esc(attente), esc(strings.Join(advs, ", ")))
		}
		b.WriteString(`</table></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func autre(m *tournoi.Match, p tournoi.PlayerID) tournoi.PlayerID {
	if m.A == p {
		return m.B
	}
	return m.A
}

// durée : une durée en h/min, sans mot — les chiffres et « h » se lisent dans toutes les langues
// que sert ce paquet.
func durée(d time.Duration) string {
	d = d.Round(time.Minute)
	if h := int(d.Hours()); h > 0 {
		return fmt.Sprintf("%dh%02d", h, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d'", int(d.Minutes()))
}

// ---- Tables ----

// TableGrid : une case par table — son match, sa durée, ou libre, indisponible, réservée.
//
// C'est la vue que le TD cherche des yeux en levant la tête : où est-ce que ça joue, où
// peut-on envoyer les deux qui attendent.
func (r *Renderer) TableGrid(st *tournoi.State, now time.Time) string {
	n := st.Config.Tables.Count
	parTable := map[int]*tournoi.Match{}
	for _, m := range st.Running() {
		if m.Table > 0 {
			parTable[m.Table] = m
			if m.Table > n && st.Config.Tables.Count == 0 {
				n = m.Table
			}
		}
	}
	for _, u := range st.Config.Tables.Unavailable {
		if u > n && st.Config.Tables.Count == 0 {
			n = u
		}
	}
	if n == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<ul class="tables">`)
	for t := 1; t <= n; t++ {
		classe, contenu := "libre", esc(r.t(TermFree, 0))
		switch {
		case indisponible(st, t):
			classe, contenu = "hs", esc(r.t(TermUnavailable, 0))
		case parTable[t] != nil:
			m := parTable[t]
			classe = "occupee"
			contenu = fmt.Sprintf(`%s %s %s<br><span class="menu">%s · %s</span>`,
				esc(r.name(st, m.A)), esc(r.t(TermVersus, 0)), esc(r.name(st, m.B)),
				esc(r.L.Label(m.Label)), esc(durée(now.Sub(m.Start))))
		default:
			if s := réservéePour(st, t); s != "" {
				contenu = fmt.Sprintf(`%s · %s`, esc(r.t(TermReserved, 0)), esc(r.L.SectionName(s)))
			}
		}
		fmt.Fprintf(&b, `<li class="%s"><span class="num">%s %d</span>%s</li>`, classe, esc(r.t(TermTable, 0)), t, contenu)
	}
	b.WriteString(`</ul>`)
	return b.String()
}

func indisponible(st *tournoi.State, t int) bool {
	for _, u := range st.Config.Tables.Unavailable {
		if u == t {
			return true
		}
	}
	return false
}

func réservéePour(st *tournoi.State, t int) string {
	for _, rule := range st.Config.Tables.Reserved {
		if rule.Table == t && rule.Section != "" {
			return rule.Section
		}
	}
	return ""
}

// ---- Matchs en cours et actions ----

// RunningTable : les matchs en cours (table, libellé, joueurs, longueur, durée).
func (r *Renderer) RunningTable(st *tournoi.State, now time.Time) string {
	ms := st.Running()
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].Table != ms[j].Table {
			return ms[i].Table < ms[j].Table
		}
		return ms[i].ID < ms[j].ID
	})
	var b strings.Builder
	fmt.Fprintf(&b, `<table class="running"><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>`,
		esc(r.t(TermTable, 0)), esc(r.t(TermMatch, 0)), esc(r.t(TermPlayer, 0)),
		esc(r.t(TermPoints, 0)), esc(r.t(TermSince, 0)))
	for _, m := range ms {
		classe := ""
		if now.Sub(m.Start) > time.Duration(1.5*float64(st.Expected(m.Length))) {
			classe = ` class="lent"`
		}
		fmt.Fprintf(&b, `<tr%s><td>%d</td><td>%s</td><td>%s %s %s</td><td>%d</td><td>%s</td></tr>`,
			classe, m.Table, esc(r.L.Label(m.Label)), esc(r.name(st, m.A)), esc(r.t(TermVersus, 0)),
			esc(r.name(st, m.B)), m.Length, esc(durée(now.Sub(m.Start))))
	}
	b.WriteString(`</table>`)
	return b.String()
}

// ActionsList : les actions proposées au TD, dans sa langue.
func (r *Renderer) ActionsList(st *tournoi.State, acts []tournoi.Action) string {
	var b strings.Builder
	b.WriteString(`<ol class="actions">`)
	for _, a := range acts {
		var s string
		switch a.Kind {
		case tournoi.ActStartMatch:
			s = fmt.Sprintf("%s %d — %s : %s %s %s, %d %s", r.t(TermTable, 0), a.Table, r.L.Label(a.Label),
				r.name(st, a.A), r.t(TermVersus, 0), r.name(st, a.B), a.Length, r.t(TermPoints, a.Length))
		case tournoi.ActCancelMatch:
			s = fmt.Sprintf("%s %s — %s : %s %s %s", r.t(TermCancel, 0), a.Match, r.L.Label(a.Label),
				r.name(st, a.A), r.t(TermVersus, 0), r.name(st, a.B))
		case tournoi.ActBye:
			s = fmt.Sprintf("%s — %s", r.t(TermBye, 0), r.name(st, a.A))
		case tournoi.ActDraw, tournoi.ActNextPhase:
			s = r.L.Label(a.Label)
		case tournoi.ActWait:
			s = r.L.Reason(a.Reason)
			if !a.Until.IsZero() {
				s += " (" + a.Until.Format("15:04") + ")"
			}
		default:
			s = string(a.Kind)
		}
		classe := ""
		if a.Warn != "" {
			classe = ` class="alerte"`
			s += " — " + r.L.Warn(a.Warn)
		}
		fmt.Fprintf(&b, `<li%s>%s</li>`, classe, esc(s))
	}
	b.WriteString(`</ol>`)
	return b.String()
}

// ---- Feuille d'appariements ----

// Batches : les lots de matchs de la phase courante, dans l'ordre de leur départ.
//
// Un « lot » est un paquet de matchs lancés au même instant. C'est ce qu'un directeur appelle
// une ronde dans un suisse par rondes, un bloc dans un GSL, un tour dans un tableau, et une
// micro-ronde dans un suisse continu à batch_minutes. Le moteur n'a pas de mot pour ça, et il
// n'en a pas besoin : l'instant de départ le dit.
func Batches(st *tournoi.State) [][]*tournoi.Match {
	if st.Current < 0 {
		return nil
	}
	parInstant := map[time.Time][]*tournoi.Match{}
	var instants []time.Time
	for _, id := range st.MatchOrder {
		m := st.Matches[id]
		if m.Phase != st.Current || m.Status == tournoi.Cancelled {
			continue
		}
		if _, vu := parInstant[m.Start]; !vu {
			instants = append(instants, m.Start)
		}
		parInstant[m.Start] = append(parInstant[m.Start], m)
	}
	sort.Slice(instants, func(i, j int) bool { return instants[i].Before(instants[j]) })
	out := make([][]*tournoi.Match, 0, len(instants))
	for _, t := range instants {
		out = append(out, parInstant[t])
	}
	return out
}

// PairingSheet : la feuille d'appariements d'un lot, à imprimer et à poser sur la table du TD.
// Une ligne par match, une case vide pour le score écrit à la main.
//
// round est le numéro du lot (1 = le premier de la phase) ; 0 ou au-delà du dernier donne le lot
// le plus récent, celui qu'on imprime en pratique.
func (r *Renderer) PairingSheet(st *tournoi.State, round int) string {
	lots := Batches(st)
	if len(lots) == 0 {
		return fmt.Sprintf(`<p class="resume">%s</p>`, esc(r.t(TermNoMatch, 0)))
	}
	if round < 1 || round > len(lots) {
		round = len(lots)
	}
	lot := lots[round-1]
	sort.Slice(lot, func(i, j int) bool {
		if lot[i].Table != lot[j].Table {
			return lot[i].Table < lot[j].Table
		}
		return lot[i].ID < lot[j].ID
	})
	var b strings.Builder
	fmt.Fprintf(&b, `<h2>%s — %s %d</h2>`, esc(r.t(TermPairings, 0)), esc(r.t(TermRound, 0)), round)
	fmt.Fprintf(&b, `<table class="appariements"><tr><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th><th>%s</th></tr>`,
		esc(r.t(TermTable, 0)), esc(r.t(TermMatch, 0)), esc(r.t(TermPlayer, 0)), esc(r.t(TermScore, 0)),
		esc(r.t(TermPlayer, 0)), esc(r.t(TermScore, 0)))
	for _, m := range lot {
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s · %d %s</td><td>%s</td><td class="case"></td><td>%s</td><td class="case"></td></tr>`,
			m.Table, esc(r.L.Label(m.Label)), m.Length, esc(r.t(TermPoints, m.Length)),
			esc(r.name(st, m.A)), esc(r.name(st, m.B)))
	}
	b.WriteString(`</table>`)
	return r.document(st.Config.Name, b.String())
}

// ---- Classement ----

// StandingsTable : le classement courant ou final, avec les prix.
func (r *Renderer) StandingsTable(st *tournoi.State) string {
	ranking := st.Final
	if ranking == nil {
		ranking = st.Ranking()
	}
	prix := st.SectionPrizes(tournoi.PrizeSectionAll)
	var b strings.Builder
	fmt.Fprintf(&b, `<table class="standings"><tr><th>%s</th><th>%s</th><th>%s</th><th></th><th>%s</th></tr>`,
		esc(r.t(TermRank, 0)), esc(r.t(TermPlayer, 0)), esc(r.t(TermClub, 0)), esc(r.t(TermPrize, 0)))
	for _, rk := range ranking {
		club := ""
		if p := st.Players[rk.Player]; p != nil {
			club = p.Club
		}
		montant := ""
		if v := prix[rk.Player]; v > 0 {
			montant = fmt.Sprintf("%.2f", v)
		}
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			rk.Rank, esc(r.name(st, rk.Player)), esc(club), esc(r.L.Note(rk.Note)), montant)
	}
	b.WriteString(`</table>`)
	return b.String()
}

// ---- Pages ----

// document enveloppe un contenu dans une page AUTONOME : un seul fichier, CSS embarqué, aucune
// ressource externe. Elle doit s'ouvrir hors ligne, depuis une clé USB, sur l'ordinateur de la
// salle — c'est la seule chose sur laquelle on puisse compter un dimanche matin.
func (r *Renderer) document(titre, corps string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><html lang="%s"><head><meta charset="utf-8">`+
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>%s</style></head><body>`,
		esc(r.lang()), esc(titre), r.style())
	b.WriteString(corps)
	fmt.Fprintf(&b, `<p class="credit">%s</p></body></html>`, esc(r.credit()))
	return b.String()
}

// Page : la page d'affichage de la salle, autonome et rafraîchie toutes les 30 s.
func (r *Renderer) Page(st *tournoi.State, acts []tournoi.Action, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<h1>%s</h1><p class="resume">%s — %s, %s, %s</p>`,
		esc(st.Config.Name), now.Format("15:04"),
		esc(r.t(TermPlayers, len(st.Order))),
		esc(r.t(TermPlayed, len(st.MatchOrder)-len(st.Running()))),
		esc(fmt.Sprintf("%d %s", len(st.Running()), r.t(TermInProgress, 0))))
	if st.Current >= 0 {
		ph := st.Phases[st.Current]
		fmt.Fprintf(&b, `<h2>%s %d : %s</h2>`, esc(r.t(TermPhase, 0)), ph.Index+1, esc(r.L.PhaseName(ph.Cfg)))
		if len(acts) > 0 {
			fmt.Fprintf(&b, `<h3>%s</h3>%s`, esc(r.t(TermTodo, 0)), r.ActionsList(st, acts))
		}
		if g := r.TableGrid(st, now); g != "" {
			fmt.Fprintf(&b, `<h3>%s</h3>%s`, esc(r.t(TermTables, 0)), g)
		}
		fmt.Fprintf(&b, `<h3>%s</h3>%s`, esc(r.t(TermRunning, 0)), r.RunningTable(st, now))
		if ph.Cfg.Kind == tournoi.KindSwissLives || ph.Cfg.Kind == tournoi.KindGSL {
			fmt.Fprintf(&b, `<h3>%s</h3>%s`, esc(r.t(TermLives, 0)), r.LivesBoard(st, ph, now))
		}
		if svg := r.BracketBoardSVG(st, ph); svg != "" {
			fmt.Fprintf(&b, `<div class="arbre">%s</div>`, svg)
		}
	}
	fmt.Fprintf(&b, `<h2>%s</h2>%s`, esc(r.t(TermStandings, 0)), r.StandingsTable(st))
	page := r.document(st.Config.Name, b.String())
	// Le rafraîchissement est une méta, pas un script : la page reste sans JavaScript.
	return strings.Replace(page, `<meta charset="utf-8">`, `<meta charset="utf-8"><meta http-equiv="refresh" content="30">`, 1)
}
