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
- [x] **Règle anti-manipulation du continu** (issue #7). `PhaseConfig.BatchMinutes` fait attendre
  les joueurs libres jusqu'à l'échéance du prochain lot, puis apparie d'un coup tous ceux d'un
  même groupe de défaites. L'échéance est DÉRIVÉE du journal — le départ du dernier match lancé
  dans la phase plus `BatchMinutes` (`horaires.go`) — et voyage dans `Action.Until` avec
  `ReasonWaitingBatch`, pour que l'hôte affiche un compte à rebours. Le temps entre par
  `ProposeAt(now)` ; `Propose()` prend l'horodatage du dernier événement.
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
- [x] **Configuration modifiable en cours, et réouverture** (issue #4). `EvConfigChanged` porte
  la configuration ENTIÈRE ; `acceptConfig` (reconfig.go) refuse seulement deux choses — retirer
  une phase ouverte, et changer le `Kind` d'une phase terminée, tirée ou commencée — en nommant
  la phase et la raison. La bascule, les longueurs, les tables, la dotation et une phase ajoutée
  après la courante passent. `EvReopened` annule la clôture et efface `Final`, recalculé à la
  clôture suivante. `EvLengthChanged` reste lu.
- [x] **Pauses programmées** (issue #7). `Config.Breaks []TimeRange` ; une proposition dont le
  match rencontrerait une pause porte `Action.Warn = WarnEndsInBreak` et RESTE proposée — rien
  n'est bloqué, le directeur décide. Reste à caler un bloc GSL par créneau, qui est un autre
  sujet (l'ordonnancement, pas l'avertissement).
- [ ] **Byes et exemptions équitables sur plusieurs rondes** : en mode `rounds`, le bye va au
  joueur du groupe le plus bas n'en ayant pas eu ; vérifier la règle « pas de second bye tant que
  d'autres n'en ont pas eu » entre groupes différents (aujourd'hui par groupe). `pairGroup`.
- [ ] **Consolante et dernière chance : options de saut** (les perdants du tour 1 seulement, ou
  jusqu'au tour k ; consolante non progressive « à tirage » avec une heure limite d'entrée).
  Les constructeurs `consolationSection` / `bracketFromSrcs` prennent des sources : ajouter un
  filtre par tour d'origine.
- [x] **Têtes de série optionnelles** dans les tableaux (issue #10). `PhaseConfig.Seeding` vaut
  `""` (défaut, tirage intégralement aléatoire — le choix de l'étude) ou `SeedingRating`, qui
  place les joueurs par cote selon la construction classique (`seeding.go`). Une cote inconnue
  passe derrière tout le monde ; dans un `lives_bracket`, les joueurs à deux vies passent devant
  tous les autres, la structure du tableau l'exigeant. Le tirage reste dans `EvDraw`.
- [ ] **Classement des places non gagnantes** : règles à valider avec la FFBG (par tour atteint
  dans les tableaux, par victoires à l'élimination dans les suisses, ex æquo partagés). Les
  notes de classement sont désormais des **codes** (`Note`, `codes.go`), traduisibles par
  l'hôte ; restent les règles de classement elles-mêmes, à valider avec la FFBG.
- [x] **Prix** (issue #9). `Config.Prizes` est un `PrizePool` (`prizes.go`) : droit d'entrée,
  retenue (montant et/ou pourcentage), barème par section en pourcentages du pool distribuable ou
  en montants fixes. `State.PrizeAmounts(sec)` arrondit à l'unité et met le reste au premier, de
  sorte que la somme distribuée égale exactement le pool après retenue ; `SectionRanking(sec)`
  donne le classement propre d'une section (par tour atteint) et `StandingsCSV` sort une section
  par bloc. La forme ancienne (`"prizes": [100, 60, 40]`) reste lue. Reste à valider avec la FFBG
  les règles de classement des places non gagnantes (point ci-dessus).
- [x] **Export / import des joueurs** (issue #12). `players.ToCSV` écrit l'annuaire dans le
  format que `players.FromCSV` relit sans perte (colonnes `nom;club;cote`, colonne `id` seulement
  quand l'identifiant ne se déduit pas du nom) ; la console TD a la commande `exporte`. Au
  passage, un en-tête EXACT l'emporte désormais sur un en-tête qui contient seulement le mot
  cherché : « prénom » contient « pr » et volait la colonne de cote. Reste l'export du journal
  en PDF ou texte lisible (feuille d'appariements imprimable : issue #11).

## 3. Rendu et affichage

- [ ] Style et charte du site hôte pour `render/` (aujourd'hui SVG et HTML bruts sans CSS).
- [ ] Arbre de double élimination : dessiner principal et consolante côte à côte avec les drops.
- [ ] Tableau des vies : afficher les adversaires déjà rencontrés et le temps d'attente.
- [ ] Écran joueur (téléphone) : « votre prochain match » et « votre table ».
- [ ] Tests unitaires de `render` (golden files SVG/HTML dans `testdata/`). Ceux de
  `players/csv.go` existent depuis l'issue #12.

## 4. Moteur : robustesse et qualité

- [x] **Les avertissements sont recalculés dès le résultat.** `check()` n'était appelée que
  depuis `recompute()`, donc depuis les seules corrections, annulations et forfaits : un score
  au-delà de la longueur annoncée n'apparaissait qu'après une correction sans rapport, ou
  jamais. `Apply` la rappelle désormais sur `match_started` et `result`. Au passage, `check()`
  parcourt les GRAPHES et non les matchs — chercher pour chaque match sa place dans toutes les
  sections était quadratique, ce qui était supportable une fois par correction et ne l'est plus
  à chaque résultat.

- [x] **Correction d'un résultat de tableau après coup** (issue #8). `Propose` émet la réparation
  comme des propositions ordinaires : l'action `cancel_match` annule le match joué par les
  mauvais joueurs ET tout ce qui en descend, du plus profond au moins profond (`reparation.go`).
  Rien n'est appliqué d'office ; les relances correctes arrivent à l'appel suivant, une fois les
  places libérées. Défaut découvert au passage : `recompute` ne remettait pas `GMatch.MatchID` à
  vide, si bien qu'un match de tableau annulé bloquait sa place pour toujours.
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
