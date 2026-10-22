// Package dores décrit les fichiers dorés des règles pures (rules/testdata) : leur forme JSON,
// leur lecture et leur réécriture.
//
// Un même fichier est rejoué deux fois : par les tests de rules, contre les fonctions pures, et
// par ceux du moteur (racine, sim, render), contre les enveloppes. Les types sont partagés ici,
// et seulement par des tests : rien de ce paquet n'est appelé par le moteur. Un client du module
// (blunderDB, gammonGo) ne peut pas importer ce paquet interne : il rejoue les fichiers JSON en
// les décodant avec ses propres types.
package dores

import (
	"bytes"
	"encoding/json"
	"os"
	"time"
)

// ---- Dotation (dotation.json) ----

// Placed : une place au classement, ex æquo partagés.
type Placed struct {
	Player string `json:"player"`
	Rank   int    `json:"rank"`
}

// PrizeCase : une dotation et le classement sur lequel elle se répartit.
type PrizeCase struct {
	Name      string    `json:"name"`
	EntryFee  float64   `json:"entry_fee"`
	Players   int       `json:"players"`
	RetAmount float64   `json:"retention_amount,omitempty"`
	RetPct    float64   `json:"retention_percent,omitempty"`
	Percents  []float64 `json:"percents,omitempty"`
	Amounts   []float64 `json:"amounts,omitempty"`
	Ranking   []Placed  `json:"ranking"`
	Out       PrizeOut  `json:"out"`
}

// PrizeOut : pool brut, pool distribuable, montants par place, part de chaque joueur.
type PrizeOut struct {
	Pool          float64            `json:"pool"`
	Distributable float64            `json:"distributable"`
	Amounts       []float64          `json:"amounts"`
	Shares        map[string]float64 `json:"shares"`
}

// ---- Rythme observé (horloge.json) ----

// TimedMatch : ce que le rythme et le temps de jeu lisent d'un match.
type TimedMatch struct {
	ID      string    `json:"id"`
	Status  string    `json:"status"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Length  int       `json:"length,omitempty"`
	Forfeit bool      `json:"forfeit,omitempty"`
}

// ClockCase : l'avancement d'un tournoi à un instant.
type ClockCase struct {
	Name        string       `json:"name"`
	Now         time.Time    `json:"now"`
	Start       time.Time    `json:"start"`
	MinPerPoint float64      `json:"min_per_point"`
	Factor      float64      `json:"factor"`
	Matches     []TimedMatch `json:"matches"`
	Out         ClockOut     `json:"out"`
}

// ClockOut : les compteurs, les matchs lents et la durée attendue par longueur.
type ClockOut struct {
	Played   int                      `json:"played"`
	Running  int                      `json:"running"`
	Elapsed  time.Duration            `json:"elapsed"`
	AvgMatch time.Duration            `json:"avg_match"`
	AvgPerPt time.Duration            `json:"avg_per_pt"`
	Slow     []string                 `json:"slow"`
	Expected map[string]time.Duration `json:"expected"`
}

// ---- Fin estimée (fin.json), arithmétique reprise de blunderDB direction/clock.go ----

// Interval : un créneau [Start, End).
type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// PlayingTimeCase : temps de jeu, union des intervalles de match.
type PlayingTimeCase struct {
	Name    string        `json:"name"`
	Now     time.Time     `json:"now"`
	Matches []TimedMatch  `json:"matches"`
	Out     time.Duration `json:"out"`
}

// DayCase : jour de jeu.
type DayCase struct {
	Start time.Time `json:"start"`
	Now   time.Time `json:"now"`
	Out   int       `json:"out"`
}

// BreaksCase : le jeu restant posé après now, pauses sautées.
type BreaksCase struct {
	Name   string        `json:"name"`
	Now    time.Time     `json:"now"`
	Play   time.Duration `json:"play"`
	Breaks []Interval    `json:"breaks"`
	Out    time.Time     `json:"out"`
}

// EndCase : la fin estimée, médiane des fins simulées reportée après les pauses.
type EndCase struct {
	Name      string          `json:"name"`
	Now       time.Time       `json:"now"`
	Remaining []time.Duration `json:"remaining"` // triées, comme sim.Forecast les rend
	Breaks    []Interval      `json:"breaks"`
	Out       time.Time       `json:"out"`
	OK        bool            `json:"ok"`
}

// End : le fichier fin.json.
type End struct {
	PlayingTime  []PlayingTimeCase `json:"playing_time"`
	DayOfPlay    []DayCase         `json:"day_of_play"`
	AfterBreaks  []BreaksCase      `json:"after_breaks"`
	EstimatedEnd []EndCase         `json:"estimated_end"`
}

// ---- Feuille d'appariements (feuille.json) ----

// SheetMatch : ce que la feuille lit d'un match.
type SheetMatch struct {
	ID     string    `json:"id"`
	Phase  int       `json:"phase"`
	Status string    `json:"status"`
	Start  time.Time `json:"start"`
	Table  int       `json:"table,omitempty"`
	Length int       `json:"length"`
	A      string    `json:"a"`
	B      string    `json:"b"`
}

// SheetCase : les lots de la phase courante et la feuille d'un lot.
type SheetCase struct {
	Name    string       `json:"name"`
	Current int          `json:"current"`
	Round   int          `json:"round"`
	Matches []SheetMatch `json:"matches"`
	Out     SheetOut     `json:"out"`
}

// SheetOut : les lots (identifiants de match), le lot retenu et ses lignes dans l'ordre imprimé.
type SheetOut struct {
	Batches [][]string `json:"batches"`
	Round   int        `json:"round"`
	Rows    []string   `json:"rows"`
}

// Read lit un fichier doré.
func Read(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Write réécrit un fichier doré, indenté, terminé par un saut de ligne.
func Write(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Same : v s'écrit exactement comme le contenu du fichier — la comparaison se fait sur le JSON
// produit, si bien qu'un écart d'un seul flottant se voit.
func Same(path string, v any) (bool, string, error) {
	want, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return false, "", err
	}
	return bytes.Equal(want, buf.Bytes()), buf.String(), nil
}
