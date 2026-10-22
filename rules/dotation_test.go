package rules_test

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/PileOfCells/backgammon-tournoi/rules"
)

// Un rang nul ou négatif n'est pas une place du barème : pas de prix, et pas de panique.
func TestSharePrizesRangHorsBareme(t *testing.T) {
	got := rules.SharePrizes([]rules.Placement{
		{Player: "a", Rank: 0}, {Player: "b", Rank: 0}, {Player: "c", Rank: -3}, {Player: "d", Rank: 1},
	}, []float64{100, 50})
	want := map[rules.Ref]float64{"a": 0, "b": 0, "c": 0, "d": 100}
	if len(got) != len(want) {
		t.Fatalf("parts = %v, attendu %v", got, want)
	}
	for p, v := range want {
		if got[p] != v {
			t.Errorf("%s : %v, attendu %v", p, got[p], v)
		}
	}
}

// 4 × 15 % d'un pool de 10 : 1,5 chacun, deux unités à répartir entre quatre restes égaux, qui
// vont aux deux mieux classés.
func TestPlaceAmountsResteEgalAuMeilleurRang(t *testing.T) {
	got := rules.PlaceAmounts(rules.PrizeScale{Percents: []float64{15, 15, 15, 15}}, 10)
	want := []float64{2, 2, 1, 1}
	for i := range want {
		if len(got) != len(want) || got[i] != want[i] {
			t.Fatalf("montants = %v, attendu %v", got, want)
		}
	}
}

// La plus grande partie fractionnaire l'emporte sur le rang : 33,333/33,333/33,334 % de 100.
func TestPlaceAmountsPlusFortReste(t *testing.T) {
	got := rules.PlaceAmounts(rules.PrizeScale{Percents: []float64{33.333, 33.333, 33.334}}, 100)
	want := []float64{33, 33, 34}
	for i := range want {
		if len(got) != len(want) || got[i] != want[i] {
			t.Fatalf("montants = %v, attendu %v", got, want)
		}
	}
}

// Les propriétés de la méthode du plus fort reste, sur des barèmes tirés au hasard (graine fixe)
// et sur des cas limites : aucun montant négatif, chaque montant à moins d'une unité de sa part
// exacte, somme égale au distribuable arrondi, et montants décroissants sur un barème
// décroissant.
func TestPlaceAmountsProprietes(t *testing.T) {
	type cas struct {
		pcts []float64
		dist float64
	}
	tous := []cas{
		{[]float64{15, 15, 15, 15}, 10},
		{[]float64{100}, 0},
		{[]float64{50, 30, 20}, 0.4},
		{[]float64{29, 29, 29, 13}, 100},
		{[]float64{25, 25, 25, 25}, 3},
		{[]float64{99.5, 0.5}, 1},
	}
	rng := rand.New(rand.NewSource(28))
	for range 5000 {
		n := 1 + rng.Intn(12)
		pcts := make([]float64, n)
		total := 0.0
		for i := range pcts {
			pcts[i] = rng.Float64()
			total += pcts[i]
		}
		budget := 100 * rng.Float64()
		if rng.Intn(3) == 0 {
			budget = 100
		}
		for i := range pcts {
			// des pourcentages au centième, comme sur une affiche
			pcts[i] = math.Round(pcts[i]/total*budget*100) / 100
		}
		if rng.Intn(2) == 0 {
			sort.Sort(sort.Reverse(sort.Float64Slice(pcts)))
		}
		var dist float64
		switch rng.Intn(3) {
		case 0:
			dist = float64(rng.Intn(50))
		case 1:
			dist = float64(rng.Intn(5000))
		default:
			dist = math.Round(rng.Float64()*10000*100) / 100
		}
		tous = append(tous, cas{pcts, dist})
	}
	for _, c := range tous {
		got := rules.PlaceAmounts(rules.PrizeScale{Percents: c.pcts}, c.dist)
		somme, exact := 0.0, 0.0
		for i, v := range got {
			part := c.dist * c.pcts[i] / 100
			exact += part
			somme += v
			if v < 0 || v != math.Trunc(v) {
				t.Fatalf("%v de %v : montant %v à la place %d", c.pcts, c.dist, v, i+1)
			}
			if math.Abs(v-part) >= 1+1e-6 {
				t.Fatalf("%v de %v : %v à la place %d, part exacte %v", c.pcts, c.dist, v, i+1, part)
			}
			if i > 0 && c.pcts[i-1] >= c.pcts[i] && got[i-1] < v {
				t.Fatalf("%v de %v : %v, la place %d touche moins que la %d", c.pcts, c.dist, got, i, i+1)
			}
		}
		if somme != math.Round(exact) {
			t.Fatalf("%v de %v : somme %v, attendu %v (montants %v)", c.pcts, c.dist, somme, math.Round(exact), got)
		}
	}
}
