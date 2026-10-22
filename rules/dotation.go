package rules

import (
	"math"
	"sort"
)

// Ce qui est calculé ici s'arrête au montant par PLACE, puis au partage entre ex æquo : deux
// demi-finalistes classés troisièmes se partagent les prix des 3ᵉ et 4ᵉ places. Quelle section
// est dotée et sur quel classement, c'est l'affaire de l'hôte.

// PrizeScale est la dotation d'une section : des pourcentages du pool distribuable, ou des
// montants fixes. Amounts l'emporte quand les deux sont donnés.
type PrizeScale struct {
	Percents []float64 `json:"percents,omitempty"`
	Amounts  []float64 `json:"amounts,omitempty"`
}

// Retention est ce que l'organisation garde avant de distribuer : le pourcentage s'applique au
// pool brut, puis le montant s'en retranche.
type Retention struct {
	Amount  float64 `json:"amount,omitempty"`
	Percent float64 `json:"percent,omitempty"`
}

// Placement est une place au classement ; les ex æquo partagent le même Rank.
type Placement struct {
	Player Ref `json:"player"`
	Rank   int `json:"rank"`
}

// Pool : le pool brut, droit d'entrée fois nombre d'inscrits. Tous les inscrits comptent, retirés
// compris : ils ont payé, et un forfait n'est pas un remboursement.
func Pool(entryFee float64, entrants int) float64 { return entryFee * float64(entrants) }

// Distributable : ce qui reste du pool après la retenue, jamais négatif.
func Distributable(pool float64, r Retention) float64 {
	reste := pool - pool*r.Percent/100 - r.Amount
	if reste < 0 {
		return 0
	}
	return reste
}

// PlaceAmounts : les montants place par place d'une dotation, sur le pool distribuable.
//
// Une place en pourcentage se paie à l'unité — personne ne paie en centimes à une table de
// tournoi — par la méthode du plus fort reste : chaque montant exact est tronqué à l'unité, puis
// les unités qui manquent pour atteindre le pool distribuable arrondi vont, une chacune, aux
// places dont la partie fractionnaire est la plus grande ; à égalité, à la mieux classée. La
// somme distribuée égale ainsi exactement le pool distribuable arrondi, aucun montant n'est
// négatif, et un barème décroissant donne des montants décroissants : une place ne touche
// jamais moins qu'une place moins bien classée dont le pourcentage n'est pas plus grand.
// Des montants fixes ne sont ni arrondis ni ajustés. Sans l'un ni l'autre : nil.
func PlaceAmounts(scale PrizeScale, distributable float64) []float64 {
	if len(scale.Amounts) > 0 {
		return append([]float64(nil), scale.Amounts...)
	}
	if len(scale.Percents) == 0 {
		return nil
	}
	n := len(scale.Percents)
	out := make([]float64, n)
	reste := make([]float64, n)
	somme, exact := 0.0, 0.0
	for i, pct := range scale.Percents {
		// Le bruit binaire (0,29 × 100 = 28,999…) ne doit pas faire perdre une unité à la
		// troncature : la part exacte est d'abord ramenée au millionième.
		v := math.Round(distributable*pct/100*1e6) / 1e6
		exact += v
		out[i] = math.Floor(v)
		reste[i] = v - out[i]
		somme += out[i]
	}
	ordre := make([]int, n)
	for i := range ordre {
		ordre[i] = i
	}
	sort.SliceStable(ordre, func(a, b int) bool { return reste[ordre[a]] > reste[ordre[b]] })
	// Σ tronqués ≤ Σ exacts < Σ tronqués + n : il manque au plus n unités, une par place.
	for k := 0; k < int(math.Round(exact)-somme) && k < n; k++ {
		out[ordre[k]]++
	}
	return out
}

// SharePrizes répartit des montants par place sur un classement avec ex æquo : les joueurs
// classés à la même place se partagent les prix des places qu'ils occupent. Un joueur classé
// au-delà des places dotées touche 0, comme un rang nul ou négatif, qui n'est pas une place du
// barème.
func SharePrizes(ranking []Placement, amounts []float64) map[Ref]float64 {
	out := map[Ref]float64{}
	byRank := map[int][]Ref{}
	for _, r := range ranking {
		byRank[r.Rank] = append(byRank[r.Rank], r.Player)
	}
	for rank, ids := range byRank {
		total := 0.0
		for i := rank - 1; rank >= 1 && i < rank-1+len(ids); i++ {
			if i < len(amounts) {
				total += amounts[i]
			}
		}
		for _, p := range ids {
			out[p] = total / float64(len(ids))
		}
	}
	return out
}
