// Package rules porte les règles de tournoi en fonctions pures : sans état, sans entrée ni
// sortie, sans horloge — l'instant et la graine sont des arguments — et sans autre dépendance
// que la bibliothèque standard.
//
// Ses types sont les siens : ni State, ni Event, ni identifiant de base de données. Un joueur y
// est une référence opaque (Ref) que l'appelant choisit et retrouve. Le moteur à journal
// (paquet tournoi), sim et render appellent ces fonctions et n'ont pas d'autre implémentation
// des mêmes règles ; un autre hôte (blunderDB, gammonGo) les appelle sur son propre état, qu'il
// garde, et ne stocke aucune de leurs sorties.
//
// Trois règles y vivent aujourd'hui :
//
//   - la DOTATION : pool brut (Pool), pool après retenue (Distributable), montants par place
//     arrondis à l'unité avec le reste au premier (PlaceAmounts), partage entre ex æquo
//     (SharePrizes) ;
//   - la PRÉVISION DE FIN (sauf les matchs restants, qui dépendent des appariements) : rythme observé (PaceAt, ExpectedDuration, IsSlow), temps de jeu
//     (PlayingTime, union des intervalles de match), jour de jeu (DayOfPlay), fin estimée
//     (EstimatedEnd : médiane des fins simulées reportée au-delà des pauses, AfterBreaks), et les
//     pièces du modèle de simulation (WinProbability, MeanRating, RemainingDuration) ; la
//     simulation elle-même rejoue le moteur et reste dans sim.Forecast ;
//   - la FEUILLE D'APPARIEMENTS : les lots d'une phase (Batches) et les lignes d'un lot dans
//     l'ordre imprimé (PairingSheet), sans phrase ni HTML.
//
// Les appariements et le classement (Pair, Rank) viendront au second lot, et avec eux les
// MATCHS RESTANTS de la prévision de fin, qui se comptent en appariant : d'ici là, sim.Forecast
// les obtient en rejouant le moteur. Le libellé d'une ligne de feuille reste aussi chez l'hôte,
// qui le retrouve par l'identifiant du match.
//
// Les fichiers dorés de testdata/ sont du JSON : pour chaque fonction, des entrées et les sorties
// attendues. Les tests de ce paquet les rejouent contre les fonctions pures, ceux du moteur
// contre ses enveloppes. Un client du module les rejoue contre son adaptateur en les décodant
// avec ses propres types : les types Go qui les décrivent ici sont internes au module. Une règle
// dont la sortie change à entrée égale est une version mineure du module.
package rules

// Ref est la référence opaque d'un joueur : l'appelant la choisit et la retrouve.
type Ref string
