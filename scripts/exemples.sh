#!/usr/bin/env bash
# Régénère exemples/ : sept formats simulés, rendus à quatre stades, plus la galerie index.html.
#
# À relancer après tout changement du rendu (paquet render) ou des formats : les pages d'exemples
# sont la seule vitrine du moteur, et une vitrine périmée coûte plus qu'elle ne rapporte.
#
#   scripts/exemples.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# format:joueurs:titre — le titre est celui de la galerie.
CAS=(
  "suisse_tableau:32:Suisse 2 vies continu puis tableau à vies"
  "suisse:24:Suisse 2 vies continu"
  "gsl:24:Blocs GSL puis tableau à vies"
  "double:16:Double élimination avec recharge"
  "conso:24:Principal, consolante, dernière chance"
  "poules:16:Poules de 4 puis tableau"
  "elim:20:Élimination simple"
)

rm -rf exemples
mkdir -p exemples

index=exemples/index.html
cat > "$index" <<'HTML'
<!doctype html><html lang="fr"><head><meta charset="utf-8"><title>Exemples de tournois simulés</title><style>body{font-family:system-ui,sans-serif;margin:24px;max-width:1000px}li{margin:4px 0}h2{margin-top:28px}</style></head><body>
<h1>Exemples de tournois simulés</h1><p>Chaque tournoi est joué par le simulateur (résultats tirés au sort selon les PR). Les pages « affichage » sont celles qu'un écran de salle montrerait : actions à faire pour le TD, matchs en cours, grille des tables, tableau des vies, arbres, classement. La « feuille d'appariements » est celle qu'on imprime. Le journal JSON est celui que l'hôte stockerait.</p>
HTML

for cas in "${CAS[@]}"; do
  IFS=: read -r format n titre <<< "$cas"
  go run ./cmd/tournoi-demo -format "$format" -joueurs "$n" -etapes -sortie "exemples/$format" > /dev/null
  {
    echo "<h2>$titre ($n joueurs)</h2><ul>"
    echo "<li><a href=\"$format/etape1_affichage.html\">Étape 1 : Début (inscriptions, premiers appariements ou tirage)</a></li>"
    echo "<li><a href=\"$format/etape2_affichage.html\">Étape 2 : Premier tiers</a></li>"
    echo "<li><a href=\"$format/etape3_affichage.html\">Étape 3 : Deux tiers</a></li>"
    echo "<li><a href=\"$format/etape4_affichage.html\">Étape 4 : Fin (classement final)</a></li>"
    liens=""
    for svg in "exemples/$format"/phase*.svg; do
      [ -e "$svg" ] || continue
      base=$(basename "$svg" .svg)
      [ -n "$liens" ] && liens="$liens · "
      liens="$liens<a href=\"$format/$base.svg\">$base</a>"
    done
    [ -n "$liens" ] && echo "<li>Arbres finaux : $liens</li>"
    echo "<li><a href=\"$format/appariements.html\">feuille d'appariements</a> · <a href=\"$format/journal.json\">journal.json</a> · <a href=\"$format/classement.csv\">classement.csv</a></li></ul>"
  } >> "$index"
done

echo "</body></html>" >> "$index"
echo "exemples/ régénéré ($((${#CAS[@]})) formats)"
