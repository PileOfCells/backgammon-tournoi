package rules

import "math"

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
// Chaque place en pourcentage est arrondie à l'unité — personne ne paie en centimes à une table
// de tournoi — et le RESTE va au premier, en plus ou en moins, si bien que la somme distribuée
// égale exactement le pool distribuable arrondi. Des montants fixes ne sont ni arrondis ni
// ajustés. Sans l'un ni l'autre : nil.
func PlaceAmounts(scale PrizeScale, distributable float64) []float64 {
	if len(scale.Amounts) > 0 {
		return append([]float64(nil), scale.Amounts...)
	}
	if len(scale.Percents) == 0 {
		return nil
	}
	out := make([]float64, len(scale.Percents))
	somme, exact := 0.0, 0.0
	for i, pct := range scale.Percents {
		exact += distributable * pct / 100
		out[i] = math.Round(distributable * pct / 100)
		somme += out[i]
	}
	out[0] += math.Round(exact) - somme
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
