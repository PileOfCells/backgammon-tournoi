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
// Cinq règles y vivent :
//
//   - les APPARIEMENTS : Pair(instantané, graine) donne les paires à lancer, leurs longueurs,
//     les exemptions ou le tirage à faire d'abord (Pairing, une proposition, distincte d'un
//     match lancé), pour chaque format — suisse à vies continu ou par rondes (et, pour une ronde
//     ouverte, PairSwissRound, PairSwissOrphans, PairSwissFallback), blocs GSL, tableau à vies
//     et à exemptions, élimination avec consolante, dernière chance et double élimination,
//     poules et barrages. L'instantané (Phase) est construit par l'hôte depuis son état ; la
//     graine (Seed) dit de quoi dépend le hasard. Les graphes de matchs (Section, Node) ont
//     leurs constructeurs (BracketSections, GSLBlockSections, PoolSections…), leur résolution
//     (Resolve) et leurs nœuds prêts (Ready, ReadyFree) ; les tirages, DrawSlots et Partition ;
//     la fin de phase, PhaseDone ;
//   - le CLASSEMENT : Rank (par phase), RankSection (le classement propre d'une section, celui
//     de sa dotation), MergeRankings (le classement général), PoolQualified ; aucun départage,
//     les ex æquo partagent leur rang ; des notes codées (Note), jamais des phrases ;
//   - la DOTATION : pool brut (Pool), pool après retenue (Distributable), montants par place
//     à l'unité par la méthode du plus fort reste (PlaceAmounts), partage entre ex æquo
//     (SharePrizes) ;
//   - la PRÉVISION DE FIN : matchs restants de la phase en cours (RemainingMatches, un
//     encadrement), rythme observé (PaceAt, ExpectedDuration, IsSlow), temps de jeu
//     (PlayingTime, union des intervalles de match), jour de jeu (DayOfPlay), fin estimée
//     (EstimatedEnd : médiane des fins simulées reportée au-delà des pauses, AfterBreaks), et les
//     pièces du modèle de simulation (WinProbability, MeanRating, RemainingDuration) ; la
//     simulation elle-même rejoue le moteur et reste dans sim.Forecast ;
//   - la FEUILLE D'APPARIEMENTS : les lots d'une phase (Batches) et les lignes d'un lot dans
//     l'ordre imprimé (PairingSheet), chaque ligne avec son libellé (Match.Label), sans phrase
//     ni HTML.
//
// Ce qui dépend de l'histoire d'un tournoi et non d'une règle reste à l'hôte : les joueurs
// appelés et déjà engagés d'une ronde ouverte, les absences, les tables, les pauses, la
// réparation d'un graphe après correction, la proposition d'un repêchage.
//
// Les fichiers dorés de testdata/ sont du JSON : pour chaque fonction, des entrées et les sorties
// attendues. Les tests de ce paquet les rejouent contre les fonctions pures, ceux du moteur
// contre ses enveloppes. Un client du module les rejoue contre son adaptateur en les décodant
// avec ses propres types : les types Go qui les décrivent ici sont internes au module. Une règle
// dont la sortie change à entrée égale est une version mineure du module.
package rules

// Ref est la référence opaque d'un joueur : l'appelant la choisit et la retrouve.
type Ref string
