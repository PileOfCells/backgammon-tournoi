package rules_test

import (
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
