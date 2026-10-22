package rules

import "sort"

// ---- Matchs restants : la pièce de la prévision de fin qui se compte en appariant ----

// Remaining encadre le nombre de matchs qu'une phase doit encore TERMINER, matchs en cours
// compris : Min ≤ restants ≤ Max. L'encadrement suppose qu'aucun joueur ne se retire plus et
// qu'aucun résultat n'est corrigé.
type Remaining struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// RemainingMatches : les matchs que la phase doit encore terminer.
//
//   - Suisse et GSL : chaque match coûte exactement une vie. La phase s'arrête quand Σ vies ≤
//     Target ou quand il ne reste qu'un joueur en vie, qui garde au plus les vies du mieux loti
//     (au plus tôt après Σ vies − ce maximum). Le suisse s'arrête pile à la bascule (au plus
//     Σ vies − Target matchs) ; un GSL finit son bloc, et peut la dépasser (au plus Σ vies − 1).
//   - Tableau : les nœuds du graphe qui seront joués — une place vide (exemption) ou un retiré
//     termine un nœud sans match. Exact, sauf la recharge de la double élimination, qui ne
//     compte que dans Max. Un tableau pas encore tiré est compté sur un tirage de graine 0 :
//     le nombre ne dépend que des retirés placés.
//   - Poules : les nœuds des poules, plus les barrages — un barrage existant entre ses bornes de
//     vies (les ex æquo éliminés perdent toutes les leurs, les qualifiés au moins une), une
//     égalité connue sans barrage entre 2(k − places) et 2k − places, et une poule pas finie
//     jusqu'à 2n − 1 dans Max seulement.
//
// Les phases suivantes ne sont pas comptées : leurs entrants dépendent des résultats.
func RemainingMatches(p Phase) Remaining {
	if PhaseDone(p) {
		return Remaining{}
	}
	v := voir(&p)
	switch p.Format {
	case FormatSwissLives, FormatGSL:
		return v.resteVies()
	case FormatBracket, FormatLivesBracket:
		if !p.Drawn {
			if len(p.Entrants) < 2 {
				return Remaining{}
			}
			q := p
			q.Sections = Resolve(BracketSections(v.tirageFictif(), p.Length, p.Rules), p.Withdrawn)
			q.Drawn = true
			return voir(&q).resteGraphe()
		}
		return v.resteGraphe()
	case FormatRoundRobin:
		if !p.Drawn {
			if len(p.Entrants) < 2 {
				return Remaining{}
			}
			q := p
			q.Sections = Resolve(PoolSections(v.poulesFictives(), p.Length), p.Withdrawn)
			q.Drawn = true
			return voir(&q).restePoules()
		}
		return v.restePoules()
	}
	return Remaining{}
}

func (v *vue) resteVies() Remaining {
	alive := v.alive()
	if len(alive) <= 1 {
		return Remaining{v.Running, v.Running}
	}
	l, maxRem := v.livesLeft(), 0
	for _, p := range alive {
		maxRem = max(maxRem, v.remaining(p))
	}
	lo, hi := l-maxRem, l-1
	if t := v.Rules.Target; t > 0 {
		lo = min(l-t, lo)
		if v.Format == FormatSwissLives {
			hi = min(l-t, hi) // le suisse borne ses matchs à la bascule ; un bloc GSL se joue en entier
		}
	}
	return Remaining{max(lo, v.Running, 0), max(hi, v.Running, 0)}
}

func (v *vue) tirageFictif() []Ref {
	ids := make([]Ref, len(v.Entrants))
	lives := make(map[Ref]int, len(v.Entrants))
	for i, e := range v.Entrants {
		ids[i], lives[e.Player] = e.Player, e.Lives
	}
	return DrawSlots(ids, lives, "", nil, 0)
}

func (v *vue) poulesFictives() [][]Ref {
	n := len(v.Entrants)
	ng := (n + v.Rules.GroupSize - 1) / v.Rules.GroupSize
	if ng < 1 {
		ng = 1
	}
	groups := make([][]Ref, ng)
	for i, e := range v.Entrants {
		groups[i%ng] = append(groups[i%ng], e.Player)
	}
	return groups
}

// Ce qu'un nœud fait sortir : une place vide, un retiré (qui perd tout ce qui l'attend par
// forfait) ou un joueur qui joue.
const (
	sortieVide = iota
	sortieRetire
	sortieJoueur
)

type issue struct {
	gagnant, perdant int
	joue             bool // le nœud sera (ou est en train d'être) joué
	conditionnel     bool
}

func (v *vue) genre(p Ref) int {
	switch {
	case p == Bye:
		return sortieVide
	case v.withdrawn[p]:
		return sortieRetire
	}
	return sortieJoueur
}

// issueDe : ce que donnera le nœud (si, ni), mémoïsé.
func (v *vue) issueDe(si, ni int, memo map[Slot]issue) issue {
	k := Slot{si, ni}
	if x, ok := memo[k]; ok {
		return x
	}
	sec := &v.Sections[si]
	g := &sec.Nodes[ni]
	var x issue
	switch {
	case g.Done:
		x = issue{gagnant: v.genre(g.Winner), perdant: v.genre(g.Loser)}
		if g.Skipped {
			x = issue{gagnant: sortieVide, perdant: sortieVide}
		}
	case g.Match != "":
		x = issue{gagnant: sortieJoueur, perdant: sortieJoueur, joue: true}
	default:
		var place [2]int
		for k := 0; k < 2; k++ {
			src := g.Src[k]
			switch {
			case g.Players[k] != "":
				place[k] = v.genre(g.Players[k])
			case src.From >= 0:
				sj := si
				if src.Section != "" {
					sj = sectionIndex(v.Sections, src.Section)
				}
				if sj < 0 || src.From >= len(v.Sections[sj].Nodes) {
					place[k] = sortieVide
					continue
				}
				amont := v.issueDe(sj, src.From, memo)
				if src.Loser {
					place[k] = amont.perdant
				} else {
					place[k] = amont.gagnant
				}
			default:
				place[k] = sortieVide
			}
		}
		a, b := place[0], place[1]
		switch {
		case a == sortieVide || b == sortieVide:
			x = issue{gagnant: max(a, b), perdant: sortieVide}
		case a == sortieRetire || b == sortieRetire:
			x = issue{gagnant: max(a, b), perdant: sortieRetire}
		default:
			x = issue{gagnant: sortieJoueur, perdant: sortieJoueur, joue: true}
		}
		x.conditionnel = g.Cond
	}
	memo[k] = x
	return x
}

func (v *vue) resteGraphe() Remaining {
	memo := map[Slot]issue{}
	var r Remaining
	for si := range v.Sections {
		for ni := range v.Sections[si].Nodes {
			x := v.issueDe(si, ni, memo)
			if !x.joue {
				continue
			}
			r.Max++
			if !x.conditionnel {
				r.Min++
			}
		}
	}
	return r
}

func (v *vue) restePoules() Remaining {
	r := v.resteGraphe()
	for i := range v.Sections {
		sec := v.Sections[i]
		if sec.Kind != SectionPool {
			continue
		}
		if !sec.Done() {
			if n := len(PoolWins(sec)); n > v.Rules.Qualifiers {
				r.Max += 2*n - 1
			}
			continue
		}
		tied, spots := PoolTie(sec, v.Rules.Qualifiers)
		if len(tied) == 0 {
			continue
		}
		bs := v.section(BarrageSectionName(sec.Name))
		if bs == nil {
			r.Min += 2 * (len(tied) - spots)
			r.Max += 2*len(tied) - spots
			continue
		}
		losses, running := v.barrageLosses(bs.Name)
		alive := v.barrageAlive(bs)
		if len(alive) <= bs.Spots {
			r.Min += running
			r.Max += running
			continue
		}
		var vies []int
		total := 0
		for _, p := range alive {
			vies = append(vies, 2-losses[p])
			total += 2 - losses[p]
		}
		sort.Ints(vies)
		lo := 0
		for _, n := range vies[:len(alive)-bs.Spots] {
			lo += n
		}
		r.Min += max(lo, running)
		r.Max += max(total-bs.Spots, running)
	}
	return r
}
