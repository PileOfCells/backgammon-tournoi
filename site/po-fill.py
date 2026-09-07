#!/usr/bin/env python3
"""Remplit les msgstr vides d'un catalogue à partir d'un fichier JSON.

    site/po-fill.py site/locale/en/LC_MESSAGES/index.po traductions.json
    site/po-fill.py site/locale/en/LC_MESSAGES traductions_en.json

La première forme prend un fichier JSON {msgid: msgstr} et un catalogue. La seconde prend un
dossier de catalogues et un JSON {document: …} : c'est la forme commode pour traduire une langue
entière d'un coup.

Les traductions d'un document se donnent soit par msgid, soit par une LISTE dans l'ordre du
catalogue. La liste évite de recopier chaque msgid français dans le fichier de traduction, ce qui
le doublerait en taille pour rien ; en contrepartie elle doit avoir exactement autant d'entrées
que le catalogue a de msgid non vides, faute de quoi le remplissage est refusé — un décalage
silencieux mettrait la bonne phrase au mauvais endroit dans huit langues à la fois. Une entrée
vide dans la liste laisse la chaîne non traduite, donc en français.

Le catalogue est édité COMME DU TEXTE, et seules les entrées demandées sont touchées : ni
msgcat, ni Babel, ni polib, qui réécrivent tous les retours à la ligne et noient le vrai
changement dans un diff de plusieurs milliers de lignes.

Une entrée déjà traduite n'est pas écrasée. Un msgid absent du catalogue est signalé — c'est
presque toujours le signe que la source a bougé sans que les catalogues aient été régénérés.
"""

import json
import pathlib
import sys


def lire_msgid(lignes, i):
    """Lit le msgid qui commence à la ligne i ; renvoie (texte, index après le msgid)."""
    parts = []
    ligne = lignes[i][len("msgid "):].strip()
    parts.append(json.loads(ligne))
    i += 1
    while i < len(lignes) and lignes[i].startswith('"'):
        parts.append(json.loads(lignes[i].strip()))
        i += 1
    return "".join(parts), i


def echapper(texte):
    """Une chaîne po sur plusieurs lignes si elle en contient, comme le fait xgettext."""
    if "\n" not in texte:
        return [json.dumps(texte, ensure_ascii=False)]
    morceaux = texte.split("\n")
    out = ['""']
    for k, m in enumerate(morceaux):
        if k < len(morceaux) - 1:
            m += "\n"
        elif m == "":
            continue
        out.append(json.dumps(m, ensure_ascii=False))
    return out


def msgids(chemin):
    """Les msgid non vides du catalogue, dans l'ordre."""
    lignes = chemin.read_text(encoding="utf-8").split("\n")
    out, i = [], 0
    while i < len(lignes):
        if lignes[i].startswith("msgid "):
            m, i = lire_msgid(lignes, i)
            if m:
                out.append(m)
        else:
            i += 1
    return out


def remplir(chemin, trad):
    if isinstance(trad, list):
        ids = msgids(chemin)
        if len(trad) != len(ids):
            sys.exit(f"{chemin} : {len(trad)} traductions pour {len(ids)} msgid — "
                     "régénérer les catalogues et refaire la liste")
        trad = {m: t for m, t in zip(ids, trad)}
    lignes = chemin.read_text(encoding="utf-8").split("\n")
    out, i, remplies, vus = [], 0, 0, set()
    while i < len(lignes):
        if not lignes[i].startswith("msgid "):
            out.append(lignes[i])
            i += 1
            continue
        debut = i
        msgid, i = lire_msgid(lignes, i)
        vus.add(msgid)
        # le msgstr suit immédiatement
        if i >= len(lignes) or not lignes[i].startswith("msgstr "):
            out.extend(lignes[debut:i])
            continue
        j = i + 1
        while j < len(lignes) and lignes[j].startswith('"'):
            j += 1
        actuel = "".join(json.loads(l.strip()) for l in
                         [lignes[i][len("msgstr "):].strip()] + lignes[i + 1:j])
        out.extend(lignes[debut:i])
        if actuel or msgid not in trad or not trad[msgid]:
            out.extend(lignes[i:j])
        else:
            morceaux = echapper(trad[msgid])
            out.append("msgstr " + morceaux[0])
            out.extend(morceaux[1:])
            remplies += 1
        i = j
    chemin.write_text("\n".join(out), encoding="utf-8")
    manquants = [m for m in trad if m not in vus]
    if manquants:
        print(f"  ! {len(manquants)} msgid absents de {chemin.name} (source modifiée ?)")
        for m in manquants[:3]:
            print(f"    {m[:70]!r}")
    print(f"  {chemin} : {remplies} entrées remplies")


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    cible = pathlib.Path(sys.argv[1])
    trad = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
    if cible.is_dir():
        for doc, entrees in sorted(trad.items()):
            po = cible / f"{doc}.po"
            if not po.exists():
                print(f"  ! catalogue absent : {po}")
                continue
            remplir(po, entrees)
    else:
        remplir(cible, trad)


if __name__ == "__main__":
    main()
