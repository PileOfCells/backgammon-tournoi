# Reste à faire

État au 6 septembre 2026. Le moteur Go est fonctionnel et testé par simulation ;
il n'a jamais dirigé un vrai tournoi. Les points sont classés par priorité ; chaque entrée dit
où intervenir et comment vérifier.

## 1. Avant un premier tournoi réel (indispensable)

- [ ] **Interface TD dans le logiciel hôte.** Le moteur ne fournit que `Propose` / `Apply`.
  Il faut une page qui liste les actions proposées avec un bouton de confirmation par action,
  une saisie de résultat par match en cours (vainqueur, score), et les boutons correction,
  annulation, forfait. Contrat : chaque bouton produit exactement un `Event` ajouté au journal
  (`state.EventFromAction`, `tournoi.ResultEvent`). Vérification : rejouer le journal donne le
  même état (`Replay`), aucune `Warning`.
- [ ] **Persistance du journal côté hôte** (une ligne JSON par événement, en append). Le moteur
  ne stocke rien. Reprise après panne = `Replay`. Tester une coupure en plein tournoi.
- [ ] **Validation manuelle sur un tournoi de club** (16 à 32 joueurs) avec le format
  `swiss_lives` continu + `lives_bracket`, en doublant sur papier. Noter les cas où le TD a
  voulu faire autre chose que la proposition du moteur : ce sont les fonctions manquantes.
- [ ] **Règle anti-manipulation du continu** (choix de conception, voir `docs/etude_formats.md`) :
  soit micro-rondes (les appariements sont tirés toutes les X minutes parmi les joueurs libres,
  pas au fil de l'eau), soit blocs GSL. Aujourd'hui `Propose` apparie tous les joueurs libres à
  chaque appel : l'hôte doit l'appeler à intervalle fixe pour obtenir l'effet micro-rondes.
  À implémenter dans `phase_swiss.go` : paramètre `batch_minutes` et horodatage du dernier lot.
- [x] **Tables.** `Config.Tables` porte les indisponibles et les réservations (`tables.go`) ;
  `assignTables` les saute et marque `ReasonWaitingTable` quand aucune table n'est libre ;
  `table_changed` déplace un match en cours.

## 2. Fonctions attendues d'un logiciel de tournoi

- [x] **Retardataires** (issue #6). `State.FreeSlots` énumère les places d'exemption libres d'un
  tour non commencé ; `PlayerAddedAtSlotEvent` y installe le joueur sans jamais refaire le
  tirage. Sans place, il est enregistré et `State.Infos` porte le code `enters_at` (ou
  `no_entry`) — dérivé de l'état, jamais accumulé. Le correctif de fond est dans `recompute`,
  qui réinitialise désormais les places dérivées de `GMatch.Players`. Les forfaits fins étaient
  déjà là : forfait d'un seul match (`ForfeitEvent`), retrait différé
  (`PlayerWithdrawnAfterCurrentEvent`).
- [x] **Longueurs de match par tour** (issue #5). `PhaseConfig.Lengths` se lit du dernier tour
  vers le premier (`[15,13,11,9]`) et passe par `lengthPlan` (graph.go), qui superpose la liste,
  `FinalLength` et `Length`. La fin d'un suisse s'allonge avec `LengthLate` + `LateThreshold`
  (`swissLength`, phase_swiss.go) ; un `length_changed` du TD l'emporte.
- [ ] **Pauses programmées** (repas) : `Propose` ne doit pas lancer de match dont la fin
  attendue dépasse l'heure de la pause, ou doit l'indiquer. Pour les blocs GSL, caler un bloc par
  créneau. Paramètre `Config.Breaks []TimeRange`, prise en compte dans `engine.go`.
- [ ] **Byes et exemptions équitables sur plusieurs rondes** : en mode `rounds`, le bye va au
  joueur du groupe le plus bas n'en ayant pas eu ; vérifier la règle « pas de second bye tant que
  d'autres n'en ont pas eu » entre groupes différents (aujourd'hui par groupe). `pairGroup`.
- [ ] **Consolante et dernière chance : options de saut** (les perdants du tour 1 seulement, ou
  jusqu'au tour k ; consolante non progressive « à tirage » avec une heure limite d'entrée).
  Les constructeurs `consolationSection` / `bracketFromSrcs` prennent des sources : ajouter un
  filtre par tour d'origine.
- [ ] **Têtes de série optionnelles** dans les tableaux (écartées en v1 par choix ; à offrir
  comme option `seeding: "rating"` dans `drawSlots`, avec placement classique 1 vs 16…).
- [ ] **Classement des places non gagnantes** : règles à valider avec la FFBG (par tour atteint
  dans les tableaux, par victoires à l'élimination dans les suisses, ex æquo partagés). Les
  notes de classement sont désormais des **codes** (`Note`, `codes.go`), traduisibles par
  l'hôte ; restent les règles de classement elles-mêmes, à valider avec la FFBG.
- [ ] **Prix** : `Prizes` gère des montants par place ; ajouter les structures en pourcentage
  du pool, la retenue d'organisation, l'arrondi, et les prix séparés par section (consolante,
  dernière chance) : aujourd'hui un seul classement général.
- [ ] **Export / import** : classement CSV existe ; ajouter l'export du journal en PDF ou texte
  lisible (feuille d'appariements imprimable par ronde ou par bloc), et l'import CSV avec
  ratings FFBG (colonnes à confirmer). `players/csv.go`.

## 3. Rendu et affichage

- [ ] Style et charte du site hôte pour `render/` (aujourd'hui SVG et HTML bruts sans CSS).
- [ ] Arbre de double élimination : dessiner principal et consolante côte à côte avec les drops.
- [ ] Tableau des vies : afficher les adversaires déjà rencontrés et le temps d'attente.
- [ ] Écran joueur (téléphone) : « votre prochain match » et « votre table ».
- [ ] Tests unitaires de `render` (golden files SVG/HTML dans `testdata/`) et de `players/csv.go`.

## 4. Moteur : robustesse et qualité

- [ ] **Correction d'un résultat de tableau après coup** : l'état est recalculé et une `Warning`
  signale un match joué par le mauvais joueur, mais rien ne propose la réparation (rejouer le
  match, annuler la suite). Définir la procédure et l'outiller (`EvMatchCancelled` en série).
- [ ] **Rematchs dans les consolantes** : l'ordre inversé des drops évite les rencontres
  immédiates, pas les suivantes. Mesurer la fréquence par simulation et, si besoin, permuter
  les drops (le tirage est enregistré, donc l'algorithme peut évoluer sans casser les journaux).
- [ ] **Poules : classement intra-poule** sans départage → barrage à 2 vies. Vérifier le cas
  d'égalité à trois pour deux places sur un tournoi réel (durée du barrage).
- [ ] **Bascule Σvies = 2^k en mode `rounds`** : la somme décroît par paquets ; aujourd'hui la
  ronde ne se lance que si elle laisse Σvies ≥ cible, ce qui peut bloquer sur une ronde impaire.
  À tester (`sim_test.go`, config suisse `rounds` + `Target`) et corriger (`budget` dans
  `proposeSwissRound`).
- [ ] **La spécification a du retard sur le lot 0** : `docs/specification.md` décrit encore les
  libellés comme des chaînes (« Ronde 3 ») là où ce sont des codes (`codes.go`), et ne
  documente pas le catalogue des codes. À reprendre avec la publication du site (issue #13),
  qui republie la spécification.
- [x] **API stable** : le format du journal est versionné (`Event.Version`, `JournalVersion`) et
  un journal `Version: 0` est converti à la lecture (`Event.upgraded`). Reste le fuzzing de
  `Apply` sur des journaux aléatoires (issue #14).
- [ ] Renommer le module au chemin définitif du dépôt de l'hôte (`go mod edit -module …`).

## 5. Étude (simulateur de l'étude, hors dépôt)

- [ ] **Effet de la longueur de match** : la base BMAB ne fixe pas l'exposant α (IC [0 ; 0,5–0,8]).
  Pistes : matchs en 3 et 5 points des tournois en ligne (Backgammon Studio, Heroes), ou
  matchs longs (21–25) des championnats ; refaire l'ajustement avec ces données.
- [ ] Modèle de durée réaliste : durées par point dépendant du niveau et de la longueur
  (mesurables dans les fichiers XG : horodatage des coups si présent).
- [ ] Le simulateur Go (`sim/`) et celui de l'étude donnent les mêmes matchs et durées ; les
  probabilités diffèrent de 1 à 2 erreurs types sur certains formats (double élimination,
  GSL). Augmenter les tirages et identifier la cause (ordre des drops, partition des groupes).
- [ ] Grilles à refaire avec le modèle de durée réel et les pauses, pour choisir N par créneau.
