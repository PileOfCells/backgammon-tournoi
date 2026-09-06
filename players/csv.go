// Package players : import léger de listes de joueurs (CSV HelloAsso, FFBG, tableur).
package players

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/PileOfCells/backgammon-tournoi"
)

// FromCSV lit un CSV (séparateur ; , ou tabulation détecté ; en-têtes reconnus de façon souple :
// nom/name/prénom/first/last, club, pr/rating/elo, id). Sans colonne id, l'identifiant est
// dérivé du nom. Les lignes vides sont ignorées.
func FromCSV(b []byte) ([]tournoi.Player, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	sep := detectSep(b)
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = sep
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("csv vide")
	}
	head := rows[0]
	col := func(keys ...string) int {
		for i, h := range head {
			h = strings.ToLower(strings.TrimSpace(h))
			for _, k := range keys {
				if strings.Contains(h, k) {
					return i
				}
			}
		}
		return -1
	}
	iID := col("id")
	iName := col("nom complet", "name", "nom")
	iFirst := col("prénom", "prenom", "first")
	iLast := col("nom de famille", "last")
	iClub := col("club")
	iPR := col("pr", "rating", "elo")
	if iName < 0 && iFirst < 0 {
		return nil, fmt.Errorf("aucune colonne de nom reconnue dans %v", head)
	}
	seen := map[tournoi.PlayerID]bool{}
	var out []tournoi.Player
	for _, row := range rows[1:] {
		get := func(i int) string {
			if i < 0 || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		name := get(iName)
		if name == "" || iFirst >= 0 && iLast >= 0 && get(iFirst) != "" {
			name = strings.TrimSpace(get(iFirst) + " " + get(iLast))
		}
		if name == "" {
			continue
		}
		id := tournoi.PlayerID(get(iID))
		if id == "" {
			id = tournoi.PlayerID(slug(name))
		}
		for k := 2; seen[id]; k++ {
			id = tournoi.PlayerID(fmt.Sprintf("%s-%d", slug(name), k))
		}
		seen[id] = true
		p := tournoi.Player{ID: id, Name: name, Club: get(iClub)}
		if v := strings.ReplaceAll(get(iPR), ",", "."); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				p.Rating = f
			}
		}
		out = append(out, p)
	}
	return out, nil
}

func detectSep(b []byte) rune {
	line := b
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		line = b[:i]
	}
	best, sep := 0, ','
	for _, c := range []byte{';', ',', '\t'} {
		if n := bytes.Count(line, []byte{c}); n > best {
			best, sep = n, rune(c)
		}
	}
	return sep
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '\'':
			b.WriteRune('-')
		default:
			if r > 127 {
				b.WriteRune(r)
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
