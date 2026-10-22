package rules

import (
	"sort"
	"time"
)

// Batches : les lots de matchs d'une phase, dans l'ordre de leur départ.
//
// Un « lot » est un paquet de matchs lancés au même instant : une ronde dans un suisse par
// rondes, un bloc dans un GSL, un tour dans un tableau, une micro-ronde dans un suisse continu
// à batch_minutes. L'instant de départ le dit, sans autre mot. Dans un lot, les matchs gardent
// l'ordre donné ; les matchs annulés et ceux d'une autre phase sont écartés. Une phase négative
// (rien n'a commencé) n'a aucun lot.
func Batches(matches []Match, phase int) [][]Match {
	if phase < 0 {
		return nil
	}
	parInstant := map[time.Time][]Match{}
	var instants []time.Time
	for _, m := range matches {
		if m.Phase != phase || m.Status == Cancelled {
			continue
		}
		if _, vu := parInstant[m.Start]; !vu {
			instants = append(instants, m.Start)
		}
		parInstant[m.Start] = append(parInstant[m.Start], m)
	}
	sort.Slice(instants, func(i, j int) bool { return instants[i].Before(instants[j]) })
	out := make([][]Match, 0, len(instants))
	for _, t := range instants {
		out = append(out, parInstant[t])
	}
	return out
}

// Sheet est la feuille d'appariements d'un lot : son numéro (1 = le premier de la phase) et ses
// lignes, une par match, dans l'ordre où elles s'impriment. Round vaut 0 quand la phase n'a aucun
// lot.
type Sheet struct {
	Round int     `json:"round"`
	Rows  []Match `json:"rows"`
}

// PairingSheet : la feuille du lot `round` de la phase. 0, un nombre négatif ou un numéro au-delà
// du dernier lot donnent le lot le plus récent, celui qu'on imprime en pratique. Les lignes sont
// triées par table, puis par identifiant de match.
func PairingSheet(matches []Match, phase, round int) Sheet {
	lots := Batches(matches, phase)
	if len(lots) == 0 {
		return Sheet{}
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
	return Sheet{Round: round, Rows: lot}
}
