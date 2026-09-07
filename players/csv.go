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
// nom/name/prénom/first/last, club, cote/pr/rating/elo, id). Sans colonne id, l'identifiant est
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
	// Un en-tête EXACT l'emporte sur un en-tête qui contient seulement le mot cherché : sinon
	// « prénom » réclame la colonne « pr » d'un fichier qui a par ailleurs une vraie colonne de
	// cote, et les cotes se retrouvent dans les prénoms.
	col := func(keys ...string) int {
		for _, k := range keys {
			for i, h := range head {
				if strings.ToLower(strings.TrimSpace(h)) == k {
					return i
				}
			}
		}
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
	iPR := col("cote", "pr", "rating", "elo")
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

// ToCSV écrit une liste de joueurs dans le format que FromCSV relit : colonnes « nom », « club »,
// « cote », séparateur point-virgule.
//
// C'est l'annuaire du logiciel hôte — tous les inscrits de tous ses tournois — qu'on exporte
// pour le réimporter d'un tournoi à l'autre, et qu'un organisateur ouvre dans un tableur : trois
// colonnes lisibles, pas d'identifiant technique. Une cote inconnue (0) sort en cellule vide et
// revient à 0 ; elle ne devient pas un « 0 » qui se lirait comme un joueur parfait.
//
// Une quatrième colonne « id » n'apparaît que si elle est NÉCESSAIRE : FromCSV dérive
// l'identifiant du nom, donc le cas courant s'en passe, mais une liste dont un identifiant vient
// d'ailleurs (un autre logiciel, une fédération) ne doit pas le perdre en silence.
func ToCSV(ps []tournoi.Player) []byte {
	avecID := false
	vus := map[tournoi.PlayerID]bool{}
	for _, p := range ps {
		attendu := tournoi.PlayerID(slug(p.Name))
		for k := 2; vus[attendu]; k++ {
			attendu = tournoi.PlayerID(fmt.Sprintf("%s-%d", slug(p.Name), k))
		}
		vus[attendu] = true
		if p.ID != "" && p.ID != attendu {
			avecID = true
		}
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	tête := []string{"nom", "club", "cote"}
	if avecID {
		tête = append([]string{"id"}, tête...)
	}
	_ = w.Write(tête)
	for _, p := range ps {
		cote := ""
		if p.Rating != 0 {
			cote = strconv.FormatFloat(p.Rating, 'g', -1, 64)
		}
		ligne := []string{p.Name, p.Club, cote}
		if avecID {
			ligne = append([]string{string(p.ID)}, ligne...)
		}
		_ = w.Write(ligne)
	}
	w.Flush()
	return buf.Bytes()
}
