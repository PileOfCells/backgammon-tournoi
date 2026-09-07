# Synthèse de l'étude des formats (septembre 2026)

L'étude a été menée avec un simulateur Python (non publié) puis vérifiée avec `sim/` de ce dépôt.

Critères : maximiser l'espérance de gain des meilleurs joueurs à durée donnée, format pilotable
par un TD (papier ou logiciel), aucun match inutile, **aucun départage**.

## Modèle

- Probabilité de gain : formule Elo/FIBS `P = 1/(1+10^(-D·√N/2000))` avec 3 PR = 100 Elo.
  Sur la base BMAB (33 238 matchs uniques), la taille de l'effet est confirmée : 1 PR d'écart
  vaut 2,8 % à 7–11 points (β = 0,036 à α = 0,5, théorie 0,039).
- L'exposant α de la longueur N n'est pas identifiable avec ces données : vraisemblance plate
  entre 0 et 0,7, α = 1 exclu. Le PR du match donne α ≈ 0,23, IC [0 ; 0,53]. La mécanique
  « erreur cumulée ∝ N, chance ∝ √N » est vérifiée, ce qui soutient α = 0,5 pour un PR réalisé.
- Durée d'un match : ≈ 8 min/point en moyenne (11 min/point pour planifier une ronde).

## Résultats robustes (grilles à budget de temps constant, 32 et 64 joueurs, 8 h et 16 h)

1. Un suisse à vies est une double élimination sans tableau : chaque défaite compte pareil,
   tout effectif est admis, la fin est toujours résoluble sans départage.
2. L'attente est le premier gaspillage : le suisse synchrone fait attendre 10 à 15 fois plus que
   le continu (174 min contre 12 par joueur sur 16 h), soit deux points de match perdus.
3. Le continu est manipulable par l'heure d'annonce du résultat ; parades : blocs GSL
   (appariement figé par les résultats) ou tirage aléatoire dans la file.
4. Bascule « somme des vies = 2^k » (2 vies = exemption du premier tour) : dans le groupe de
   tête que la longueur compte ou non. L'élimination simple longue est la meilleure si √N est
   vrai, la pire sinon. 3 vies et HSBT ne sont jamais gagnants ; HSBT jette l'information des
   rondes du samedi (≈ 50 % du champ qualifié).
5. Répartition des longueurs entre rondes : presque neutre (finale longue = spectacle gratuit).
6. Une fin de semaine ne fait gagner « le meilleur » que 4 à 9 % du temps (64 ou 32 joueurs),
   contre 1,6 à 3 % au hasard : le format joue sur un facteur 2 à 3.

Choix retenus : pas de finale à handicap, pas de têtes de série protégées (culture actuelle du backgammon).
Les têtes de série existent depuis en **option** du moteur (`seeding: "rating"` sur une phase de
tableau), éteintes par défaut : la conclusion de l'étude reste le défaut, l'option est là pour
l'organisateur qui décide autrement.

## Format recommandé

Suisse 2 vies en continu (appariement aléatoire dans le groupe de même nombre de défaites, pas
de rematch, bye au groupe le plus bas), arrêt quand la somme des vies vaut 16 ou 32, puis tableau
à exemptions avec matchs plus longs. Variante anti-manipulation : blocs GSL calés sur les repas.
