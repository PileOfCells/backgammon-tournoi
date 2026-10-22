package rules

import (
	"math"
	"sort"
	"time"
)

// La prévision de fin se lit en trois temps : le RYTHME observé (matchs joués, en cours, durées
// moyennes), le TEMPS DE JEU et le JOUR, puis la FIN ESTIMÉE, médiane des fins simulées reportée
// au-delà des pauses. La simulation elle-même rejoue le moteur (sim.Forecast) ; ses pièces
// arithmétiques sont ici.

// Pace résume l'avancement à un instant.
type Pace struct {
	Played   int           `json:"played"`
	Running  int           `json:"running"`
	Elapsed  time.Duration `json:"elapsed"`
	AvgMatch time.Duration `json:"avg_match"`  // durée moyenne observée des matchs terminés
	AvgPerPt time.Duration `json:"avg_per_pt"` // durée moyenne observée par point
}

// ExpectedDuration : durée attendue d'un match de n points à minPerPoint minutes le point.
func ExpectedDuration(points int, minPerPoint float64) time.Duration {
	return time.Duration(minPerPoint * float64(points) * float64(time.Minute))
}

// IsSlow : le match, lancé à m.Start, dépasse à now `factor` fois sa durée attendue. La règle ne
// regarde pas l'état du match : l'appelant ne la pose que sur les matchs en cours.
func IsSlow(m Match, now time.Time, factor, minPerPoint float64) bool {
	return now.Sub(m.Start) > time.Duration(factor*float64(ExpectedDuration(m.Length, minPerPoint)))
}

// PaceAt : le rythme observé à now, pour un tournoi commencé à start. Un match terminé compte
// s'il a été joué (ni forfait, ni fin inconnue) ; un match annulé ne compte pas.
func PaceAt(matches []Match, now, start time.Time) Pace {
	c := Pace{Elapsed: now.Sub(start)}
	var total time.Duration
	pts := 0
	for _, m := range matches {
		switch m.Status {
		case Finished:
			if !m.Forfeit && !m.End.IsZero() {
				c.Played++
				total += m.End.Sub(m.Start)
				pts += m.Length
			}
		case Running:
			c.Running++
		}
	}
	if c.Played > 0 {
		c.AvgMatch = total / time.Duration(c.Played)
	}
	if pts > 0 {
		c.AvgPerPt = total / time.Duration(pts)
	}
	return c
}

// PlayingTime : le temps pendant lequel au moins un match se jouait, union des intervalles
// [début, fin) jusqu'à now. Un match en cours, sans fin, ou dont la fin est après now compte
// jusqu'à now ; un match annulé ou jamais lancé ne compte pas. Les trous où toutes les tables
// étaient vides (une nuit, un repas, une semaine entre deux rondes de club) ne comptent pas.
func PlayingTime(matches []Match, now time.Time) time.Duration {
	type span struct{ a, b time.Time }
	var spans []span
	for _, m := range matches {
		if m.Start.IsZero() || m.Status == Cancelled {
			continue
		}
		end := m.End
		if m.Status == Running || end.IsZero() || end.After(now) {
			end = now
		}
		if end.After(m.Start) {
			spans = append(spans, span{m.Start, end})
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].a.Before(spans[j].a) })
	var total time.Duration
	var cur span
	for i, s := range spans {
		if i == 0 {
			cur = s
			continue
		}
		if !s.a.After(cur.b) {
			if s.b.After(cur.b) {
				cur.b = s.b
			}
			continue
		}
		total += cur.b.Sub(cur.a)
		cur = s
	}
	if len(spans) > 0 {
		total += cur.b.Sub(cur.a)
	}
	return total
}

// DayOfPlay : le jour calendaire de now compté depuis celui de start (1 = le même jour), dans le
// fuseau de now ; 0 quand rien n'a commencé (start nul).
func DayOfPlay(start, now time.Time) int {
	if start.IsZero() {
		return 0
	}
	s := start.In(now.Location())
	a := time.Date(s.Year(), s.Month(), s.Day(), 12, 0, 0, 0, time.UTC)
	b := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours()/24) + 1
}

// AfterBreaks pose `play` à partir de now et dit quand il se termine, pauses sautées : une pause
// en cours retarde le départ, une pause rencontrée en chemin ajoute sa longueur. Un jeu qui finit
// exactement au début d'une pause finit avant elle. Une nuit non déclarée compte comme du jeu.
func AfterBreaks(now time.Time, play time.Duration, breaks []Interval) time.Time {
	bs := append([]Interval(nil), breaks...)
	sort.Slice(bs, func(i, j int) bool { return bs[i].Start.Before(bs[j].Start) })
	t := now
	left := play
	for _, b := range bs {
		if !b.End.After(t) {
			continue
		}
		if b.Start.After(t) {
			if run := b.Start.Sub(t); run >= left {
				break
			} else {
				left -= run
			}
		}
		t = b.End
	}
	return t.Add(left)
}

// EstimatedEnd : la fin estimée, médiane des durées restantes simulées (en minutes, telles que
// sim.Forecast les rend) posée après now et reportée au-delà des pauses. La médiane d'un nombre
// pair de fins est la plus haute des deux du milieu. ok est faux sans aucune fin simulée.
func EstimatedEnd(now time.Time, remaining []float64, breaks []Interval) (end time.Time, ok bool) {
	if len(remaining) == 0 {
		return time.Time{}, false
	}
	r := append([]float64(nil), remaining...)
	sort.Float64s(r)
	median := r[len(r)/2]
	return AfterBreaks(now, time.Duration(median*float64(time.Minute)), breaks), true
}

// WinProbability : probabilité que A (PR a) batte B (PR b) en n points, formule Elo/FIBS
// (3 PR = 100 Elo).
func WinProbability(a, b float64, points int) float64 {
	d := (b - a) * 100 / 3
	return 1 / (1 + math.Pow(10, -d*math.Sqrt(float64(points))/2000))
}

// MeanRating : la cote prêtée à un joueur de cote inconnue, moyenne des cotes connues (> 0), ou
// 6 quand aucune ne l'est. La somme suit l'ordre donné.
func MeanRating(ratings []float64) float64 {
	mean, n := 0.0, 0
	for _, r := range ratings {
		if r > 0 {
			mean += r
			n++
		}
	}
	if n > 0 {
		return mean / float64(n)
	}
	return 6
}

// RemainingDuration : ce qui reste à jouer d'un match en cours, durée tirée moins temps écoulé,
// jamais moins de 10 % de la durée attendue — un match qui déborde finit quand même.
func RemainingDuration(drawn, elapsed time.Duration, points int, minPerPoint float64) time.Duration {
	rest := drawn - elapsed
	if min := time.Duration(0.1 * minPerPoint * float64(points) * float64(time.Minute)); rest < min {
		rest = min
	}
	return rest
}
