package tournoi

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// La dotation d'un tournoi réel.
//
// `Prizes` était une simple liste de montants par place, répartie sur un classement général
// unique. Un tournoi réel a un droit d'entrée, une retenue d'organisation, des pourcentages —
// une affiche annonce « 50/30/20 % » et non « 216/130/86 € », puisque le pool dépend du nombre
// d'inscrits — et une dotation PAR SECTION : la consolante a ses propres prix, et son propre
// classement.
//
// Ce qui est calculé ici s'arrête au montant par PLACE. Le partage entre ex æquo reste dans
// Prizes (standings.go) : deux demi-finalistes classés troisièmes se partagent les prix des 3ᵉ
// et 4ᵉ places, et cette règle-là ne dépend ni de la section ni du pool.

// PrizeSectionAll est la clé de la dotation du classement GÉNÉRAL, celui que Ranking renvoie.
// Ce n'est pas un nom de section — aucune section ne s'appelle ainsi — mais la place réservée,
// dans la même carte, à la dotation d'un tournoi qui n'a pas de tableau (un suisse, par exemple).
const PrizeSectionAll = "all"

// PrizeScale est la dotation d'une section : des pourcentages du pool distribuable, ou des
// montants fixes. Les deux ensemble n'ont pas de sens ; Amounts l'emporte, et Validate le dit.
type PrizeScale struct {
	Percents []float64 `json:"percents,omitempty"` // % du pool distribuable, place par place
	Amounts  []float64 `json:"amounts,omitempty"`  // montants fixes, place par place
}

// Retention est ce que l'organisation garde avant de distribuer : un montant, un pourcentage du
// pool, ou les deux (le pourcentage s'applique au pool brut, puis le montant s'en retranche).
type Retention struct {
	Amount  float64 `json:"amount,omitempty"`
	Percent float64 `json:"percent,omitempty"`
}

// PrizePool décrit la dotation entière d'un tournoi.
type PrizePool struct {
	EntryFee  float64               `json:"entry_fee,omitempty"` // droit d'entrée par inscrit
	Retention Retention             `json:"retention,omitempty"`
	Sections  map[string]PrizeScale `json:"sections,omitempty"` // clé : PrizeSectionAll, "main", "conso"…
}

// UnmarshalJSON accepte les deux formes : la liste de montants des configurations d'avant les
// sections (`"prizes": [100, 60, 40]`), qui devient la dotation du classement général, et la
// structure. Un journal existant se relit donc sans être réécrit.
func (p *PrizePool) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		var v []float64
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*p = PrizePool{}
		if len(v) > 0 {
			p.Sections = map[string]PrizeScale{PrizeSectionAll: {Amounts: v}}
		}
		return nil
	}
	type brut PrizePool // évite la récursion
	var v brut
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = PrizePool(v)
	return nil
}

// Empty : aucune dotation configurée.
func (p PrizePool) Empty() bool { return len(p.Sections) == 0 && p.EntryFee == 0 }

// SectionNames : les sections dotées, dans un ordre stable (le classement général d'abord).
func (p PrizePool) SectionNames() []string {
	var out []string
	for k := range p.Sections {
		if k != PrizeSectionAll {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	if _, ok := p.Sections[PrizeSectionAll]; ok {
		out = append([]string{PrizeSectionAll}, out...)
	}
	return out
}

// clone : copie profonde (voir Config.clone).
func (p PrizePool) clone() PrizePool {
	out := p
	if p.Sections != nil {
		out.Sections = make(map[string]PrizeScale, len(p.Sections))
		for k, v := range p.Sections {
			out.Sections[k] = PrizeScale{
				Percents: append([]float64(nil), v.Percents...),
				Amounts:  append([]float64(nil), v.Amounts...),
			}
		}
	}
	return out
}

// Pool : le pool brut, droit d'entrée fois nombre d'inscrits.
//
// Tous les inscrits comptent, y compris ceux qui se sont retirés : ils ont payé. Un forfait
// n'est pas un remboursement, et le moteur n'a pas à en décider.
func (s *State) Pool() float64 { return s.Config.Prizes.EntryFee * float64(len(s.Order)) }

// Distributable : ce qui reste à distribuer après la retenue d'organisation.
func (s *State) Distributable() float64 {
	pool := s.Pool()
	reste := pool - pool*s.Config.Prizes.Retention.Percent/100 - s.Config.Prizes.Retention.Amount
	if reste < 0 {
		return 0
	}
	return reste
}

// PrizeAmounts : les montants place par place d'une section, arrondis à l'unité.
//
// Les pourcentages d'une affiche ne tombent presque jamais juste : 50/30/20 % de 432 € font
// 216, 129,60 et 86,40. Chaque place est arrondie à l'unité — personne ne paie en centimes à
// une table de tournoi — et le RESTE va au premier, en plus ou en moins. Le premier prix est
// celui où un euro de trop ou de trop peu se voit le moins, et c'est la seule façon de garantir
// que la somme distribuée égale exactement le pool après retenue.
//
// Des montants fixes (Amounts) ne sont ni arrondis ni ajustés : ils sont ce que l'organisateur
// a écrit.
func (s *State) PrizeAmounts(section string) []float64 {
	sc, ok := s.Config.Prizes.Sections[section]
	if !ok {
		return nil
	}
	if len(sc.Amounts) > 0 {
		return append([]float64(nil), sc.Amounts...)
	}
	if len(sc.Percents) == 0 {
		return nil
	}
	total := s.Distributable()
	out := make([]float64, len(sc.Percents))
	somme, exact := 0.0, 0.0
	for i, pct := range sc.Percents {
		exact += total * pct / 100
		out[i] = math.Round(total * pct / 100)
		somme += out[i]
	}
	out[0] += math.Round(exact) - somme
	return out
}

// SectionPrizes : ce que chaque joueur d'une section touche, ex æquo partagés.
func (s *State) SectionPrizes(section string) map[PlayerID]float64 {
	ranking := s.Ranking()
	if section != PrizeSectionAll {
		ranking = s.SectionRanking(section)
	} else if s.Final != nil {
		ranking = s.Final
	}
	return Prizes(ranking, s.PrizeAmounts(section))
}

// validate vérifie la dotation et dit ce qui cloche. Une dotation fausse ne se voit qu'au moment
// de payer, devant les joueurs : mieux vaut la refuser à la configuration.
func (p PrizePool) validate() error {
	if p.EntryFee < 0 {
		return fmt.Errorf("dotation : droit d'entrée négatif (%.2f)", p.EntryFee)
	}
	if p.Retention.Percent < 0 || p.Retention.Percent > 100 {
		return fmt.Errorf("dotation : retenue de %.1f %% (attendu entre 0 et 100)", p.Retention.Percent)
	}
	if p.Retention.Amount < 0 {
		return fmt.Errorf("dotation : retenue négative (%.2f)", p.Retention.Amount)
	}
	total := 0.0
	for _, name := range p.SectionNames() {
		sc := p.Sections[name]
		if len(sc.Percents) > 0 && len(sc.Amounts) > 0 {
			return fmt.Errorf("dotation de la section %q : des pourcentages ET des montants ; il faut choisir", name)
		}
		for i, v := range sc.Percents {
			if v < 0 {
				return fmt.Errorf("dotation de la section %q : pourcentage négatif à la place %d", name, i+1)
			}
			total += v
		}
		for i, v := range sc.Amounts {
			if v < 0 {
				return fmt.Errorf("dotation de la section %q : montant négatif à la place %d", name, i+1)
			}
		}
	}
	if total > 100.0001 {
		return fmt.Errorf("dotation : les pourcentages distribuent %.1f %% du pool ; il n'y en a que 100", total)
	}
	return nil
}
