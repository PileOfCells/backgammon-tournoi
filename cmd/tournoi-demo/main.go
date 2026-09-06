// tournoi-demo : joue un tournoi fictif avec le moteur et écrit le journal, la page d'affichage
// et les SVG dans un dossier ; ou rejoue un journal existant et affiche ce que le TD doit faire.
//
//	tournoi-demo -format suisse_tableau -joueurs 32 -sortie demo/
//	tournoi-demo -rejouer demo/journal.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/render"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

var formats = map[string]tournoi.Config{
	"suisse":         {Name: "Suisse 2 vies continu", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}},
	"suisse_tableau": {Name: "Suisse 2 vies puis tableau à vies", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Target: 16}, {Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11}}},
	"gsl":            {Name: "Blocs GSL puis tableau", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindGSL, Length: 7, Target: 16}, {Kind: tournoi.KindLivesBracket, Length: 9}}},
	"elim":           {Name: "Élimination simple", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, FinalLength: 11}}},
	"conso":          {Name: "Principal, consolante, dernière chance", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true, LastChance: true}}},
	"double":         {Name: "Double élimination", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7, Consolation: true, Reconciliation: true, Recharge: true}}},
	"poules":         {Name: "Poules puis tableau", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindRoundRobin, Length: 5, GroupSize: 4, Qualifiers: 2}, {Kind: tournoi.KindBracket, Length: 9}}},
}

func main() {
	format := flag.String("format", "suisse_tableau", "suisse, suisse_tableau, gsl, elim, conso, double, poules")
	P := flag.Int("joueurs", 32, "nombre de joueurs")
	seed := flag.Int64("graine", 1, "graine")
	out := flag.String("sortie", "demo", "dossier de sortie")
	rejouer := flag.String("rejouer", "", "journal JSON à rejouer")
	etapes := flag.Bool("etapes", false, "écrire aussi les pages d'affichage à plusieurs stades du tournoi")
	flag.Parse()

	if *rejouer != "" {
		b, err := os.ReadFile(*rejouer)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		j, err := tournoi.ParseJournal(b)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		st, err := tournoi.Replay(j)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s : %d événements, %d joueurs, phase %d, terminé=%v\n", st.Config.Name, st.NEvents, len(st.Order), st.Current+1, st.Finished)
		for _, a := range st.Propose() {
			fmt.Println(" -", a)
		}
		for _, w := range st.Warnings {
			fmt.Println(" ! ", w)
		}
		return
	}
	cfg, ok := formats[*format]
	if !ok {
		fmt.Fprintln(os.Stderr, "format inconnu")
		os.Exit(1)
	}
	cfg.Prizes = []float64{40, 20, 10, 5, 5, 5, 5, 5, 2.5, 2.5}
	players := sim.Champ(*P, 6, 2, 2, 10, rand.New(rand.NewSource(*seed)))
	r := sim.Run(cfg, players, sim.Options{Seed: *seed})
	if r.Err != nil {
		fmt.Fprintln(os.Stderr, "erreur :", r.Err)
		os.Exit(1)
	}
	_ = os.MkdirAll(*out, 0o755)
	jb, _ := r.Journal.Bytes()
	_ = os.WriteFile(filepath.Join(*out, "journal.json"), jb, 0o644)
	now := r.State.Last
	_ = os.WriteFile(filepath.Join(*out, "affichage.html"), []byte(render.Page(r.State, nil, now)), 0o644)
	_ = os.WriteFile(filepath.Join(*out, "classement.csv"), r.State.StandingsCSV(), 0o644)
	for _, ph := range r.State.Phases {
		for _, sec := range ph.Sections {
			if len(sec.Rounds) > 0 {
				_ = os.WriteFile(filepath.Join(*out, fmt.Sprintf("phase%d_%s.svg", ph.Index+1, sec.Name)), []byte(render.BracketSVG(r.State, sec)), 0o644)
			}
		}
	}
	if *etapes {
		fractions := []float64{0.15, 0.4, 0.7, 1.0}
		for k, fr := range fractions {
			n := int(float64(len(r.Journal)) * fr)
			if n < 1 {
				n = 1
			}
			st, err := tournoi.Replay(r.Journal[:n])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			acts := st.Propose()
			_ = os.WriteFile(filepath.Join(*out, fmt.Sprintf("etape%d_affichage.html", k+1)), []byte(render.Page(st, acts, st.Last)), 0o644)
			for _, ph := range st.Phases {
				for _, sec := range ph.Sections {
					if len(sec.Rounds) > 0 {
						_ = os.WriteFile(filepath.Join(*out, fmt.Sprintf("etape%d_phase%d_%s.svg", k+1, ph.Index+1, sec.Name)), []byte(render.BracketSVG(st, sec)), 0o644)
					}
				}
			}
		}
	}
	// page « à mi-tournoi » : rejeu partiel + prévision de fin
	half := r.Journal[:len(r.Journal)*2/3]
	st, _ := tournoi.Replay(half)
	mid := st.Last
	acts := st.Propose()
	_ = os.WriteFile(filepath.Join(*out, "affichage_mi_tournoi.html"), []byte(render.Page(st, acts, mid)), 0o644)
	prev, err := sim.Forecast(half, mid, 50, cfg.MinPerPoint, *seed)
	if err == nil && len(prev) > 0 {
		fmt.Printf("À mi-tournoi (%s) : fin prévue dans %.0f min (médiane), %.0f min (90 %%) ; fin réelle %.0f min plus tard\n",
			mid.Format("15:04"), prev[len(prev)/2], prev[len(prev)*9/10], now.Sub(mid).Minutes())
	}
	sb, _ := json.MarshalIndent(r.State.Final[:min(8, len(r.State.Final))], "", " ")
	fmt.Printf("%s : %d joueurs, %d matchs, %.0f min ; vainqueur %s (PR %.2f)\n", cfg.Name, *P, r.NMatches, r.Minutes, r.Winner, r.State.Players[r.Winner].Rating)
	fmt.Println(string(sb))
	fmt.Println("fichiers écrits dans", *out)
}
