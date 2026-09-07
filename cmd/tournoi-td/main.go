// tournoi-td : console interactive de direction de tournoi pour tester le moteur à la main.
//
// Le journal est écrit dans un fichier JSON après chaque événement ; relancer la commande sur le
// même fichier reprend le tournoi où il en était. Après chaque commande, la page d'affichage
// (affichage.html, à côté du journal) est régénérée : ouvrez-la dans un navigateur.
//
//	tournoi-td -journal montournoi.json -format suisse_tableau      # nouveau tournoi
//	tournoi-td -journal montournoi.json                             # reprise
//	tournoi-td -journal montournoi.json -config maconfig.json       # configuration JSON personnalisée
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/players"
	"github.com/PileOfCells/backgammon-tournoi/render"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

var formats = map[string]tournoi.Config{
	"suisse":         {Name: "Suisse 2 vies continu", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7}}},
	"suisse_rondes":  {Name: "Suisse 2 vies par rondes", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Mode: "rounds"}}},
	"suisse_tableau": {Name: "Suisse 2 vies puis tableau à vies", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindSwissLives, Length: 7, Target: 16}, {Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11}}},
	"gsl":            {Name: "Blocs GSL puis tableau", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindGSL, Length: 7, Target: 16}, {Kind: tournoi.KindLivesBracket, Length: 9}}},
	"elim":           {Name: "Élimination simple", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, FinalLength: 11}}},
	"conso":          {Name: "Principal, consolante, dernière chance", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true, LastChance: true}}},
	"double":         {Name: "Double élimination", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7, Consolation: true, Reconciliation: true, Recharge: true}}},
	"poules":         {Name: "Poules puis tableau", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindRoundRobin, Length: 5, GroupSize: 4, Qualifiers: 2}, {Kind: tournoi.KindBracket, Length: 9}}},
}

type td struct {
	path    string
	journal tournoi.Journal
	st      *tournoi.State
	acts    []tournoi.Action
}

func (t *td) save() {
	b, _ := t.journal.Bytes()
	_ = os.WriteFile(t.path, b, 0o644)
	page := filepath.Join(filepath.Dir(t.path), "affichage.html")
	_ = os.WriteFile(page, []byte(render.Page(t.st, t.st.ProposeAt(time.Now()), time.Now())), 0o644)
}

func (t *td) apply(ev tournoi.Event) error {
	ev.Seq = len(t.journal)
	if err := t.st.Apply(ev); err != nil {
		return err
	}
	t.journal = append(t.journal, ev)
	t.save()
	return nil
}

func (t *td) name(id tournoi.PlayerID) string {
	if p := t.st.Players[id]; p != nil {
		return fmt.Sprintf("%s (%s)", p.Name, id)
	}
	return string(id)
}

func (t *td) propose() {
	for _, i := range t.st.Infos {
		fmt.Println("  i", i)
	}
	t.acts = t.st.ProposeAt(time.Now())
	if len(t.acts) == 0 {
		fmt.Println("  rien à proposer")
		return
	}
	for i, a := range t.acts {
		switch a.Kind {
		case tournoi.ActStartMatch:
			fmt.Printf("  %2d. table %d, %s : %s contre %s, %d points\n", i+1, a.Table, a.Label, t.name(a.A), t.name(a.B), a.Length)
		case tournoi.ActDraw:
			fmt.Printf("  %2d. tirage : %s\n", i+1, a.Label)
			if a.Draw != nil {
				if len(a.Draw.Slots) > 0 {
					for j := 0; j+1 < len(a.Draw.Slots); j += 2 {
						fmt.Printf("        %s — %s\n", t.name(a.Draw.Slots[j]), t.name(a.Draw.Slots[j+1]))
					}
				}
				for j, g := range a.Draw.Groups {
					var ns []string
					for _, p := range g {
						ns = append(ns, t.name(p))
					}
					fmt.Printf("        groupe %d : %s\n", j+1, strings.Join(ns, ", "))
				}
			}
		default:
			fmt.Printf("  %2d. %s\n", i+1, a)
		}
	}
}

func (t *td) running() {
	ms := t.st.Running()
	sort.Slice(ms, func(i, j int) bool { return ms[i].Table < ms[j].Table })
	if len(ms) == 0 {
		fmt.Println("  aucun match en cours")
	}
	for _, m := range ms {
		fmt.Printf("  %s  table %d  %s : %s contre %s, %d pts, depuis %s\n", m.ID, m.Table, m.Label, t.name(m.A), t.name(m.B), m.Length, time.Since(m.Start).Round(time.Minute))
	}
}

func (t *td) findPlayer(s string) (tournoi.PlayerID, error) {
	if _, ok := t.st.Players[tournoi.PlayerID(s)]; ok {
		return tournoi.PlayerID(s), nil
	}
	ls := strings.ToLower(s)
	var hits []tournoi.PlayerID
	for id, p := range t.st.Players {
		if strings.Contains(strings.ToLower(p.Name), ls) {
			hits = append(hits, id)
		}
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	if len(hits) == 0 {
		return "", fmt.Errorf("joueur %q inconnu", s)
	}
	return "", fmt.Errorf("joueur %q ambigu : %v", s, hits)
}

func help() {
	fmt.Print(`Commandes :
  ajoute <nom> [club] [pr]     inscrire un joueur (identifiant dérivé du nom)
  import <fichier.csv>         inscrire depuis un CSV (nom, club, pr)
  exporte <fichier.csv>        écrire les inscrits en CSV (relisible par import)
  places                       places d'exemption libres pour un retardataire
  retardataire <nom> <place>   inscrire un joueur sur une place d'exemption libre
  propose                      afficher ce que le moteur propose
  ok [n | tous]                confirmer la proposition n (ou toutes)
  resultat <match> <vainqueur> [scoreA scoreB]   ex. : resultat M12 dupont 7 3
  corrige <match> <vainqueur> [scoreA scoreB]    corriger un résultat déjà saisi
  annule <match>               annuler un match lancé par erreur
  forfait <joueur>             retrait du tournoi (ses matchs en cours sont perdus)
  longueur <points>            changer la longueur des prochains matchs de la phase
  configure <fichier.json>     remplacer la configuration entière (bascule, phase ajoutée…)
  rouvre                       annuler la clôture d'un tournoi terminé
  matchs                       matchs en cours
  vies                         joueurs par nombre de défaites (phase à vies)
  classement                   classement courant
  simule                       jouer au hasard tous les matchs en cours (pour tester vite)
  aide / quitte
`)
}

func main() {
	path := flag.String("journal", "tournoi.json", "fichier journal (créé si absent)")
	format := flag.String("format", "suisse_tableau", "format d'un nouveau tournoi : "+keys())
	cfgPath := flag.String("config", "", "configuration JSON d'un nouveau tournoi (prioritaire sur -format)")
	nom := flag.String("nom", "", "nom du tournoi")
	flag.Parse()

	t := &td{path: *path}
	if b, err := os.ReadFile(*path); err == nil {
		j, err := tournoi.ParseJournal(b)
		if err != nil {
			fmt.Fprintln(os.Stderr, "journal illisible :", err)
			os.Exit(1)
		}
		st, err := tournoi.Replay(j)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rejeu :", err)
			os.Exit(1)
		}
		t.journal, t.st = j, st
		fmt.Printf("Reprise de « %s » : %d événements, %d joueurs, phase %d/%d.\n", st.Config.Name, st.NEvents, len(st.Order), st.Current+1, len(st.Config.Phases))
	} else {
		var cfg tournoi.Config
		if *cfgPath != "" {
			b, err := os.ReadFile(*cfgPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			c, err := tournoi.ParseConfig(b)
			if err != nil {
				fmt.Fprintln(os.Stderr, "config :", err)
				os.Exit(1)
			}
			cfg = *c
		} else {
			c, ok := formats[*format]
			if !ok {
				fmt.Fprintln(os.Stderr, "format inconnu ; choix :", keys())
				os.Exit(1)
			}
			cfg = c
		}
		if *nom != "" {
			cfg.Name = *nom
		}
		st, ev, err := tournoi.New(cfg, time.Now().UnixNano(), time.Now())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		t.st = st
		t.journal = tournoi.Journal{ev}
		t.save()
		fmt.Printf("Nouveau tournoi « %s » dans %s. Inscrivez les joueurs (ajoute / import) puis tapez propose.\n", cfg.Name, *path)
	}
	help()
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("td> ")
		if !in.Scan() {
			return
		}
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		if err := t.exec(f); err != nil {
			fmt.Println("  erreur :", err)
		}
		if t.st.Finished && f[0] != "classement" {
			fmt.Println("  Tournoi terminé. Tapez classement.")
		}
	}
}

func keys() string {
	var k []string
	for n := range formats {
		k = append(k, n)
	}
	sort.Strings(k)
	return strings.Join(k, ", ")
}

func (t *td) exec(f []string) error {
	now := time.Now()
	switch f[0] {
	case "aide", "help", "?":
		help()
	case "quitte", "quit", "q":
		os.Exit(0)
	case "ajoute":
		if len(f) < 2 {
			return fmt.Errorf("ajoute <nom> [club] [pr]")
		}
		p := tournoi.Player{Name: f[1], ID: tournoi.PlayerID(strings.ToLower(f[1]))}
		if len(f) >= 3 {
			p.Club = f[2]
		}
		if len(f) >= 4 {
			p.Rating, _ = strconv.ParseFloat(strings.ReplaceAll(f[3], ",", "."), 64)
		}
		return t.apply(tournoi.PlayerAddedEvent(p, now))
	case "places":
		libres := t.st.FreeSlots()
		if len(libres) == 0 {
			fmt.Println("  aucune place d'exemption libre")
			return nil
		}
		for _, sl := range libres {
			fmt.Printf("  %s (%s, %s)\n", sl.Key, sl.Section, sl.Label)
		}
		return nil
	case "retardataire":
		if len(f) < 3 {
			return fmt.Errorf("retardataire <nom> <place> (voir la commande places)")
		}
		p := tournoi.Player{Name: f[1], ID: tournoi.PlayerID(strings.ToLower(f[1]))}
		for _, sl := range t.st.FreeSlots() {
			if sl.Key == f[2] {
				return t.apply(tournoi.PlayerAddedAtSlotEvent(p, sl, now))
			}
		}
		return fmt.Errorf("place %q introuvable ou plus libre (voir la commande places)", f[2])
	case "import":
		if len(f) < 2 {
			return fmt.Errorf("import <fichier.csv>")
		}
		b, err := os.ReadFile(f[1])
		if err != nil {
			return err
		}
		ps, err := players.FromCSV(b)
		if err != nil {
			return err
		}
		for i := range ps {
			p := ps[i]
			if err := t.apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
				return err
			}
		}
		fmt.Printf("  %d joueurs inscrits\n", len(ps))
	case "exporte":
		// L'annuaire fait l'aller-retour : ce qu'on écrit ici, « import » le relit tel quel.
		if len(f) < 2 {
			return fmt.Errorf("exporte <fichier.csv>")
		}
		ps := make([]tournoi.Player, 0, len(t.st.Order))
		for _, id := range t.st.Order {
			ps = append(ps, *t.st.Players[id])
		}
		if err := os.WriteFile(f[1], players.ToCSV(ps), 0o644); err != nil {
			return err
		}
		fmt.Printf("  %d joueurs écrits dans %s\n", len(ps), f[1])
	case "propose":
		t.propose()
	case "ok":
		if len(t.acts) == 0 {
			t.acts = t.st.ProposeAt(time.Now())
		}
		which := "tous"
		if len(f) >= 2 {
			which = f[1]
		}
		var sel []tournoi.Action
		if which == "tous" {
			sel = t.acts
		} else {
			n, err := strconv.Atoi(which)
			if err != nil || n < 1 || n > len(t.acts) {
				return fmt.Errorf("numéro de proposition invalide")
			}
			sel = []tournoi.Action{t.acts[n-1]}
		}
		for _, a := range sel {
			if a.Kind == tournoi.ActWait {
				continue
			}
			ev, err := t.st.EventFromAction(a, now)
			if err != nil {
				return err
			}
			if err := t.apply(ev); err != nil {
				return err
			}
			fmt.Println("  ✔", a)
			if a.Kind == tournoi.ActDraw || a.Kind == tournoi.ActNextPhase || a.Kind == tournoi.ActFinish {
				break
			}
		}
		t.acts = nil
		t.propose()
	case "resultat", "corrige":
		if len(f) < 3 {
			return fmt.Errorf("%s <match> <vainqueur> [scoreA scoreB]", f[0])
		}
		m, ok := t.st.Matches[tournoi.MatchID(strings.ToUpper(f[1]))]
		if !ok {
			return fmt.Errorf("match %s inconnu", f[1])
		}
		w, err := t.findPlayer(f[2])
		if err != nil {
			return err
		}
		sa, sb := m.Length, 0
		if w == m.B {
			sa, sb = 0, m.Length
		}
		if len(f) >= 5 {
			sa, _ = strconv.Atoi(f[3])
			sb, _ = strconv.Atoi(f[4])
		}
		ev := tournoi.ResultEvent(m.ID, w, sa, sb, now)
		if f[0] == "corrige" {
			ev.Kind = tournoi.EvResultCorrected
		}
		if err := t.apply(ev); err != nil {
			return err
		}
		for _, wn := range t.st.Warnings {
			fmt.Println("  !", wn)
		}
		t.propose()
	case "configure":
		// La configuration change ENTIÈRE : le TD envoie le même formulaire qu'à la création.
		if len(f) < 2 {
			return fmt.Errorf("configure <fichier.json>")
		}
		b, err := os.ReadFile(f[1])
		if err != nil {
			return err
		}
		cfg, err := tournoi.ParseConfig(b)
		if err != nil {
			return err
		}
		if err := t.apply(tournoi.ConfigChangedEvent(*cfg, now)); err != nil {
			return err
		}
		fmt.Printf("  configuration remplacée : %d phase(s)\n", len(cfg.Phases))
		t.propose()
	case "rouvre":
		if err := t.apply(tournoi.ReopenedEvent(now)); err != nil {
			return err
		}
		fmt.Println("  tournoi rouvert : corrigez, puis proposez la clôture")
		t.propose()
	case "annule":
		if len(f) < 2 {
			return fmt.Errorf("annule <match>")
		}
		return t.apply(tournoi.CancelEvent(tournoi.MatchID(strings.ToUpper(f[1])), now))
	case "forfait":
		if len(f) < 2 {
			return fmt.Errorf("forfait <joueur>")
		}
		p, err := t.findPlayer(f[1])
		if err != nil {
			return err
		}
		return t.apply(tournoi.PlayerWithdrawnEvent(p, now))
	case "longueur":
		if len(f) < 2 {
			return fmt.Errorf("longueur <points>")
		}
		n, _ := strconv.Atoi(f[1])
		return t.apply(tournoi.LengthChangedEvent(t.st.Current, n, now))
	case "matchs":
		t.running()
	case "vies":
		ph := t.st.Phases[t.st.Current]
		for l := 0; l <= 3; l++ {
			var ns []string
			for _, p := range ph.Entrants {
				if ph.Losses[p] == l {
					ns = append(ns, fmt.Sprintf("%s [%dV]", t.name(p), ph.Wins[p]))
				}
			}
			if len(ns) > 0 {
				fmt.Printf("  %d défaite(s) : %s\n", l, strings.Join(ns, ", "))
			}
		}
	case "classement":
		r := t.st.Final
		if r == nil {
			r = t.st.Ranking()
		}
		pr := t.st.SectionPrizes(tournoi.PrizeSectionAll)
		for _, x := range r {
			fmt.Printf("  %2d. %-30s %s  %.0f\n", x.Rank, t.name(x.Player), x.Note, pr[x.Player])
		}
	case "simule":
		rng := sim.PGain
		_ = rng
		ms := t.st.Running()
		for _, m := range ms {
			pa, pb := t.st.Players[m.A].Rating, t.st.Players[m.B].Rating
			if pa == 0 {
				pa = 6
			}
			if pb == 0 {
				pb = 6
			}
			w := m.A
			if float64(time.Now().UnixNano()%1000)/1000 >= sim.PGain(pa, pb, m.Length) {
				w = m.B
			}
			sa, sb := m.Length, int(time.Now().UnixNano()%int64(m.Length))
			if w == m.B {
				sa, sb = sb, sa
			}
			if err := t.apply(tournoi.ResultEvent(m.ID, w, sa, sb, now)); err != nil {
				return err
			}
			fmt.Printf("  %s : %s gagne %d-%d\n", m.ID, t.name(w), sa, sb)
		}
		t.propose()
	case "etat":
		b, _ := json.MarshalIndent(t.st.Phases[t.st.Current], "", " ")
		fmt.Println(string(b))
	default:
		return fmt.Errorf("commande inconnue (aide)")
	}
	return nil
}
