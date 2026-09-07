package tournoi_test

import (
	"encoding/csv"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
	"github.com/PileOfCells/backgammon-tournoi/sim"
)

// TestPoolRetenueEtPourcentages : 24 joueurs à 20 €, 10 % de retenue, 50/30/20 %. Les montants
// tombent juste et la somme distribuée égale exactement le pool après retenue.
func TestPoolRetenueEtPourcentages(t *testing.T) {
	cfg := tournoi.Config{Name: "Open", Phases: []tournoi.PhaseConfig{
		{Kind: tournoi.KindBracket, Length: 9}},
		Prizes: tournoi.PrizePool{
			EntryFee:  20,
			Retention: tournoi.Retention{Percent: 10},
			Sections:  map[string]tournoi.PrizeScale{tournoi.PrizeSectionAll: {Percents: []float64{50, 30, 20}}},
		}}
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	st, _, err := tournoi.New(cfg, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sim.Champ(24, 6, 2, 2, 10, rand.New(rand.NewSource(1))) {
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	if st.Pool() != 480 {
		t.Errorf("pool %v, attendu 480", st.Pool())
	}
	if st.Distributable() != 432 {
		t.Errorf("distribuable %v, attendu 432", st.Distributable())
	}
	montants := st.PrizeAmounts(tournoi.PrizeSectionAll)
	somme := 0.0
	for i, m := range montants {
		if m != math.Trunc(m) {
			t.Errorf("place %d : %v n'est pas arrondi à l'unité", i+1, m)
		}
		somme += m
	}
	if somme != 432 {
		t.Errorf("somme distribuée %v, attendu 432 (le pool après retenue)", somme)
	}
	veut := []float64{216, 130, 86}
	for i := range veut {
		if montants[i] != veut[i] {
			t.Fatalf("montants %v, attendu %v", montants, veut)
		}
	}
}

// TestResteAuPremier : les pourcentages d'une affiche ne tombent presque jamais juste. Le reste
// va au premier, et la somme reste exacte.
func TestResteAuPremier(t *testing.T) {
	cfg := tournoi.Config{Name: "Open", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9}},
		Prizes: tournoi.PrizePool{EntryFee: 17, Retention: tournoi.Retention{Amount: 13},
			Sections: map[string]tournoi.PrizeScale{tournoi.PrizeSectionAll: {Percents: []float64{40, 25, 20, 15}}}}}
	now := time.Now()
	st, _, err := tournoi.New(cfg, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 23; i++ {
		p := tournoi.Player{ID: tournoi.PlayerID("j" + strconv.Itoa(i)), Name: "J" + strconv.Itoa(i)}
		if err := st.Apply(tournoi.PlayerAddedEvent(p, now)); err != nil {
			t.Fatal(err)
		}
	}
	total := st.Distributable() // 23*17 - 13 = 378
	if total != 378 {
		t.Fatalf("distribuable %v, attendu 378", total)
	}
	m := st.PrizeAmounts(tournoi.PrizeSectionAll)
	somme := 0.0
	for _, v := range m {
		if v != math.Trunc(v) {
			t.Errorf("montant non entier : %v", v)
		}
		somme += v
	}
	if somme != 378 {
		t.Errorf("montants %v : somme %v, attendu 378", m, somme)
	}
}

// TestExAequoPartagentLesPlaces : deux demi-finalistes classés troisièmes se partagent les prix
// des 3ᵉ et 4ᵉ places.
func TestExAequoPartagentLesPlaces(t *testing.T) {
	ranking := []tournoi.Rank{
		{Player: "a", Rank: 1}, {Player: "b", Rank: 2},
		{Player: "c", Rank: 3}, {Player: "d", Rank: 3},
	}
	pr := tournoi.Prizes(ranking, []float64{200, 120, 80, 40})
	if pr["c"] != 60 || pr["d"] != 60 {
		t.Errorf("les demi-finalistes devraient toucher (80+40)/2 = 60 : c=%v d=%v", pr["c"], pr["d"])
	}
	if pr["a"] != 200 || pr["b"] != 120 {
		t.Errorf("a=%v b=%v", pr["a"], pr["b"])
	}
}

// TestConsolanteADotationPropre : la consolante distribue sa propre dotation sur SON classement.
// Son vainqueur est premier de la consolante, alors qu'il est loin dans le classement général.
func TestConsolanteADotationPropre(t *testing.T) {
	cfg := tournoi.Config{Name: "Open",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true}},
		Prizes: tournoi.PrizePool{EntryFee: 20, Retention: tournoi.Retention{Percent: 10},
			Sections: map[string]tournoi.PrizeScale{
				tournoi.PrizeSectionAll: {Percents: []float64{50, 25}},
				"conso":                 {Percents: []float64{15, 10}},
			}}}
	joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(4)))
	r := sim.Run(cfg, joueurs, sim.Options{Seed: 4})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	st := r.State
	conso := st.SectionRanking("conso")
	if len(conso) == 0 {
		t.Fatal("classement de consolante vide")
	}
	if conso[0].Note.Kind != tournoi.NoteSectionWinner {
		t.Errorf("le premier de la consolante devrait en être le vainqueur : %+v", conso[0])
	}
	// il n'est pas premier du classement général
	for _, rk := range st.Final {
		if rk.Player == conso[0].Player && rk.Rank == 1 {
			t.Error("le vainqueur de la consolante ne peut pas être premier du tournoi")
		}
	}
	// 16 joueurs × 20 € = 320, moins 10 % = 288 ; conso = 15 % et 10 %
	if got := st.PrizeAmounts("conso"); len(got) != 2 || got[0] != 43 || got[1] != 29 {
		t.Errorf("dotation de consolante %v, attendu [43 29]", got)
	}
	pr := st.SectionPrizes("conso")
	if pr[conso[0].Player] != 43 {
		t.Errorf("le vainqueur de la consolante touche %v, attendu 43", pr[conso[0].Player])
	}
	// et le classement général a sa dotation à lui
	if got := st.PrizeAmounts(tournoi.PrizeSectionAll); len(got) != 2 || got[0] != 144 || got[1] != 72 {
		t.Errorf("dotation générale %v, attendu [144 72]", got)
	}
}

// TestCSVUneSectionParBloc : le CSV sort le classement général puis chaque section dotée, avec
// les prix.
func TestCSVUneSectionParBloc(t *testing.T) {
	cfg := tournoi.Config{Name: "Open",
		Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9, Consolation: true}},
		Prizes: tournoi.PrizePool{EntryFee: 20,
			Sections: map[string]tournoi.PrizeScale{
				tournoi.PrizeSectionAll: {Percents: []float64{50, 25}},
				"conso":                 {Percents: []float64{15}},
			}}}
	joueurs := sim.Champ(16, 6, 2, 2, 10, rand.New(rand.NewSource(6)))
	r := sim.Run(cfg, joueurs, sim.Options{Seed: 6})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	rd := csv.NewReader(strings.NewReader(string(r.State.StandingsCSV())))
	rd.Comma = ';'
	lignes, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if lignes[0][0] != "section" || lignes[0][len(lignes[0])-1] != "prix" {
		t.Fatalf("en-tête %v", lignes[0])
	}
	blocs := map[string]int{}
	var ordre []string
	for _, l := range lignes[1:] {
		if blocs[l[0]] == 0 {
			ordre = append(ordre, l[0])
		}
		blocs[l[0]]++
	}
	if len(ordre) != 2 || ordre[0] != tournoi.PrizeSectionAll || ordre[1] != "conso" {
		t.Fatalf("blocs %v, attendu [all conso]", ordre)
	}
	if blocs[tournoi.PrizeSectionAll] != 16 {
		t.Errorf("le bloc général compte %d lignes, attendu 16", blocs[tournoi.PrizeSectionAll])
	}
	if blocs["conso"] == 0 || blocs["conso"] >= 16 {
		t.Errorf("le bloc consolante compte %d lignes", blocs["conso"])
	}
	// un prix non nul apparaît dans chaque bloc
	for _, sec := range ordre {
		vu := false
		for _, l := range lignes[1:] {
			if l[0] == sec && l[len(l)-1] != "0.00" {
				vu = true
			}
		}
		if !vu {
			t.Errorf("aucun prix dans le bloc %q", sec)
		}
	}
}

// TestDotationAncienneFormeRelue : une configuration écrite « prizes: [100, 60, 40] » reste
// lisible ; on ne réécrit pas les journaux existants.
func TestDotationAncienneFormeRelue(t *testing.T) {
	cfg, err := tournoi.ParseConfig([]byte(`{"name":"T","phases":[{"kind":"bracket","length":9}],"prizes":[100,60,40]}`))
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := cfg.Prizes.Sections[tournoi.PrizeSectionAll]
	if !ok {
		t.Fatal("la liste de montants devrait devenir la dotation du classement général")
	}
	if len(sc.Amounts) != 3 || sc.Amounts[0] != 100 {
		t.Errorf("montants relus %v", sc.Amounts)
	}
	// et un journal entier qui la porte se rejoue
	j := tournoi.Journal{{Version: tournoi.JournalVersion, Kind: tournoi.EvCreated,
		Time: time.Now(), Config: cfg, Seed: 1}}
	if _, err := tournoi.Replay(j); err != nil {
		t.Fatal(err)
	}
}

// TestDotationIncoherenteRefusee : une dotation fausse ne se voit qu'au moment de payer, devant
// les joueurs.
func TestDotationIncoherenteRefusee(t *testing.T) {
	base := func(p tournoi.PrizePool) error {
		c := tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 9}}, Prizes: p}
		return c.Validate()
	}
	cas := map[string]tournoi.PrizePool{
		"plus de 100 %": {Sections: map[string]tournoi.PrizeScale{
			tournoi.PrizeSectionAll: {Percents: []float64{60, 50}}}},
		"pourcentages et montants": {Sections: map[string]tournoi.PrizeScale{
			tournoi.PrizeSectionAll: {Percents: []float64{50}, Amounts: []float64{100}}}},
		"retenue hors bornes":    {Retention: tournoi.Retention{Percent: 140}},
		"droit d'entrée négatif": {EntryFee: -1},
	}
	for nom, p := range cas {
		if err := base(p); err == nil {
			t.Errorf("%s : devrait être refusé", nom)
		}
	}
	// et une dotation répartie entre deux sections qui totalise 100 % passe
	ok := tournoi.PrizePool{Sections: map[string]tournoi.PrizeScale{
		tournoi.PrizeSectionAll: {Percents: []float64{50, 25}},
		"conso":                 {Percents: []float64{15, 10}}}}
	if err := base(ok); err != nil {
		t.Errorf("90 %% répartis entre deux sections : %v", err)
	}
}
