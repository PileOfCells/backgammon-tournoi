package tournoi

import "github.com/PileOfCells/backgammon-tournoi/rules"

// Têtes de série, en option et éteintes par défaut.
//
// L'étude par simulation (docs/etude_formats.md) conclut « pas de têtes de série protégées » :
// c'est la culture actuelle du backgammon, où le tirage intégral est ce que les joueurs
// attendent, et c'est pourquoi PhaseConfig.Seeding est VIDE par défaut. Le défaut est un choix
// de conception, pas un oubli. Certains organisateurs en veulent quand même — un open à cent
// joueurs dont les quatre meilleurs se croisent au premier tour laisse un goût amer — et
// l'option est là pour eux.
//
// Le placement est rules.DrawSlots (et rules.SeedOrder). Le tirage reste matérialisé dans
// EvDraw : un journal ancien se rejoue sans dépendre de l'algorithme, et cette option peut donc
// changer sans casser un journal existant.

// SeedingRating : placement classique par cote d'entrée (1 contre 16, 2 contre 15…).
const SeedingRating = rules.SeedingRating
