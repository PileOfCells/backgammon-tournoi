# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Projet

Bibliothèque Go (`tournoi`, module `github.com/PileOfCells/backgammon-tournoi`, Go 1.22, **aucune dépendance externe**) : moteur de tournoi de backgammon destiné à être intégré dans un logiciel hôte, dans le même processus. Code, commentaires, identifiants de labels et documentation sont **en français** ; garder cette convention (noms de fonctions exportées en anglais, commentaires et messages en français).

Le moteur n'a jamais dirigé de vrai tournoi. Les choix de conception (formats, absence de départage, pas de têtes de série) sont justifiés dans `docs/etude_formats.md` ; la liste priorisée du travail restant est dans `docs/RESTE_A_FAIRE.md` : consulter ce fichier avant d'ajouter une fonction, et le mettre à jour quand un point est traité.
`docs/specification.md` (et son PDF) est la spécification complète du moteur : la mettre à jour quand le comportement change. `docs/comprendre_le_moteur.md` est la présentation accessible (décisions de conception, justifications, questions fréquentes, chiffres de référence) : les chiffres du chapitre « Chiffres de référence » viennent de `go test -run TestParity -v ./` et sont à refaire si l'appariement change.

## Commandes

```bash
go build ./...
go test ./...                    # tests courts ≈ 3 s : invariants (11 formats × 8 effectifs × 8 graines), rejeu, corrections
go test -run TestFormatsInvariants ./          # un seul test
go test -run TestParity -v ./    # long (1500 tournois de 64 joueurs par format) ; sauté avec -short
go vet ./... && gofmt -l .

# Démo : simule un tournoi et écrit journal.json, affichage.html, classement.csv, SVG dans un dossier
go run ./cmd/tournoi-demo -format suisse_tableau -joueurs 32 -etapes -sortie demo
go run ./cmd/tournoi-demo -rejouer demo/journal.json          # rejoue un journal, affiche les actions proposées

# Console TD interactive (journal réécrit après chaque commande, affichage.html régénéré à côté)
go run ./cmd/tournoi-td -journal montournoi.json -format suisse_tableau
```

`demo/` est ignoré par git. `exemples/` contient les sorties de `tournoi-demo -etapes` pour sept formats (galerie `exemples/index.html`) ; les régénérer avec la démo si le rendu ou les formats changent.

## Architecture

### Journal d'événements, état dérivé

Tout passe par un **journal** (`Journal`, tableau JSON d'`Event`) que l'hôte stocke ; le moteur ne persiste rien. `Replay(journal)` reconstruit l'état complet ; `State.Apply(ev)` applique un événement. **On ne modifie jamais le journal** : une correction est un événement `EvResultCorrected`, un forfait `EvPlayerWithdrawn`, un match lancé par erreur `EvMatchCancelled`. Ces trois événements déclenchent `recompute()` (state.go) qui rejoue toute la comptabilité des phases à partir des matchs et remplit `State.Warnings` (par exemple un match de tableau joué par le mauvais joueur).

Boucle côté hôte (exemple complet dans `doc.go`) :

1. `Propose()` renvoie des `Action` (lancer un match, bye, tirage, phase suivante, clore, attendre).
2. Le TD confirme ; `EventFromAction` produit l'`Event`, `Apply` l'applique, l'hôte l'ajoute au journal.
3. Résultats via `ResultEvent(...)`.

### Déterminisme

`Propose` ne dépend que du journal : le générateur aléatoire est `s.rng()` = f(graine, nombre d'événements, phase courante). Les **tirages** (placement dans un tableau, composition des groupes) sont matérialisés dans l'événement `EvDraw` (`Draw{Slots, Groups, Lives}`), si bien que rejouer un journal ne dépend pas de l'algorithme d'appariement : on peut changer `drawSlots`, `pairGroup`, etc. sans casser les journaux existants. Les tests vérifient qu'un rejeu donne le même classement final et aucun `Warning`.

Toute itération sur une map dont l'ordre influence une proposition doit passer par `sortedIDs` ou un tri stable.

### Phases et formats

Un tournoi est une suite de `PhaseConfig` (`Config.Validate` remplit les défauts). `engine.go` dispatche `Propose`/`phaseDone` par `Kind` vers un fichier par format :

| Kind | Fichier | Mécanisme |
|---|---|---|
| `swiss_lives` | `phase_swiss.go` | appariement dynamique dans le groupe de même nombre de défaites (continu ou par rondes), pas de graphe ; `Target` fige quand Σvies ≤ 2^k |
| `gsl` | `phase_gsl.go` | blocs de groupes GSL de 4/3/2 (sections `gsl`) |
| `bracket`, `lives_bracket` | `phase_bracket.go` | tirage puis sections `main`, `conso`, `last`, `gf` |
| `round_robin` | `phase_rr.go` | poules (table de Berger), puis sections `barrage` à appariement dynamique en cas d'égalité |

Le passage de phase (`enterFrom`) admet les survivants **avec leurs vies restantes** quand la phase suivante est à vies (`lives_bracket` : 2 vies = exempt du premier tour), sinon `entry` vaut `all` ou `top:N`.

### Graphes de matchs (`graph.go`)

Tableaux, GSL, poules et consolantes sont des `Section` : listes de `GMatch` dont chaque place (`Src`) est soit un joueur fixé, soit le vainqueur/perdant d'un autre match (même section ou autre section, ex. les perdants du principal alimentent la consolante). `resolve()` propage les résultats, les exemptions (`BYE`) et les matchs conditionnels (recharge en double élimination) jusqu'au point fixe ; `ready()`/`readyFree()` donnent les matchs lançables. Les constructeurs (`bracketSection`, `consolationSection`, `gslSection`, `bracketFromSrcs`) ne font que poser des `Src` ; ajouter un format de graphe revient à écrire un constructeur.

Un `Match` (state.go, joué, avec table et horodatages) est relié à son `GMatch` par `(Section, Key)` ; `EvMatchStarted` porte ces deux champs.

### Classement et prix

`Ranking()` concatène les classements de phases de la dernière à la première (`phaseRanking` par format), puis renumérote avec ex æquo conservés. Pas de départage par principe : les égalités restent des ex æquo ou se règlent par barrage. `standings.go` partage les prix entre ex æquo et exporte le CSV.

### Paquets annexes

- `sim/` : `Run(cfg, players, Options)` joue un tournoi complet avec résultats tirés selon les PR (`PGain`, modèle Elo/FIBS) ; `Options.Hook` est appelé après chaque événement (c'est ainsi que les tests vérifient les invariants) ; `Forecast` prévoit la fin d'un tournoi en cours. Les tests du paquet racine (`sim_test.go`, `parity_test.go`) sont dans `tournoi_test` et reposent entièrement sur `sim`.
- `render/` : HTML/SVG bruts sans CSS (`BracketSVG`, `LivesBoard`, `RunningTable`, `ActionsList`, `StandingsTable`, `Page`). Pas de tests.
- `players/` : import CSV (séparateur détecté, identifiants en slug).
- `cmd/tournoi-demo`, `cmd/tournoi-td` : chacun a sa propre map `formats` de configurations nommées ; les garder cohérentes si on en ajoute une.

## Conventions de test

Les tests sont des tests d'invariants par simulation, pas des tests unitaires : un nouveau format ou une nouvelle option s'ajoute dans `configs()` de `sim_test.go` pour être couvert automatiquement (vainqueur unique, classement complet, rejeu identique, aucun match entre joueurs de groupes différents, peu de rematchs). Un changement d'algorithme d'appariement se valide en plus avec `TestParity` (P(meilleur gagne), nombre de matchs, durée) comparé aux valeurs de `docs/etude_formats.md`.
