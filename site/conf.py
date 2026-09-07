# Configuration Sphinx du site de Nicomaque.
#
# Le site est publié en NEUF langues : le français est la source, les huit autres sont des
# catalogues gettext (site/locale/<langue>/LC_MESSAGES/*.po). Une chaîne non traduite retombe
# sur le français — c'est le comportement natif de gettext, et c'est ce qu'on veut : mieux vaut
# une page à moitié française qu'une page absente.
#
# Les documents longs (spécification, étude, présentation) sont les fichiers Markdown de docs/,
# qui restent la source canonique dans le dépôt. build.py les recopie ici avant la construction.

import datetime

project = "Nicomaque"
author = "Nicolas Harmand"
copyright = f"{datetime.date.today().year}, {author}"
release = "0.2.0"

extensions = ["myst_parser"]
myst_enable_extensions = ["deflist", "colon_fence"]
myst_heading_anchors = 3

source_suffix = {".md": "markdown", ".rst": "restructuredtext"}
master_doc = "index"
exclude_patterns = ["_build", "locale", "README.md"]

# --- Traduction ---
language = "fr"
locale_dirs = ["locale/"]
gettext_compact = False  # un catalogue par document : les diffs restent lisibles
gettext_uuid = False

LANGUES = {
    "fr": "Français",
    "en": "English",
    "de": "Deutsch",
    "el": "Ελληνικά",
    "es": "Español",
    "fi": "Suomi",
    "it": "Italiano",
    "ja": "日本語",
    "ru": "Русский",
}

# --- Rendu ---
html_theme = "furo"
html_title = "Nicomaque"
html_short_title = "Nicomaque"
html_static_path = []
html_copy_source = False
html_show_sphinx = False
html_baseurl = "https://pileofcells.github.io/backgammon-tournoi/"
templates_path = ["_templates"]
html_sidebars = {
    "**": [
        "sidebar/brand.html",
        "sidebar/search.html",
        "sidebar/scroll-start.html",
        "sidebar/navigation.html",
        "sidebar/langues.html",
        "sidebar/scroll-end.html",
    ]
}
html_context = {"langues": LANGUES}
