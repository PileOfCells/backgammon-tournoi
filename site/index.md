# Nicomaque

**Nicomaque** est un moteur de tournoi de backgammon. Ce n'est pas un logiciel qu'on ouvre :
c'est une bibliothèque que d'autres logiciels utilisent pour diriger un tournoi — inscrire les
joueurs, apparier les matchs, tenir les tableaux, produire le classement.

Il est écrit en Go, sans aucune dépendance, et se lit en entier. Il est publié sous une licence
libre et son code est sur [GitHub](https://github.com/PileOfCells/backgammon-tournoi).

## Ce qu'il fait

Un directeur de tournoi passe sa journée à répondre à une seule question : *qui joue contre qui,
maintenant, et sur quelle table ?* Nicomaque répond à cette question, et rien d'autre.

À tout instant, il propose une liste d'actions — lancer ce match sur cette table, donner cette
exemption, tirer ce tableau, passer à la phase suivante, clore le tournoi. Le directeur en
confirme ce qu'il veut. Chaque confirmation devient un **événement** ajouté à un journal, et
l'état du tournoi est reconstruit à partir de ce journal.

**Le moteur propose, le directeur décide.** Rien n'est appliqué d'office, jamais. Un
avertissement — un score incohérent, un match joué par le mauvais joueur, une fin de match qui
tombe pendant le repas — est dit, pas imposé : le directeur sait des choses que le moteur ignore.

## Ce qu'il ne fait pas

Il n'a pas d'interface. Il ne stocke rien. Il n'affiche aucune phrase : tout ce qui sort du
moteur est un **code**, que le logiciel hôte traduit dans la langue de son utilisateur.

Il ne départage pas. Les égalités restent des égalités, ou se règlent par un barrage. Aucun
classement n'est décidé par un critère caché.

Il n'a jamais dirigé de vrai tournoi. Il est testé par simulation — des milliers de tournois
joués à chaque modification — mais la simulation n'est pas la salle.

## Par où commencer

```{toctree}
:maxdepth: 1

deroulement
formats
integration
comprendre_le_moteur
etude_formats
specification
```

- [Le déroulement d'un tournoi](deroulement.md) — la journée du directeur, de l'inscription au
  classement final.
- [Les formats](formats.md) — suisse à vies, tableaux, GSL, poules, et lequel choisir.
- [Intégrer le moteur](integration.md) — la boucle que le logiciel hôte écrit, en une page.
- [Comprendre le moteur](comprendre_le_moteur.md) — les décisions de conception et leurs
  justifications, les questions fréquentes, les chiffres de référence.
- [L'étude des formats](etude_formats.md) — la simulation qui a servi à choisir.
- [La spécification](specification.md) — assez détaillée pour reconstruire le moteur.

## Le nom

Nicomaque était le fils d'Aristote, et le destinataire de l'*Éthique à Nicomaque*, où il est
question de ce qu'est une décision juste. Diriger un tournoi consiste à en prendre beaucoup, vite,
devant des gens qui attendent.
