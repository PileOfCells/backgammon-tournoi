// Package render produit des composants d'affichage (SVG, HTML) à partir d'un état de tournoi :
// arbres de tableaux, tableau des vies, appariements en cours, classement. Fonctions pures,
// sans JavaScript ; l'hôte les insère dans ses pages.
package render

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/PileOfCells/backgammon-tournoi"
)

func name(st *tournoi.State, id tournoi.PlayerID) string {
	if id == tournoi.BYE {
		return "exempt"
	}
	if id == "" {
		return "…"
	}
	if p := st.Players[id]; p != nil && p.Name != "" {
		return p.Name
	}
	return string(id)
}

func esc(s string) string { return html.EscapeString(s) }

// BracketSVG dessine une section en graphe (tableau, groupe GSL, consolante) en SVG.
func BracketSVG(st *tournoi.State, sec *tournoi.Section) string {
	if len(sec.Rounds) == 0 {
		return ""
	}
	const w, h, gapX, padY = 190.0, 44.0, 60.0, 12.0
	rows := len(sec.Rounds[0])
	height := float64(rows)*(h+padY) + 40
	width := float64(len(sec.Rounds))*(w+gapX) + 20
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" font-family="system-ui, sans-serif" font-size="12">`, width, height, width, height)
	fmt.Fprintf(&b, `<text x="10" y="18" font-size="14" font-weight="bold">%s</text>`, esc(sec.Name))
	pos := map[int][2]float64{}
	for r, idx := range sec.Rounds {
		x := 10 + float64(r)*(w+gapX)
		// ordonnée souhaitée : au milieu des matchs sources, sinon répartition régulière
		ys := make([]float64, len(idx))
		for k, i := range idx {
			if r == 0 {
				ys[k] = 30 + float64(k)*(h+padY)
				continue
			}
			g := sec.Matches[i]
			var src []float64
			for _, sr := range g.Src {
				if sr.From >= 0 && sr.Section == "" {
					if p, ok := pos[sr.From]; ok {
						src = append(src, p[1])
					}
				}
			}
			if len(src) == 0 {
				ys[k] = 30 + float64(k)*(h+padY)*float64(rows)/float64(len(idx))
			} else {
				for _, v := range src {
					ys[k] += v
				}
				ys[k] /= float64(len(src))
			}
		}
		// évite les chevauchements (matchs des gagnants et des perdants d'un groupe GSL, par exemple)
		for k := 1; k < len(ys); k++ {
			if ys[k] < ys[k-1]+h+padY {
				ys[k] = ys[k-1] + h + padY
			}
		}
		for k, i := range idx {
			y := ys[k]
			pos[i] = [2]float64{x, y}
			g := sec.Matches[i]
			if g.Players[0] == tournoi.BYE && g.Players[1] == tournoi.BYE {
				continue // paire d'exemptions : rien à dessiner
			}
			fill := "#fff"
			if g.Done {
				fill = "#eef5ee"
			} else if g.MatchID != "" {
				fill = "#fff7dd"
			}
			fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="4" fill="%s" stroke="#888"/>`, x, y, w, h, fill)
			for s2 := 0; s2 < 2; s2++ {
				p := g.Players[s2]
				label := name(st, p)
				if p == "" && g.Src[s2].From >= 0 {
					label = "…"
				}
				weight := "normal"
				if g.Done && p == g.Winner && p != tournoi.BYE {
					weight = "bold"
				}
				score := ""
				if g.MatchID != "" {
					if m := st.Matches[g.MatchID]; m != nil && m.Status == tournoi.Finished {
						sc := m.ScoreA
						if p == m.B {
							sc = m.ScoreB
						}
						score = fmt.Sprintf("%d", sc)
					}
				}
				fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" font-weight="%s">%s</text><text x="%.0f" y="%.0f" text-anchor="end">%s</text>`,
					x+6, y+17+float64(s2)*20, weight, esc(label), x+w-6, y+17+float64(s2)*20, score)
			}
			fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" font-size="9" fill="#666">%s · %d pts</text>`, x+6, y-2, esc(g.Label), g.Length)
			// liaisons
			for _, src := range g.Src {
				if src.From >= 0 && src.Section == "" {
					if p, ok := pos[src.From]; ok {
						fmt.Fprintf(&b, `<path d="M%.0f %.0f H%.0f V%.0f H%.0f" fill="none" stroke="#aaa"/>`, p[0]+w, p[1]+h/2, x-gapX/2, y+h/2, x)
					}
				}
			}
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// LivesBoard : tableau des joueurs d'une phase à vies par nombre de défaites (état, adversaires).
func LivesBoard(st *tournoi.State, ph *tournoi.PhaseState) string {
	L := ph.Cfg.Lives
	if L == 0 {
		L = 2
	}
	busy := map[tournoi.PlayerID]*tournoi.Match{}
	for _, m := range st.Running() {
		busy[m.A], busy[m.B] = m, m
	}
	var b strings.Builder
	b.WriteString(`<div class="lives-board" style="display:flex;gap:16px;align-items:flex-start">`)
	for l := 0; l <= L; l++ {
		var ids []tournoi.PlayerID
		for _, p := range ph.Entrants {
			if ph.Losses[p] == l || (l == L && ph.Losses[p] >= L) {
				ids = append(ids, p)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ph.Wins[ids[i]] > ph.Wins[ids[j]] })
		title := fmt.Sprintf("%d défaite(s) — %d joueurs", l, len(ids))
		if l == L {
			title = fmt.Sprintf("Éliminés — %d", len(ids))
		}
		fmt.Fprintf(&b, `<div style="flex:1"><h3>%s</h3><table><tr><th>Joueur</th><th>V</th><th>État</th></tr>`, esc(title))
		for _, p := range ids {
			state := "libre"
			if m, ok := busy[p]; ok {
				state = fmt.Sprintf("table %d contre %s", m.Table, esc(name(st, other(m, p))))
			}
			if st.Withdrawn[p] {
				state = "forfait"
			}
			if l == L {
				state = ""
			}
			fmt.Fprintf(&b, `<tr><td>%s</td><td>%d</td><td>%s</td></tr>`, esc(name(st, p)), ph.Wins[p], state)
		}
		b.WriteString(`</table></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func other(m *tournoi.Match, p tournoi.PlayerID) tournoi.PlayerID {
	if m.A == p {
		return m.B
	}
	return m.A
}

// RunningTable : matchs en cours (table, joueurs, longueur, début, durée).
func RunningTable(st *tournoi.State, now time.Time) string {
	ms := st.Running()
	sort.Slice(ms, func(i, j int) bool { return ms[i].Table < ms[j].Table })
	var b strings.Builder
	b.WriteString(`<table class="running"><tr><th>Table</th><th>Match</th><th>Joueurs</th><th>Pts</th><th>Depuis</th></tr>`)
	for _, m := range ms {
		d := now.Sub(m.Start).Round(time.Minute)
		slow := ""
		if d > time.Duration(1.5*float64(st.Expected(m.Length))) {
			slow = ` style="color:#b00"`
		}
		fmt.Fprintf(&b, `<tr%s><td>%d</td><td>%s</td><td>%s — %s</td><td>%d</td><td>%s</td></tr>`, slow, m.Table, esc(m.Label), esc(name(st, m.A)), esc(name(st, m.B)), m.Length, d)
	}
	b.WriteString(`</table>`)
	return b.String()
}

// ActionsList : actions proposées au TD, en clair.
func ActionsList(st *tournoi.State, acts []tournoi.Action) string {
	var b strings.Builder
	b.WriteString(`<ol class="actions">`)
	for _, a := range acts {
		s := a.String()
		if a.Kind == tournoi.ActStartMatch {
			s = fmt.Sprintf("Table %d — %s : %s contre %s, %d points", a.Table, a.Label, name(st, a.A), name(st, a.B), a.Length)
		}
		fmt.Fprintf(&b, `<li>%s</li>`, esc(s))
	}
	b.WriteString(`</ol>`)
	return b.String()
}

// StandingsTable : classement courant ou final, avec prix.
func StandingsTable(st *tournoi.State) string {
	ranking := st.Final
	if ranking == nil {
		ranking = st.Ranking()
	}
	prizes := tournoi.Prizes(ranking, st.Config.Prizes)
	var b strings.Builder
	b.WriteString(`<table class="standings"><tr><th>#</th><th>Joueur</th><th>Club</th><th></th><th>Prix</th></tr>`)
	for _, r := range ranking {
		club := ""
		if p := st.Players[r.Player]; p != nil {
			club = p.Club
		}
		prize := ""
		if v := prizes[r.Player]; v > 0 {
			prize = fmt.Sprintf("%.2f", v)
		}
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`, r.Rank, esc(name(st, r.Player)), esc(club), esc(r.Note), prize)
	}
	b.WriteString(`</table>`)
	return b.String()
}

// Page : page d'affichage complète (écran de salle), auto-rafraîchie toutes les 30 s.
func Page(st *tournoi.State, acts []tournoi.Action, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><html lang="fr"><head><meta charset="utf-8"><meta http-equiv="refresh" content="30"><title>%s</title>
<style>body{font-family:system-ui,sans-serif;margin:16px;color:#222}table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:3px 8px;text-align:left}h2{margin-top:28px}</style></head><body>`, esc(st.Config.Name))
	fmt.Fprintf(&b, `<h1>%s</h1><p>%s — %d joueurs, %d matchs joués, %d en cours</p>`, esc(st.Config.Name), now.Format("15:04"), len(st.Order), len(st.MatchOrder)-len(st.Running()), len(st.Running()))
	if ph := st.Phases[st.Current]; ph != nil {
		fmt.Fprintf(&b, `<h2>Phase %d : %s</h2>`, ph.Index+1, esc(ph.Cfg.Name))
		if len(acts) > 0 {
			b.WriteString(`<h3>À faire</h3>` + ActionsList(st, acts))
		}
		b.WriteString(`<h3>Matchs en cours</h3>` + RunningTable(st, now))
		if ph.Cfg.Kind == tournoi.KindSwissLives || ph.Cfg.Kind == tournoi.KindGSL {
			b.WriteString(`<h3>Vies</h3>` + LivesBoard(st, ph))
		}
		for _, sec := range ph.Sections {
			if len(sec.Rounds) > 0 {
				b.WriteString(`<div style="overflow-x:auto">` + BracketSVG(st, sec) + `</div>`)
			}
		}
	}
	b.WriteString(`<h2>Classement</h2>` + StandingsTable(st))
	b.WriteString(`</body></html>`)
	return b.String()
}
