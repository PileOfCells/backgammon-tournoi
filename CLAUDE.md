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

`demo/` est ignoré par git. `exemples/` contient les sorties de `tournoi-demo -etapes` pour sept formats (galerie `exemples/index.html`) ; les régénérer avec `scripts/exemples.sh` si le rendu ou les formats changent — le script refait aussi la galerie, dont les liens dépendent des noms de fichiers produits.

## Architecture

### Journal d'événements, état dérivé

Tout passe par un **journal** (`Journal`, tableau JSON d'`Event`) que l'hôte stocke ; le moteur ne persiste rien. `Replay(journal)` reconstruit l'état complet ; `State.Apply(ev)` applique un événement. **On ne modifie jamais le journal** : une correction est un événement `EvResultCorrected`, un forfait `EvPlayerWithdrawn`, un match lancé par erreur `EvMatchCancelled`, un changement de configuration `EvConfigChanged` (la configuration ENTIÈRE, voir `reconfig.go`), une clôture annulée `EvReopened`. Ces trois événements déclenchent `recompute()` (state.go) qui rejoue toute la comptabilité des phases à partir des matchs et remplit `State.Warnings` (par exemple un match de tableau joué par le mauvais joueur). `check()` est aussi rappelée sur `match_started` et `result`, sans quoi un avertissement n'apparaîtrait qu'après une correction sans rapport. Quand un graphe est désaccordé, `reparation.go` PROPOSE l'annulation en série des matchs incohérents (`ActCancelMatch`) — rien n'est appliqué d'office.

Boucle côté hôte (exemple complet dans `doc.go`) :

1. `Propose()` renvoie des `Action` (lancer un match, bye, tirage, phase suivante, clore, attendre). `ProposeAt(now)` est la même chose à une heure donnée : le moteur n'a pas d'horloge, et seul ce qui dépend du temps (micro-rondes, pauses) change avec `now`. `Propose()` prend l'horodatage du dernier événement du journal.
2. Le TD confirme ; `EventFromAction` produit l'`Event`, `Apply` l'applique, l'hôte l'ajoute au journal.
3. Résultats via `ResultEvent(...)`.

### Codes, pas de phrases

Rien de ce qui sort du moteur n'est destiné à être affiché tel quel : les libellés de match
(`Label`), les notes de classement (`Note`), les avertissements (`Warning`) et les raisons
d'attente (`ReasonCode`) sont des **codes structurés** (`codes.go`), parce que le logiciel hôte
affiche le tournoi dans la langue de son utilisateur — blunderDB en parle neuf. Le rendu
français vit dans `fr.go` et ne sert que la console, la démo et `render/`.

Les noms de section (`Section.Name`, `Match.Section`) sont des **identifiants** (`main`,
`conso`, `poule:A`, `barrage:A`), pas des libellés : ils voyagent dans le journal et dans la
base de l'hôte. Un test (`codes_test.go`) refuse toute chaîne accentuée ou à espaces sortant du
moteur, hors les champs que le TD a lui-même saisis (nom de joueur, de club, de phase, note).

Le numéro de ronde est le champ `Event.Round` : avant, il était relu dans le texte « Ronde k ».
Chaque événement porte `Version` (`JournalVersion`) ; les constructeurs de `events.go` la
posent, et un journal `Version: 0` est converti à la lecture par `Event.upgraded()`.

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

Le tirage d'un tableau (`drawSlots`) est aléatoire par défaut ; `PhaseConfig.Seeding = "rating"` le remplace par le placement classique par cote (`seeding.go`). Le défaut vide est un choix de conception justifié dans `docs/etude_formats.md`, pas un oubli : ne pas l'inverser.

Le passage de phase (`enterFrom`) admet les survivants **avec leurs vies restantes** quand la phase suivante est à vies (`lives_bracket` : 2 vies = exempt du premier tour), sinon `entry` vaut `all` ou `top:N`.

### Graphes de matchs (`graph.go`)

Tableaux, GSL, poules et consolantes sont des `Section` : listes de `GMatch` dont chaque place (`Src`) est soit un joueur fixé, soit le vainqueur/perdant d'un autre match (même section ou autre section, ex. les perdants du principal alimentent la consolante). `resolve()` propage les résultats, les exemptions (`BYE`) et les matchs conditionnels (recharge en double élimination) jusqu'au point fixe ; `ready()`/`readyFree()` donnent les matchs lançables. Les constructeurs (`bracketSection`, `consolationSection`, `gslSection`, `bracketFromSrcs`) ne font que poser des `Src` ; ajouter un format de graphe revient à écrire un constructeur.

Un `Match` (state.go, joué, avec table et horodatages) est relié à son `GMatch` par `(Section, Key)` ; `EvMatchStarted` porte ces deux champs.

### Classement et prix

`Ranking()` concatène les classements de phases de la dernière à la première (`phaseRanking` par format), puis renumérote avec ex æquo conservés. `SectionRanking(sec)` donne le classement PROPRE d'une section (par tour atteint), sur lequel la dotation de cette section se répartit. Pas de départage par principe : les égalités restent des ex æquo ou se règlent par barrage.

`prizes.go` porte la dotation (`Config.Prizes` est un `PrizePool` : droit d'entrée, retenue, barème par section en pourcentages ou en montants ; arrondi à l'unité, reste au premier). `standings.go` partage les prix entre ex æquo et exporte le CSV, une section par bloc.

### Paquets annexes

- `sim/` : `Run(cfg, players, Options)` joue un tournoi complet avec résultats tirés selon les PR (`PGain`, modèle Elo/FIBS) ; `Options.Hook` est appelé après chaque événement (c'est ainsi que les tests vérifient les invariants) ; `Forecast` prévoit la fin d'un tournoi en cours. Les tests du paquet racine (`sim_test.go`, `parity_test.go`) sont dans `tournoi_test` et reposent entièrement sur `sim`.
- `render/` : rendu traduisible. Tout passe par un `Labeler` injecté — les codes du moteur ET les mots du rendu lui-même (`Term`) ; `French()` est fourni pour la console, la démo et les tests. La feuille de style, le crédit et la langue sont injectés ; les pages sont AUTONOMES (un fichier, CSS embarqué, aucune ressource externe, aucun script). `BracketBoardSVG` (toutes les sections d'une phase côte à côte, avec les descentes), `LivesBoard`, `TableGrid`, `RunningTable`, `ActionsList`, `StandingsTable`, `PairingSheet`, `Page`. Fichiers témoins dans `render/testdata/` : `go test ./render -update` pour les régénérer après un changement voulu.
- `players/` : import et export CSV (séparateur détecté, identifiants en slug ; `ToCSV` écrit ce que `FromCSV` relit).
- `cmd/tournoi-demo`, `cmd/tournoi-td` : chacun a sa propre map `formats` de configurations nommées ; les garder cohérentes si on en ajoute une.

## Conventions de test

Les tests sont des tests d'invariants par simulation, pas des tests unitaires : un nouveau format ou une nouvelle option s'ajoute dans `configs()` de `sim_test.go` pour être couvert automatiquement (vainqueur unique, classement complet, rejeu identique, aucun match entre joueurs de groupes différents, peu de rematchs). Un changement d'algorithme d'appariement se valide en plus avec `TestParity` (P(meilleur gagne), nombre de matchs, durée) comparé aux valeurs de `docs/etude_formats.md`.
