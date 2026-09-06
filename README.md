# tournoi — moteur de tournoi de backgammon (Go)

Bibliothèque Go sans dépendance, à intégrer dans un logiciel web dans le même processus.
Journal d'événements, état reconstruit par `Replay`, propositions d'actions pour le directeur de
tournoi, aucun départage. Exemple d'intégration dans `doc.go`. Les choix de conception viennent
d'une étude par simulation des formats résumée dans `docs/etude_formats.md` ; le travail restant
est dans `docs/RESTE_A_FAIRE.md`.

```bash
go get github.com/PileOfCells/backgammon-tournoi
```

## Formats

| Kind | Description |
|---|---|
| `swiss_lives` | Suisse à L vies, appariement dans le groupe de même nombre de défaites, continu (dès que deux joueurs sont libres) ou par rondes ; option `target` : arrêt quand la somme des vies vaut une puissance de 2 pour basculer sur un tableau |
| `lives_bracket` | Tableau à élimination simple où un joueur à 2 vies est exempt du premier tour |
| `gsl` | Blocs de groupes GSL de 4 (3, 2) pour les joueurs à 0 défaite, mini-tableaux pour ceux à 1 défaite ; option `target` |
| `bracket` | Élimination simple ; `consolation` (consolante progressive), `last_chance`, `reconciliation` + `recharge` (double élimination vraie) |
| `round_robin` | Poules (table de Berger), `qualifiers` par poule, égalités réglées par barrage à 2 vies |

Les phases s'enchaînent (`entry` : `survivors` avec leurs vies, `all`, `top:N`).

## Fichiers

- `types.go`, `events.go`, `state.go`, `engine.go` : modèle, journal, état, propositions.
- `graph.go` : graphes de matchs (tableaux, GSL, poules, consolantes) remplis par les résultats.
- `phase_swiss.go`, `phase_bracket.go`, `phase_gsl.go`, `phase_rr.go` : les formats.
- `standings.go` (rangs, prix partagés, CSV), `clock.go` (durées, matchs lents).
- `render/` : SVG des arbres, tableau des vies, matchs en cours, classement, page complète.
- `sim/` : simulation (tests d'invariants, comparaison avec le simulateur de l'étude, prévision de fin).
- `players/` : import CSV.
- `cmd/tournoi-demo` : démonstration (`go run ./cmd/tournoi-demo -format suisse_tableau -joueurs 32 -sortie demo`, `-rejouer demo/journal.json`).

## Tests

```bash
go test ./...                    # invariants sur 11 formats × 8 effectifs × 8 graines, rejeu, corrections
go test -run TestParity -v ./    # P(meilleur gagne), matchs et durées sur 64 joueurs (long)
```
