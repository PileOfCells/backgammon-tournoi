package tournoi

import "sort"

// Têtes de série, en option et éteintes par défaut.
//
// L'étude par simulation (docs/etude_formats.md) conclut « pas de têtes de série protégées » :
// c'est la culture actuelle du backgammon, où le tirage intégral est ce que les joueurs
// attendent, et c'est pourquoi PhaseConfig.Seeding est VIDE par défaut. Le défaut est un choix
// de conception, pas un oubli. Certains organisateurs en veulent quand même — un open à cent
// joueurs dont les quatre meilleurs se croisent au premier tour laisse un goût amer — et
// l'option est là pour eux.
//
// Le tirage reste matérialisé dans EvDraw : un journal ancien se rejoue sans dépendre de
// l'algorithme, et cette option peut donc changer sans casser un journal existant.

// SeedingRating : placement classique par cote d'entrée (1 contre 16, 2 contre 15…).
const SeedingRating = "rating"

// seedOrder donne, pour chaque place d'un tableau de size places, le NUMÉRO de tête de série qui
// s'y installe. C'est la construction classique, par doublements successifs : chaque place x d'un
// tableau de n devient les deux places x et 2n+1-x du tableau de 2n. Elle a la propriété
// recherchée — les têtes 1 et 2 sont dans deux moitiés différentes, 1 à 4 dans quatre quarts
// différents, et ainsi de suite jusqu'au premier tour.
func seedOrder(size int) []int {
	order := []int{1}
	for n := 1; n < size; n *= 2 {
		next := make([]int, 0, 2*n)
		for _, x := range order {
			next = append(next, x, 2*n+1-x)
		}
		order = next
	}
	return order
}

// seededSlots place les joueurs sur un tableau de size places selon leur cote.
//
// L'ordre des têtes de série n'est pas seulement l'ordre des cotes : les joueurs à deux vies
// passent devant TOUS les autres, quelle que soit leur cote. Ils l'ont gagné à la phase
// précédente, et c'est aussi ce que la structure du tableau exige — dans le placement classique,
// la tête j rencontre au premier tour la tête size+1-j, donc les exemptions vont aux premières
// têtes. Leur donner les premiers numéros est la seule façon de les rendre exempts sans défaire
// le placement.
func seededSlots(deux, une []PlayerID, rating func(PlayerID) float64, size int) []PlayerID {
	ordre := append(byStrength(deux, rating), byStrength(une, rating)...)
	slots := make([]PlayerID, size)
	for i := range slots {
		slots[i] = BYE
	}
	for place, tête := range seedOrder(size) {
		if tête <= len(ordre) {
			slots[place] = ordre[tête-1]
		}
	}
	return slots
}

// byStrength trie du meilleur au moins bon. La cote est un PR : plus bas vaut mieux.
//
// Une cote inconnue vaut 0, qui est la cote d'un joueur parfait : la prendre au pied de la
// lettre ferait du nouvel inscrit sans cote la tête de série numéro un. Les cotes inconnues
// passent donc DERRIÈRE tout le monde, dans l'ordre de leurs identifiants — de façon
// déterministe, puisqu'un tirage doit se rejouer.
func byStrength(g []PlayerID, rating func(PlayerID) float64) []PlayerID {
	out := sortedIDs(g)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rating(out[i]), rating(out[j])
		inconnuI, inconnuJ := ri <= 0, rj <= 0
		if inconnuI != inconnuJ {
			return inconnuJ
		}
		if inconnuI {
			return false // deux inconnues : l'ordre des identifiants, déjà en place
		}
		return ri < rj
	})
	return out
}

// rating : la cote d'un joueur, 0 si elle est inconnue ou si le joueur n'existe pas.
func (s *State) rating(p PlayerID) float64 {
	if pl := s.Players[p]; pl != nil {
		return pl.Rating
	}
	return 0
}
