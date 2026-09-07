---
title: "Comprendre le moteur de tournoi"
subtitle: "Ce qu'il fait, pourquoi il le fait ainsi, et ce qu'il ne fait pas encore"
date: "7 septembre 2026"
lang: fr
toc: true
toc-depth: 2
numbersections: true
geometry: "a4paper,margin=2.4cm"
fontsize: 11pt
colorlinks: true
---

# À qui s'adresse ce document

À toute personne qui doit comprendre — ou défendre — le fonctionnement du moteur sans en lire le
code : directeur de tournoi, responsable de club, fédération, joueur qui pose une question à
l'issue d'une ronde, développeur qui découvre le projet.

Il ne suppose aucune connaissance en programmation. Il suppose de savoir ce qu'est un tournoi de
backgammon.

Trois autres documents existent :

| Document | Contenu |
|---|---|
| `docs/etude_formats.md` | La synthèse de l'étude par simulation qui justifie les choix de format |
| `docs/specification.md` | La spécification technique complète (44 pages), de quoi reconstruire le moteur |
| `docs/RESTE_A_FAIRE.md` | La liste priorisée du travail restant |

Le présent document est le résumé argumenté : chaque décision importante y est énoncée avec sa
raison, et la dernière partie répond directement aux objections les plus probables.

---

# Ce que le moteur fait

C'est une **bibliothèque** : un composant sans écran ni base de données, destiné à être intégré
dans un logiciel de tournoi. Il ne remplace pas ce logiciel, il en constitue le cerveau.

Son travail tient en une phrase : **à partir de l'historique de ce qui s'est passé, dire au
directeur de tournoi ce qu'il y a à faire maintenant.**

Concrètement, à chaque instant il produit une liste de propositions du type :

- « Lancer Ronde 3 : Alice contre Bob en 7 points, table 4 »
- « Bye pour Chloé (Ronde 3) »
- « Tirage : Tableau final, tableau de 16 places » — avec le tirage joint
- « Passer à la phase suivante : Tableau final »
- « Clore le tournoi »
- « Attendre : matchs en cours »

Le directeur de tournoi confirme ce qu'il veut, quand il veut. Il peut tout confirmer, une partie,
ou rien. Il peut aussi faire des choses que le moteur n'a pas proposées : corriger un résultat,
annuler un match lancé par erreur, enregistrer un forfait.

Ce que le moteur **ne fait pas** : il n'affiche rien de lui-même, il ne stocke rien, il ne décide
jamais tout seul, il n'envoie aucun message. Ce sont des choix, expliqués plus loin.

---

# Ce qu'on cherche à obtenir

Le format d'un tournoi est un compromis. Les critères retenus, dans cet ordre :

1. **Maximiser les chances des meilleurs joueurs, à durée donnée.** Pas dans l'absolu : à budget de
   temps constant. Un format qui fait mieux gagner les forts en durant deux jours de plus n'est pas
   meilleur, il est plus long.
2. **Être pilotable par un directeur de tournoi**, y compris sur papier en cas de panne.
3. **Aucun match inutile.** Chaque match doit servir à quelque chose : éliminer, qualifier,
   départager.
4. **Aucun départage arithmétique.** Les égalités restent des égalités, ou se jouent.
5. **Accepter n'importe quel effectif.** 13 joueurs, 47 joueurs, 100 joueurs : le format doit
   fonctionner sans bricolage.

Les points 4 et 5 sont des contraintes fortes qui éliminent d'emblée beaucoup de formats usuels.

---

# Ce que l'étude a établi

Une étude par simulation a précédé l'écriture du moteur. Elle a comparé les formats à budget de
temps constant, sur des champs de 32 et 64 joueurs, pour des tournois de 8 h et 16 h. Les résultats
robustes sont les suivants.

## Le modèle

La probabilité qu'un joueur batte un autre suit la formule Elo/FIBS, avec la conversion habituelle
**3 PR = 100 points Elo** :

$$ P = \frac{1}{1 + 10^{-D\sqrt{N}/2000}} $$

où $D$ est l'écart en Elo et $N$ la longueur du match en points.

Cette formule a été confrontée à la base BMAB (33 238 matchs uniques) : **1 PR d'écart vaut environ
2,8 % de chances supplémentaires sur un match en 7 à 11 points**, ce qui correspond à la théorie
(2,8 % mesurés contre 3,9 % théoriques, même ordre de grandeur).

Une inconnue subsiste, et il faut la dire : **l'effet exact de la longueur du match n'est pas
mesurable avec ces données.** L'exposant de $N$ est compris entre 0 et 0,7 sans que l'on puisse
trancher ; seule la valeur 1 est exclue. La valeur retenue, $\sqrt{N}$, est soutenue par la
mécanique du jeu (l'erreur cumulée croît comme $N$, la chance comme $\sqrt N$) mais reste une
hypothèse. C'est la principale fragilité du modèle, et elle est signalée comme telle.

**Durée d'un match** : environ 8 minutes par point en moyenne ; compter 11 minutes par point pour
planifier une ronde, attente comprise.

## Les six résultats

1. **Un suisse à vies est une double élimination sans tableau.** Chaque défaite compte pareil, tout
   effectif est admis, et la fin est toujours résoluble sans départage. C'est le format le plus
   souple qui satisfasse les cinq critères.

2. **L'attente est le premier gaspillage.** Un suisse synchrone — où tout le monde attend la fin de
   la ronde — fait attendre les joueurs **10 à 15 fois plus** qu'un suisse continu : 174 minutes
   d'attente par joueur contre 12, sur un tournoi de 16 heures. Ces 162 minutes valent environ deux
   matchs de plus. C'est le gain le plus important de toute l'étude, et il ne coûte rien.

3. **Le continu est manipulable.** Un joueur qui connaît la situation peut retarder l'annonce de son
   résultat pour choisir son prochain adversaire. Deux parades existent : les blocs GSL
   (l'appariement est figé par les résultats d'un bloc, pas par l'heure) ou le tirage aléatoire dans
   la file d'attente.

4. **La bascule « somme des vies = puissance de 2 » fonctionne.** Quand la somme des vies restantes
   atteint 16 ou 32, on peut passer à un tableau parfaitement équilibré, où un joueur encore
   invaincu est exempt du premier tour. Dans le groupe de tête, que la longueur des matchs compte ou
   non, ce format tient. **3 vies et le format HSBT ne sont jamais gagnants** ; le HSBT jette
   l'information des rondes du samedi (environ la moitié du champ se retrouve qualifiée).

5. **La répartition des longueurs de match entre les tours est presque neutre.** Allonger la finale
   ne change pratiquement rien au résultat : c'est du spectacle gratuit, à prendre comme tel.

6. **Le format joue sur un facteur 2 à 3, pas davantage.** Une fin de semaine ne fait gagner « le
   meilleur joueur » que 4 à 9 % du temps selon l'effectif, contre 1,6 à 3 % si le vainqueur était
   tiré au sort (les mesures du moteur, en fin de document, confirment cet ordre de grandeur).
   C'est le chiffre le plus important à avoir en tête : **aucun format ne rend le backgammon
   déterministe**, et prétendre le contraire serait malhonnête.

---

# Les décisions de format

## Suisse à vies plutôt qu'élimination directe

L'élimination directe est le format à plus forte variance : une seule défaite, souvent sur un match
court, suffit. À 7 points, un joueur meilleur de 3 PR ne gagne qu'environ 58 % du temps — perdre
n'a rien d'exceptionnel.

Le suisse à 2 vies donne à chacun le droit à une défaite, tout en produisant le même nombre de
matchs qu'une double élimination classique (**2N − 2** pour N joueurs, plus un éventuel match de
recharge). Il n'exige aucun tableau, donc aucun effectif particulier.

## Continu plutôt que par rondes

Conséquence directe du résultat n° 2. Dès que deux joueurs du même groupe sont libres, un match leur
est proposé. Personne n'attend que la ronde se termine.

Le mode par rondes reste disponible : il est plus lisible, plus facile à doubler sur papier, et
insensible à la manipulation. C'est un choix du directeur de tournoi, pas une contrainte du moteur.

## Appariement dans le groupe de même nombre de défaites

C'est la règle unique : on affronte quelqu'un qui a le même nombre de défaites que soi. Pas de
score, pas de classement intermédiaire, pas de calcul.

Trois raffinements, dans cet ordre de priorité :

- **un joueur qui a déjà été exempté est apparié en priorité** (pour ne pas cumuler les byes) ;
- **on n'affronte pas deux fois le même adversaire**, sauf s'il n'y a plus d'autre solution ;
- **on évite les joueurs du même club** quand c'est possible (option).

Quand plus aucun appariement régulier n'est possible — typiquement, en fin de tournoi, quand il ne
reste que deux joueurs qui se sont déjà rencontrés, ou un invaincu face à un joueur à une défaite —
le moteur propose quand même le match, sous le libellé « Finale » ou « Match croisé ». C'est ce qui
garantit qu'un tournoi **se termine toujours**.

## Bascule vers un tableau

Chaque match consomme exactement une vie. Quand la somme des vies restantes atteint une puissance de
2, on peut construire un tableau exact : un joueur à 2 vies occupe deux places (il est exempt du
premier tour), un joueur à 1 vie en occupe une.

C'est pourquoi **le tableau final peut avoir 32 places pour 20 joueurs** : sa taille est dictée par
la somme des vies, pas par le nombre de têtes. Les exemptions ne sont pas des cadeaux du hasard,
elles sont gagnées.

## Aucun départage

Un départage arithmétique — Buchholz, différence de points, départage au rating — prétend séparer
deux joueurs qui ont fait la même chose, à partir d'informations qui ne les concernent pas (la force
de leurs adversaires) ou qui ne mesurent pas la performance (la marge d'un match).

Au backgammon, la variance d'un match est trop élevée pour que ces critères portent une information
fiable. Un départage fabrique une décision là où les données n'en contiennent pas.

Deux réponses honnêtes existent, et le moteur n'utilise que celles-là :

- **l'ex æquo**, avec partage des prix des places occupées ;
- **le barrage joué**, quand une place qualificative est en jeu.

## Aucune tête de série

Techniquement possible (le moteur connaît le PR de chaque joueur) et prévu comme option future,
mais écarté pour la version 1, pour deux raisons :

- c'est la culture actuelle du backgammon, qui tire au sort ;
- protéger les meilleurs revient à leur ajouter un avantage structurel, alors que l'objectif de
  départ — leur donner de meilleures chances — est déjà atteint par le choix du format, et que le
  classement PR est lui-même bruité.

## Aucune finale à handicap

Même raison : le format doit récompenser le parcours, pas le corriger.

## Longueurs de match

Résultat n° 5 : c'est presque neutre. Le moteur permet une longueur par phase et une longueur de
finale distincte. Allonger la finale est un choix d'organisateur, pas un choix statistique.

---

# Les décisions techniques

Ces décisions n'intéressent pas directement les joueurs, mais elles expliquent le comportement du
logiciel — et notamment ce qui se passe quand quelque chose se passe mal.

## Tout passe par un journal

La seule donnée conservée est un **journal** : la liste, dans l'ordre, de tout ce qui s'est passé.
Création du tournoi, inscription de chaque joueur, lancement de chaque match, résultat, forfait,
tirage, correction. Rien d'autre n'est stocké.

L'état du tournoi — qui est en vie, qui joue contre qui, quel est le classement — n'est jamais
enregistré : il est **recalculé** à partir du journal chaque fois qu'on en a besoin.

Trois conséquences pratiques :

- **Panne, redémarrage, changement de machine** : on relit le journal, on retrouve l'état exact. Il
  n'y a pas d'état à sauvegarder, donc pas d'état à perdre.
- **Audit** : on peut rejouer le tournoi à n'importe quel moment de son déroulement et voir
  exactement ce que le logiciel proposait à cet instant. C'est ainsi que sont produites les pages
  d'exemple du dépôt.
- **Contestation** : le journal est la trace. Il dit qui a joué contre qui, à quelle heure, sur
  quelle table, avec quel résultat.

## On ne réécrit jamais le journal

Une erreur ne se corrige pas en effaçant. Elle se corrige en **ajoutant** :

| Erreur | Ce qu'on ajoute |
|---|---|
| Mauvais vainqueur saisi | une correction de résultat |
| Match lancé par erreur | une annulation de match |
| Joueur qui abandonne | un forfait |

Le logiciel recalcule alors tout, et **signale ce qui n'est plus cohérent** — typiquement : « le
match M17 a été joué par Alice et Bob, mais le tableau attend maintenant Alice et Chloé ». Le
directeur de tournoi voit immédiatement l'étendue des dégâts.

Ce que le moteur ne fait pas encore : proposer la réparation. C'est une limite connue.

## Le moteur propose, le directeur dispose

Aucune action n'est automatique. Le moteur ne lance pas un match : il propose de le lancer. Cette
séparation a un coût (il faut confirmer) et deux avantages :

- le directeur de tournoi garde la main, y compris pour faire autre chose que la proposition ;
- chaque bouton de l'interface correspond exactement à une ligne du journal, ce qui rend le système
  vérifiable.

## Les tirages sont enregistrés, pas recalculés

Quand le moteur tire un tableau ou compose des groupes, **le résultat du tirage est écrit dans le
journal**, pas seulement la décision de tirer.

Cela veut dire qu'on peut améliorer l'algorithme de tirage plus tard sans invalider les tournois
déjà joués : rejouer un ancien journal ne fait plus appel à l'algorithme, il relit le tirage.

## Une bibliothèque, pas un service

Le moteur s'intègre dans le logiciel hôte, dans le même processus, sans réseau ni base de données.
Il n'a aucune dépendance extérieure. Cela le rend facile à embarquer, facile à tester, et sans point
de panne propre.

---

# Les formats disponibles

| Format | Description |
|---|---|
| **Suisse à vies** | *L* vies (2 par défaut), appariement dans le groupe de même nombre de défaites, en continu ou par rondes. Option : arrêt quand la somme des vies atteint une puissance de 2, pour basculer sur un tableau. |
| **Tableau à exemptions** | Élimination directe où un joueur arrivé avec 2 vies est exempt du premier tour. |
| **Blocs GSL** | Groupes de 4 (ou 3, ou 2) à double élimination interne pour les invaincus, mini-tableaux pour les joueurs à une défaite. Version non manipulable du suisse à vies, qui se cale bien sur des créneaux horaires. |
| **Tableau** | Élimination simple, avec en option une consolante progressive, une dernière chance, et une vraie double élimination (le vainqueur du tableau principal doit être battu deux fois). |
| **Poules** | Toutes rondes par poules (table de Berger), avec un nombre de qualifiés par poule ; les égalités à la place qualificative se règlent par barrage. |

Les formats s'enchaînent en **phases**. Le format recommandé par l'étude est :

> **Suisse 2 vies en continu**, appariement aléatoire dans le groupe de même nombre de défaites,
> sans revanche, bye au groupe le plus bas ; arrêt quand la somme des vies vaut 16 ou 32 ; puis
> **tableau à exemptions** avec des matchs plus longs.
>
> Variante anti-manipulation : blocs GSL calés sur les repas.

---

# Ce qui est vérifié, ce qui ne l'est pas

Cette partie est celle qu'il faut lire avant de confier un tournoi au moteur.

## Ce qui est vérifié

Le moteur est validé par **simulation** : on lui fait jouer des tournois entiers, et on vérifie que
certaines propriétés sont toujours vraies. La batterie standard couvre **11 configurations de
format × 8 effectifs (2, 3, 5, 8, 13, 32, 64 et 100 joueurs) × 8 tirages**, soit 704 tournois
complets, et vérifie à chaque événement :

- le tournoi se **termine toujours**, quel que soit l'effectif — y compris 3 joueurs ou 13 ;
- il y a **un vainqueur et un seul** ;
- **tous les inscrits sont classés**, aucun n'est oublié ;
- **rejouer le journal donne exactement le même classement**, et aucune incohérence ;
- **aucun joueur éliminé ne joue** ;
- **aucun match entre joueurs de groupes différents**, hors les cas de fin de tournoi explicitement
  prévus ;
- **peu de revanches** en suisse.

S'y ajoutent des vérifications ciblées : le nombre de matchs produits par chaque format sur
64 joueurs, le comportement après correction d'un résultat, et le comportement après le forfait d'un
joueur déjà placé dans un tableau.

Enfin, une campagne longue (1 500 tournois de 64 joueurs par format) mesure les probabilités de
victoire, le nombre de matchs et la durée, à comparer aux valeurs de l'étude. **Tout changement de
l'algorithme d'appariement doit repasser par cette campagne.**

## Ce qui ne l'est pas

**Le moteur n'a jamais dirigé de tournoi réel.** C'est le point le plus important de ce document.

Ce qui est simulé, c'est le déroulement mathématique d'un tournoi. Ce qui ne l'est pas, c'est la
réalité d'une salle : les retardataires, les joueurs qui disparaissent, les tables indisponibles,
les pauses repas, un directeur de tournoi qui veut faire autrement.

Les manques identifiés, par ordre d'urgence :

- **Interface** : le moteur ne fournit que la logique ; l'écran du directeur de tournoi reste à
  écrire.
- **Retardataires** : un joueur inscrit après le tirage d'un tableau n'entre nulle part.
- **Forfaits fins** : le forfait est aujourd'hui total. Il manque le forfait sur un seul match, et
  le retrait « à partir de la ronde suivante ».
- **Tables** : pas de table indisponible ou réservée, pas de changement de table en cours de match.
- **Pauses repas** : le moteur ne sait pas s'abstenir de lancer un match qui finirait après l'heure
  de la pause.
- **Réparation après correction** : l'incohérence est signalée, pas réparée.
- **Prix** : les montants par place et le partage entre ex æquo existent ; les structures en
  pourcentage du pool, la retenue d'organisation et les prix par section restent à faire.
- **Longueurs de match par tour** (9 / 11 / 13 / 15) : pas encore paramétrables.

La recommandation qui en découle : **un premier tournoi de club de 16 à 32 joueurs, en doublant sur
papier**, en notant chaque fois que le directeur de tournoi a voulu faire autre chose que la
proposition du moteur. Ce sont ces cas-là qui définiront les fonctions manquantes.

---

# Questions fréquentes

## « Pourquoi mes deux joueurs ex æquo ne sont-ils pas départagés ? »

Parce qu'ils ont fait la même chose. Les départager exigerait d'utiliser soit la force de leurs
adversaires respectifs — qu'ils n'ont pas choisis —, soit la marge de leurs victoires — qui au
backgammon dépend fortement du videau et de la chance. Ni l'un ni l'autre ne mesure ce qu'on
prétendrait mesurer.

Si la place a une conséquence (une qualification), elle se joue : c'est le barrage. Si elle n'en a
pas, les prix des places concernées sont partagés à parts égales.

## « Pourquoi le meilleur joueur ne gagne-t-il que 4 à 9 % du temps ? N'est-ce pas un échec ? »

Non, c'est le backgammon. Sur 64 joueurs, les formats mesurés donnent entre 3,5 % et 5,7 % ; sur
32 joueurs, l'étude monte jusqu'à environ 9 %. Si le vainqueur était tiré au sort parmi 64 joueurs,
ce serait 1,6 %. Le format multiplie donc ce chiffre par 2 à 3,5.

Aucun format raisonnable ne fait beaucoup mieux à durée égale : pour que le meilleur gagne vraiment
souvent, il faudrait des matchs de 25 points et une semaine de jeu.

Un format qui prétendrait garantir la victoire du meilleur mentirait sur la nature du jeu.

## « Pourquoi un joueur qui a perdu peut-il gagner le tournoi ? »

C'est la définition d'un format à deux vies. À 7 points, un joueur meilleur de 3 PR que son
adversaire gagne environ 58 % du temps : perdre un match n'est pas un accident rare, c'est la
normale statistique. Éliminer sur une seule défaite maximiserait la part du hasard, pas l'inverse.

## « Le jeu en continu n'avantage-t-il pas ceux qui jouent vite ? »

Il avantage surtout tout le monde : l'attente passe de 174 minutes par joueur à 12 sur un tournoi de
16 heures. Ce temps rendu vaut environ deux matchs supplémentaires.

Il a un défaut réel, identifié et assumé : un joueur peut retarder l'annonce de son résultat pour
influencer son appariement. Deux parades existent — les blocs GSL, où l'appariement est figé par les
résultats et non par l'heure, et l'appariement par lots à intervalle fixe. Le mode par rondes reste
disponible pour qui préfère.

Ces parades ne sont pas gratuites : sur 64 joueurs, les blocs GSL demandent **14 h 25** de salle
contre 10 h 20 pour le suisse continu, et le mode par rondes 14 h 10 — pour exactement le même
nombre de matchs. Synchroniser coûte du temps, quelle que soit la forme que prend la
synchronisation.

## « Pourquoi le tableau final a-t-il 32 places alors que nous sommes 20 ? »

Parce qu'il est dimensionné par la **somme des vies restantes**, pas par le nombre de joueurs. Un
joueur encore invaincu vaut deux places : il est exempt du premier tour. C'est la récompense de son
parcours, et c'est ce qui permet d'avoir un tableau exact, sans exemption arbitraire.

## « Pourquoi ai-je un bye alors qu'un autre joueur en a déjà eu un ? »

La règle appliquée est : celui qui a déjà été exempté est apparié en priorité, donc il n'a
normalement pas de second bye tant que d'autres n'en ont pas eu. Cette règle s'applique **à
l'intérieur d'un groupe de joueurs ayant le même nombre de défaites**. Entre deux groupes différents,
l'équité n'est pas encore garantie : c'est une limite connue et documentée.

## « J'ai perdu contre X au premier tour et je le retrouve en consolante. Normal ? »

La consolante est construite pour éviter la rencontre **immédiate** : les joueurs qui retombent du
tableau principal sont insérés dans l'ordre inverse, ce qui écarte ceux qui viennent de la même
moitié de tableau. Cela n'évite pas les retrouvailles aux tours suivants. La fréquence réelle de ces
revanches reste à mesurer ; le cas échéant, l'ordre d'insertion sera modifié — sans invalider les
tournois déjà joués, puisque les tirages sont enregistrés.

## « Le directeur de tournoi s'est trompé de vainqueur. Faut-il tout refaire ? »

Non. Il saisit une correction, qui est ajoutée à l'historique — l'erreur reste visible, ce qui est
volontaire. Le logiciel recalcule tout et signale ce qui n'est plus cohérent, par exemple un match de
tableau désormais joué par le mauvais joueur.

En revanche, il ne propose pas encore la réparation (rejouer tel match, annuler telle suite). Cette
décision revient au directeur de tournoi.

## « Et si l'ordinateur s'éteint en plein tournoi ? »

On relit l'historique et on retrouve l'état exact, y compris les matchs en cours et les tirages déjà
effectués. Il n'y a rien d'autre à sauvegarder. Cette propriété est vérifiée sur les 704 tournois de
la batterie de tests : rejouer l'historique redonne toujours le même classement.

À condition, évidemment, que le logiciel hôte ait bien enregistré l'historique au fur et à mesure —
c'est sa responsabilité, et c'est le premier point à tester avant un tournoi réel.

## « Peut-on inscrire un joueur en retard ? »

Dans un suisse, oui, tant que la phase n'est pas figée : il entre avec toutes ses vies. Dans un
tableau ou des poules, seulement avant le tirage. Après le tirage d'un tableau, non — le joueur est
enregistré mais n'entre nulle part. C'est une limite connue, à corriger avant un usage réel.

## « Pourquoi les meilleurs joueurs ne sont-ils pas protégés dans le tableau ? »

Parce que le backgammon tire au sort, et parce que le PR est une mesure bruitée. Protéger les têtes
de série ajouterait un avantage structurel à des joueurs que le format avantage déjà. La
fonctionnalité est prévue comme option ; elle n'est pas activée.

## « Pourquoi 2 vies plutôt que 3 ? »

Parce que l'étude a mesuré que **3 vies n'est jamais gagnant** à budget de temps constant : le
troisième droit à l'erreur coûte plus de temps qu'il n'apporte de justesse. Le moteur accepte
néanmoins un nombre de vies quelconque, si un organisateur veut en faire l'essai.

## « Combien de temps dure un tournoi ? »

Compter **8 minutes par point** en moyenne pour un match, et **11 minutes par point** pour planifier
une ronde, attente comprise. Un match en 7 points dure donc environ une heure.

Pour le nombre de matchs : un format à 2 vies produit environ **2 fois le nombre de joueurs**
(126 matchs pour 64 joueurs) ; une élimination simple en produit **le nombre de joueurs moins un**
(63 matchs pour 64 joueurs), mais avec une variance bien plus forte sur le résultat.

En temps de salle simulé sur 64 joueurs et des matchs en 7 points, cela donne environ 7 h pour une
élimination simple, 10 h pour un suisse 2 vies en continu et 14 h pour le même suisse par rondes
(voir les chiffres de référence).

Le logiciel fournit en outre une **prévision de fin** en cours de tournoi, par simulation des
matchs restants : elle donne une médiane et un délai à 90 %.

## « Pourquoi une bibliothèque plutôt qu'un logiciel complet ? »

Pour que la logique du tournoi puisse être vérifiée indépendamment de l'affichage, et pour qu'elle
soit réutilisable par n'importe quel logiciel hôte. Les 704 tournois de la batterie de tests sont
joués sans qu'aucun écran n'existe : c'est précisément ce qui permet de les jouer par milliers.

---

# Chiffres de référence

Mesures sur **1 500 tournois simulés de 64 joueurs**, matchs en 7 points, champ de PR tirés dans une
loi normale de moyenne 6 et d'écart-type 2, bornée entre 2 et 10. « P(meilleur) » est la probabilité
que le meilleur joueur du champ gagne le tournoi ; « P(top 4) » celle qu'un des quatre meilleurs le
gagne. La durée est le temps de salle simulé, toutes tables confondues.

| Format | P(meilleur) | P(top 4) | Matchs | Durée |
|---|---|---|---|---|
| Élimination simple | 3,8 % | 14,6 % | 63,0 | 7 h 00 |
| Suisse 2 vies, continu | 3,5 % | 16,5 % | 126,5 | 10 h 20 |
| Suisse 2 vies, par rondes | 4,9 % | 15,9 % | 126,5 | 14 h 10 |
| Suisse 2 vies → tableau 16 | 4,2 % | 15,5 % | 124,3 | 9 h 45 |
| Blocs GSL | 4,5 % | 15,3 % | 126,5 | 14 h 25 |
| Double élimination avec recharge | 5,7 % | 18,0 % | 126,5 | 13 h 35 |

Sur 1 500 tirages, l'incertitude est d'environ **±0,5 point** sur P(meilleur) et **±1 point** sur
P(top 4). Les durées supposent un nombre de tables illimité : ce sont des durées de salle, pas des
durées de jeu.

Trois lectures :

- **Le prix de l'attente est directement visible.** Le suisse 2 vies joue exactement le même nombre
  de matchs en continu et par rondes — 126,5 — mais met **3 h 50 de plus** par rondes. C'est du
  temps de salle pur, sans aucun match supplémentaire. C'est l'argument principal en faveur du
  continu.
- **Le format recommandé est le meilleur compromis.** Le suisse 2 vies suivi d'un tableau de 16
  obtient une qualité comparable au reste de la famille à deux vies pour la durée la plus courte du
  groupe (9 h 45 contre 13 à 14 h).
- **Les écarts de qualité sont réels mais modestes.** Sur P(top 4), la famille à deux vies (15,3 à
  18,0 %) devance nettement l'élimination simple (14,6 %) — l'écart dépasse l'incertitude. Sur
  P(meilleur), en revanche, seule la double élimination se détache franchement ; les autres écarts
  sont du même ordre que le bruit de mesure et ne doivent pas être surinterprétés.

Pour référence, un vainqueur tiré au sort parmi 64 joueurs donnerait P(meilleur) = 1,6 % et
P(top 4) = 6,3 %. Le format multiplie donc ces chiffres par 2 à 3,5 — pas davantage.

Ces mesures proviennent du simulateur du dépôt. Le simulateur indépendant de l'étude donne les mêmes
nombres de matchs et les mêmes durées, mais des probabilités qui diffèrent de 1 à 2 écarts types sur
la double élimination et le GSL ; la cause n'est pas encore identifiée. Les chiffres ci-dessus sont
donc à lire comme des ordres de grandeur fiables, pas comme des valeurs au dixième de point près.

---

# Pour aller plus loin

| Question | Document |
|---|---|
| Pourquoi ces formats et pas d'autres ? | `docs/etude_formats.md` |
| Comment le moteur fonctionne exactement ? | `docs/specification.md` |
| Qu'est-ce qui reste à faire ? | `docs/RESTE_A_FAIRE.md` |
| À quoi ressemble un tournoi rendu ? | `exemples/index.html` |
| Comment l'essayer soi-même ? | `README.md`, section « Tester à la main » |

Pour essayer le moteur en console, sans rien installer d'autre que Go :

```bash
go run ./cmd/tournoi-td -journal montournoi.json -format suisse_tableau
```

Puis, dans la console : `ajoute Alice Paris 4.2`, `propose`, `ok tous`, `resultat M1 alice 7 3`,
`classement`. Le fichier `affichage.html` créé à côté du journal se recharge tout seul dans un
navigateur.
