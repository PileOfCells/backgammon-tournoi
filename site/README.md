# Le site

Site publié sur GitHub Pages à chaque poussée sur `main` :
<https://pileofcells.github.io/backgammon-tournoi/>

## Construire

```bash
python3 -m venv .venv-doc
.venv-doc/bin/pip install -r site/requirements.txt

.venv-doc/bin/python site/build.py              # les neuf langues, dans site/_site/
.venv-doc/bin/python site/build.py --langue fr  # une seule, pour aller vite
.venv-doc/bin/python site/build.py --pot        # régénérer les catalogues, puis construire
```

## Ce qui est traduit, et ce qui ne l'est pas

Le français est la **source** ; les huit autres langues (en, de, el, es, fi, it, ja, ru) sont des
catalogues gettext dans `locale/`. Une chaîne non traduite retombe sur le français : c'est le
comportement natif de gettext, et il est voulu — mieux vaut une page à moitié française qu'une
page absente.

| Document | msgid | Traduit |
|---|---|---|
| `index.md` | 20 | les neuf langues |
| `deroulement.md` | 28 | les neuf langues |
| `formats.md` | 36 | les neuf langues |
| `integration.md` | 18 | les neuf langues |
| `sphinx` (le sélecteur de langue) | 1 | les neuf langues |
| `comprendre_le_moteur.md` | 243 | **français seulement** |
| `etude_formats.md` | 18 | **français seulement** |
| `specification.md` | 819 | **français seulement** |

Autrement dit : **les pages d'accueil et d'introduction sont traduites dans les neuf langues**,
et les trois documents longs restent en français dans toutes. Ce n'est pas un oubli, c'est un
ordre de priorité : ce sont les pages d'introduction qu'un joueur ou un organisateur ouvre, et
les trois documents longs s'adressent à qui intègre le moteur ou à qui veut le détail des choix.

Reste donc à traduire, dans huit langues : 243 + 18 + 819 = **1 080 msgid**, soit environ
8 600 chaînes. C'est un travail de traduction, pas un travail de programmation, et il peut se
faire document par document sans rien casser — chaque `msgstr` rempli apparaît en ligne à la
poussée suivante.

## Traduire

```bash
# 1. régénérer les catalogues après toute modification d'une page
.venv-doc/bin/python site/build.py --pot --langue fr

# 2. écrire les traductions dans site/translations/<langue>.json
#    (une liste par document, dans l'ordre des msgid du catalogue)

# 3. les verser dans les catalogues
python3 site/po-fill.py site/locale/de/LC_MESSAGES site/translations/de.json
```

`po-fill.py` édite les catalogues **comme du texte** et ne touche qu'aux entrées demandées : ni
`msgcat`, ni Babel, ni polib, qui réécrivent tous les retours à la ligne et noient le vrai
changement dans un diff de plusieurs milliers de lignes. Il refuse une liste dont la longueur ne
correspond pas au catalogue — un décalage silencieux mettrait la bonne phrase au mauvais endroit
dans huit langues à la fois.

## Les documents longs

`comprendre_le_moteur.md`, `etude_formats.md` et `specification.md` restent dans `docs/`, qui est
leur source canonique dans le dépôt. `build.py` les recopie ici avant la construction ; les copies
sont ignorées par git.
