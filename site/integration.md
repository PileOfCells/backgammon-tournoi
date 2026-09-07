# Intégrer le moteur

Cette page s'adresse à qui écrit le logiciel qui utilisera Nicomaque. Elle tient en une page ; la
[spécification](specification.md) donne le détail.

## Le principe

Le moteur ne persiste rien et n'affiche rien. L'hôte conserve un **journal** — une liste ordonnée
d'événements — et le moteur en reconstruit l'état à la demande.

```go
import tournoi "github.com/PileOfCells/backgammon-tournoi"
```

Trois appels suffisent à comprendre la boucle :

- `Replay(journal)` reconstruit l'état complet ;
- `Propose()` — ou `ProposeAt(now)` — dit ce qu'il y a à faire maintenant ;
- `Apply(événement)` applique la décision du directeur.

## La boucle

```go
// 1. Créer le tournoi
cfg := tournoi.Config{Name: "Open de printemps", Phases: []tournoi.PhaseConfig{
    {Kind: tournoi.KindSwissLives, Length: 7, Target: 16},
    {Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11},
}}
st, créé, err := tournoi.New(cfg, graine, time.Now())
journal := tournoi.Journal{créé}

// 2. Inscrire
ev := tournoi.PlayerAddedEvent(joueur, time.Now())
st.Apply(ev)
journal = append(journal, ev)

// 3. Proposer, confirmer
for _, a := range st.ProposeAt(time.Now()) {
    // afficher l'action au directeur ; s'il la confirme :
    ev, err := st.EventFromAction(a, time.Now())
    st.Apply(ev)
    journal = append(journal, ev)
}

// 4. Saisir un résultat
ev = tournoi.ResultEvent(idDuMatch, vainqueur, scoreA, scoreB, time.Now())
st.Apply(ev)
journal = append(journal, ev)
```

Le journal se sérialise en JSON. Une reprise après panne est un `Replay`.

## Les trois règles à ne pas oublier

**On ne modifie jamais le journal.** Une correction est un événement de plus
(`CorrectionEvent`), un forfait aussi (`PlayerWithdrawnEvent`), une annulation aussi
(`CancelEvent`). Rien n'est effacé, rien n'est réécrit.

**Rien de ce qui sort du moteur n'est destiné à l'affichage.** Les libellés de match, les notes
de classement, les avertissements et les raisons d'attente sont des **codes** structurés. C'est
l'hôte qui les rend, dans la langue de son utilisateur. Un rendu français est fourni pour la
console et les exemples, mais un hôte multilingue l'ignore.

**Les tirages sont enregistrés, pas recalculés.** L'événement de tirage porte le placement
obtenu. Rejouer un journal ne dépend donc jamais de l'algorithme d'appariement, et celui-ci peut
évoluer sans casser un journal existant.

## L'affichage

Le paquet `render` produit des pages autonomes — un seul fichier, feuille de style embarquée,
aucune ressource externe, aucun script. Il prend un traducteur (`Labeler`) que l'hôte fournit :
les codes du moteur et les mots du rendu lui-même passent tous par là.

Il produit l'écran de salle, les arbres de tableaux avec les descentes vers la consolante, la
grille des tables, le tableau des vies, le classement, et la feuille d'appariements imprimable.

## L'essayer sans rien écrire

```bash
git clone https://github.com/PileOfCells/backgammon-tournoi
cd backgammon-tournoi

# Simuler un tournoi complet et écrire les pages
go run ./cmd/tournoi-demo -format suisse_tableau -joueurs 32 -etapes -sortie demo

# Diriger un tournoi à la main, dans une console
go run ./cmd/tournoi-td -journal montournoi.json -format suisse_tableau
```
