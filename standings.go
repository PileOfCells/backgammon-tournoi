package tournoi

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// Prizes répartit une dotation par place sur un classement avec ex æquo : les joueurs classés
// à la même place se partagent les prix des places qu'ils occupent (rules.SharePrizes).
func Prizes(ranking []Rank, prizes []float64) map[PlayerID]float64 {
	places := make([]rules.Placement, len(ranking))
	for i, r := range ranking {
		places[i] = rules.Placement{Player: rules.Ref(r.Player), Rank: r.Rank}
	}
	out := map[PlayerID]float64{}
	for p, v := range rules.SharePrizes(places, prizes) {
		out[PlayerID(p)] = v
	}
	return out
}

// StandingsCSV exporte le classement, une section par bloc.
//
// La première colonne dit à quel classement la ligne appartient : « all » pour le classement
// général, puis l'identifiant de chaque section dotée (« main », « conso »…). Les blocs sont
// contigus et se lisent dans un tableur comme des tableaux séparés ; un hôte qui n'en veut
// qu'un filtre sur la colonne.
//
// Les sections apparaissent parce qu'elles ont une DOTATION : sortir le classement de chaque
// groupe GSL d'un tournoi de cent joueurs noierait la feuille que le TD affiche au mur.
func (s *State) StandingsCSV() []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	_ = w.Write([]string{"section", "rang", "id", "nom", "club", "note", "prix"})
	bloc := func(section string, ranking []Rank, prix map[PlayerID]float64) {
		for _, r := range ranking {
			p := s.Players[r.Player]
			name, club := string(r.Player), ""
			if p != nil {
				name, club = p.Name, p.Club
			}
			_ = w.Write([]string{section, strconv.Itoa(r.Rank), string(r.Player), name, club,
				r.Note.String(), fmt.Sprintf("%.2f", prix[r.Player])})
		}
	}
	ranking := s.Final
	if ranking == nil {
		ranking = s.Ranking()
	}
	bloc(PrizeSectionAll, ranking, Prizes(ranking, s.PrizeAmounts(PrizeSectionAll)))
	for _, name := range s.Config.Prizes.SectionNames() {
		if name == PrizeSectionAll {
			continue
		}
		r := s.SectionRanking(name)
		if len(r) == 0 {
			continue
		}
		bloc(name, r, Prizes(r, s.PrizeAmounts(name)))
	}
	w.Flush()
	return buf.Bytes()
}
