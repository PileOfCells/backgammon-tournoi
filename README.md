# tournoi — moteur de tournoi de backgammon (Go)

Bibliothèque Go sans dépendance, à intégrer dans un logiciel web dans le même processus.
Journal d'événements, état reconstruit par `Replay`, propositions d'actions pour le directeur de
tournoi, aucun départage. Exemple d'intégration dans `doc.go`. Les choix de conception viennent
d'une étude par simulation des formats résumée dans `docs/etude_formats.md` ; le travail restant
est dans `docs/RESTE_A_FAIRE.md`. Pour une présentation accessible du fonctionnement et des
décisions de conception — utilisable telle quelle pour répondre aux questions des joueurs et des
organisateurs — voir `docs/comprendre_le_moteur.md`. La **spécification complète**, assez détaillée
pour reconstruire le moteur à partir d'elle seule, est dans `docs/specification.md`.
Les deux existent aussi en PDF à côté.

```bash
go get github.com/PileOfCells/backgammon-tournoi
```

## Formats

| Kind | Description |
|---|---|
| `swiss_lives` | Suisse à L vies, appariement dans le groupe de même nombre de défaites, continu (dès que deux joueurs sont libres) ou par rondes ; option `batch_minutes` : micro-rondes, les joueurs libres attendent l'échéance et partent par lots ; option `target` : arrêt quand la somme des vies vaut une puissance de 2 pour basculer sur un tableau |
| `lives_bracket` | Tableau à élimination simple où un joueur à 2 vies est exempt du premier tour |
| `gsl` | Blocs de groupes GSL de 4 (3, 2) pour les joueurs à 0 défaite, mini-tableaux pour ceux à 1 défaite ; option `target` |
| `bracket` | Élimination simple ; `consolation` (consolante progressive), `last_chance`, `reconciliation` + `recharge` (double élimination vraie) |
| `round_robin` | Poules (table de Berger), `qualifiers` par poule, égalités réglées par barrage à 2 vies |

Les phases s'enchaînent (`entry` : `survivors` avec leurs vies, `all`, `top:N`).

Le tirage d'un tableau est intégralement aléatoire par défaut — c'est la conclusion de l'étude,
pas un oubli. `seeding: "rating"` sur une phase de tableau active le placement classique par
cote (1 contre 16, 2 contre 15…) pour l'organisateur qui le veut.

Longueurs de match : `length` partout, `final_length` pour le dernier tour, `lengths` pour une
grille par tour lue **du dernier tour vers le premier** (`[15,13,11,9]`), `length_late` +
`late_threshold` pour allonger la fin d'un suisse.

## Fichiers

- `types.go`, `events.go`, `state.go`, `engine.go` : modèle, journal, état, propositions.
- `codes.go`, `fr.go` : les libellés, notes, avertissements et raisons sont des **codes** ; `fr.go` en donne le rendu français. Un hôte multilingue traduit les codes lui-même.
- `graph.go` : graphes de matchs (tableaux, GSL, poules, consolantes) remplis par les résultats.
- `phase_swiss.go`, `phase_bracket.go`, `phase_gsl.go`, `phase_rr.go` : les formats.
- `standings.go` (rangs, prix partagés, CSV par section), `prizes.go` (droit d'entrée, retenue, barème par section), `clock.go` (durées, matchs lents), `tables.go` (tables indisponibles, réservées).
- `retardataire.go` : places d'exemption libres (`FreeSlots`) et entrée d'un joueur arrivé après le tirage, sans jamais refaire le tirage.
- `reparation.go` : après une correction, `Propose` propose l'annulation en série des matchs devenus incohérents (`cancel_match`) ; rien n'est appliqué d'office.
- `horaires.go` : micro-rondes (`batch_minutes`, échéance dérivée du journal) et pauses programmées (`Config.Breaks`, avertissement sans blocage).
- `seeding.go` : têtes de série en option (`seeding: "rating"`), éteintes par défaut.
- `reconfig.go` : configuration modifiable en cours (`ConfigChangedEvent`, la configuration entière) et réouverture d'un tournoi clos (`ReopenedEvent`).
- `render/` : rendu traduisible (un `Labeler` injecté rend les codes ET les mots du rendu),
  feuille de style et crédit injectés. Arbres SVG de toutes les sections d'une phase côte à côte
  avec les descentes, grille des tables, tableau des vies (adversaires rencontrés, attente),
  matchs en cours, classement, feuille d'appariements imprimable, page autonome (un fichier, CSS
  embarqué, aucune ressource externe). Fichiers témoins dans `render/testdata/`.
- `sim/` : simulation (tests d'invariants, comparaison avec le simulateur de l'étude, prévision de fin).
- `players/` : import et export CSV d'une liste de joueurs (l'annuaire de l'hôte fait l'aller-retour).
- `cmd/tournoi-demo` : démonstration (`go run ./cmd/tournoi-demo -format suisse_tableau -joueurs 32 -etapes -sortie demo`, `-rejouer demo/journal.json`).
- `cmd/tournoi-td` : console interactive pour diriger un tournoi à la main (voir « Tester à la main »).
- `exemples/` : tournois simulés rendus à quatre stades pour sept formats ; ouvrir `exemples/index.html`. Régénérer avec `scripts/exemples.sh`.

## Tests

```bash
go test ./...                    # invariants sur 11 formats × 8 effectifs × 8 graines, rejeu, corrections
go test -run TestParity -v ./    # P(meilleur gagne), matchs et durées sur 64 joueurs (long)
```

## Tester à la main

```bash
go run ./cmd/tournoi-td -journal montournoi.json -format suisse_tableau
```

Dans la console : `ajoute Alice Paris 4.2` (ou `import joueurs.csv`), `propose`, `ok tous`,
`resultat M1 alice 7 3`, `corrige M1 bob`, `forfait chloe`, `configure nouvelle.json`,
`rouvre`, `exporte joueurs.csv`, `matchs`, `vies`, `classement`,
`simule` (joue au hasard les matchs en cours), `quitte`. Le journal est écrit après chaque
commande ; relancer la même commande reprend le tournoi. La page `affichage.html` à côté du
journal est régénérée à chaque commande : ouvrez-la dans un navigateur.
