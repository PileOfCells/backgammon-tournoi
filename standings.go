package tournoi

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

// Prizes répartit une dotation par place sur un classement avec ex æquo : les joueurs classés
// à la même place se partagent les prix des places qu'ils occupent.
func Prizes(ranking []Rank, prizes []float64) map[PlayerID]float64 {
	out := map[PlayerID]float64{}
	byRank := map[int][]PlayerID{}
	for _, r := range ranking {
		byRank[r.Rank] = append(byRank[r.Rank], r.Player)
	}
	for rank, ids := range byRank {
		total := 0.0
		for i := rank - 1; i < rank-1+len(ids); i++ {
			if i < len(prizes) {
				total += prizes[i]
			}
		}
		for _, p := range ids {
			out[p] = total / float64(len(ids))
		}
	}
	return out
}

// StandingsCSV exporte le classement (rang, joueur, nom, club, note, prix).
func (s *State) StandingsCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	_ = w.Write([]string{"rang", "id", "nom", "club", "note", "prix"})
	ranking := s.Final
	if ranking == nil {
		ranking = s.Ranking()
	}
	pr := Prizes(ranking, s.Config.Prizes)
	for _, r := range ranking {
		p := s.Players[r.Player]
		name, club := string(r.Player), ""
		if p != nil {
			name, club = p.Name, p.Club
		}
		_ = w.Write([]string{strconv.Itoa(r.Rank), string(r.Player), name, club, r.Note.String(), fmt.Sprintf("%.2f", pr[r.Player])})
	}
	w.Flush()
	return buf.Bytes()
}
