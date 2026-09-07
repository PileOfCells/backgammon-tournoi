#!/usr/bin/env python3
"""Construit le site de Nicomaque dans les neuf langues.

    .venv-doc/bin/python site/build.py            # construit tout dans site/_site/
    .venv-doc/bin/python site/build.py --pot      # régénère les catalogues .pot puis les .po
    .venv-doc/bin/python site/build.py --langue fr  # une seule langue, pour aller vite

Le français est la source ; les huit autres langues sont des catalogues gettext. Une chaîne non
traduite retombe sur le français, ce qui est le comportement voulu : mieux vaut une page à moitié
française qu'une page absente.

Les trois documents longs — la présentation, l'étude et la spécification — restent dans docs/ au
format Markdown, qui est leur source canonique dans le dépôt. Ils sont RECOPIÉS ici avant la
construction plutôt qu'inclus : une copie se relit, se traduit et se diffe comme n'importe quelle
page, là qu'une inclusion hors du dossier source est un cas particulier de plus.
"""

import argparse
import pathlib
import shutil
import subprocess
import sys

RACINE = pathlib.Path(__file__).resolve().parent
DEPOT = RACINE.parent
SORTIE = RACINE / "_site"
BUILD = RACINE / "_build"

# Documents recopiés depuis docs/ ; le nom du fichier devient le nom du document du site.
RECOPIES = ["comprendre_le_moteur.md", "etude_formats.md", "specification.md"]

sys.path.insert(0, str(RACINE))
from conf import LANGUES  # noqa: E402


def recopier():
    """Amène les documents longs de docs/ dans le dossier source du site."""
    for nom in RECOPIES:
        src = DEPOT / "docs" / nom
        if not src.exists():
            sys.exit(f"document manquant : {src}")
        shutil.copyfile(src, RACINE / nom)


def sphinx(*args):
    cmd = [sys.executable, "-m", "sphinx", *args]
    if subprocess.run(cmd, cwd=RACINE).returncode:
        sys.exit(f"échec : {' '.join(cmd)}")


def catalogues():
    """Régénère les .pot puis fusionne dans les .po des huit langues traduites."""
    sphinx("-b", "gettext", "-q", ".", str(BUILD / "gettext"))
    langues = [c for c in LANGUES if c != "fr"]
    cmd = [sys.executable, "-m", "sphinx_intl", "update",
           "-p", str(BUILD / "gettext"), "-d", "locale"]
    for c in langues:
        cmd += ["-l", c]
    if subprocess.run(cmd, cwd=RACINE).returncode:
        sys.exit("échec de sphinx-intl update")


def construire(code):
    dest = SORTIE / code
    sphinx("-b", "html", "-q", "-D", f"language={code}",
           "-d", str(BUILD / "doctrees" / code), ".", str(dest))


def racine_html():
    """Une page d'accueil qui redirige vers le français et liste les neuf langues.

    Le site n'a pas de langue « par défaut » côté serveur : GitHub Pages sert des fichiers. On
    redirige donc vers le français, qui est la source et la seule langue complète, en laissant
    les huit autres à un clic.
    """
    liens = "\n".join(
        f'    <li><a href="{c}/" hreflang="{c}">{n}</a></li>' for c, n in LANGUES.items())
    (SORTIE / "index.html").write_text(f"""<!doctype html>
<html lang="fr"><head><meta charset="utf-8">
<meta http-equiv="refresh" content="0; url=fr/">
<title>Nicomaque</title>
<style>body{{font-family:system-ui,sans-serif;margin:3rem auto;max-width:32rem;line-height:1.5}}</style>
</head><body>
<h1>Nicomaque</h1>
<p>Moteur de tournoi de backgammon. <a href="fr/">Aller au site</a>.</p>
<ul>
{liens}
</ul>
</body></html>
""", encoding="utf-8")
    (SORTIE / ".nojekyll").write_text("", encoding="utf-8")


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--pot", action="store_true", help="régénérer les catalogues avant de construire")
    ap.add_argument("--langue", action="append", help="ne construire que cette langue")
    args = ap.parse_args()

    recopier()
    if args.pot:
        catalogues()
    codes = args.langue or list(LANGUES)
    shutil.rmtree(SORTIE, ignore_errors=True)
    for code in codes:
        if code not in LANGUES:
            sys.exit(f"langue inconnue : {code}")
        print(f"  {code}")
        construire(code)
    racine_html()
    print(f"site construit dans {SORTIE}")


if __name__ == "__main__":
    main()
