# Le déroulement d'un tournoi

Cette page décrit une journée de tournoi du point de vue du directeur, et ce que le moteur fait à
chaque étape. Rien ici ne suppose de savoir programmer.

## Avant le début : les inscriptions

Le directeur inscrit les joueurs. Un joueur a un nom, éventuellement un club et une cote (le PR :
plus il est bas, meilleur est le joueur). La cote est facultative — un joueur sans cote n'est pas
un joueur parfait, et le moteur ne le traite jamais comme tel.

Une liste peut aussi être importée depuis un fichier CSV, celui d'une billetterie ou d'un
tableur, et réexportée pour le tournoi suivant.

## Le premier appariement

Dès qu'il y a deux joueurs libres, le moteur propose un match : deux noms, une longueur en
points, et un numéro de table. Le directeur confirme ; les joueurs vont s'asseoir.

Les tables ne sont pas de simples numéros. Une table peut être **indisponible** — plateau cassé,
table retirée —, ou **réservée** à la retransmission de la finale. Le moteur les saute, et si
aucune table n'est libre, il le dit au lieu de rester muet.

## Pendant : les résultats

Le directeur saisit les résultats au fur et à mesure : le vainqueur, et le score s'il le connaît.
Le score est facultatif ; ce qui compte, c'est qui a gagné.

Chaque résultat libère deux joueurs, et le moteur propose aussitôt de nouveaux matchs. C'est le
mode **continu** : on ne fait attendre personne. Il a un défaut connu — celui qui finit à 14 h 03
pourrait choisir son adversaire en annonçant à 14 h 04 ou à 14 h 20. Les **micro-rondes** ferment
cette porte : les appariements se font par lots, à heure fixe, et le moteur affiche le temps qui
reste avant le prochain.

Le contraire du continu, ce sont les **rondes** : tout le monde joue en même temps, et l'on
attend le dernier match avant de repartir. C'est plus simple à suivre, et beaucoup plus long.

## Ce qui ne se passe pas comme prévu

Un joueur arrive en retard, après le tirage. Le moteur énumère les places d'exemption encore
libres, et l'y installe **sans refaire le tirage**. S'il n'y a pas de place, il est inscrit
quand même et le moteur dit où il entrera.

Un joueur doit partir à 18 h. Il peut se retirer tout de suite — ses matchs en cours sont perdus
par forfait — ou se retirer **après son match en cours**, qui va alors à son terme.

Un joueur ne se présente pas. C'est un forfait pour ce match seulement : il n'a pas quitté le
tournoi, et suit ensuite le chemin d'un perdant ordinaire, la consolante par exemple.

Un match a été lancé par erreur : on l'annule, et sa place redevient libre.

## Une erreur découverte après coup

Un résultat était faux. **Le journal n'est jamais modifié** : une correction est un événement de
plus, et l'état est recalculé à partir de tout ce qui a été enregistré.

Si le match corrigé était dans un tableau, les matchs suivants ont été joués par les mauvaises
personnes. Le moteur le signale, puis **propose la réparation** : annuler la demi-finale, annuler
la finale, puis relancer les bons matchs. Le directeur confirme, ou fait autrement. Rien n'est
annulé d'office.

Un tournoi déjà clos peut être **rouvert** : le classement final est effacé, la correction
appliquée, et le classement recalculé à la clôture suivante.

## Le repas

Les pauses programmées sont déclarées à l'avance. Un match dont la fin attendue tomberait pendant
la pause est signalé — mais **il reste proposé**. Le directeur sait si ces deux-là mangeront
après.

## La fin

Le classement se lit à tout moment, pas seulement à la fin. Il place les joueurs par ce qu'ils
ont atteint : vainqueur, finaliste, éliminé à tel tour, éliminé avec tant de victoires. **Les ex
æquo restent ex æquo** : aucun départage caché ne les sépare.

La dotation se répartit sur ce classement. Un droit d'entrée, une retenue d'organisation, des
pourcentages — une affiche annonce « 50/30/20 % » et non des montants, puisque le pool dépend du
nombre d'inscrits. Chaque prix est arrondi à l'unité et le reste va au premier, de sorte que la
somme distribuée égale exactement le pool après retenue. Une consolante peut avoir ses propres
prix, sur son propre classement.

## Ce qui est affiché

Le moteur produit des pages autonomes : un seul fichier, qui s'ouvre hors ligne depuis une clé
USB sur l'ordinateur de la salle. L'écran mural montre les actions à faire, la grille des tables,
les matchs en cours, les arbres et le classement. La feuille d'appariements s'imprime sur une
page A4, avec une case vide par joueur pour écrire le score à la main.
