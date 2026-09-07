package render

// Feuille de style injectée, et crédit du pied de page.
//
// Une page produite ici doit s'ouvrir sur un mur, hors ligne, depuis une clé USB : un seul
// fichier, CSS embarqué, aucune ressource externe — pas de police distante, pas de script, pas
// d'image liée. C'est pourquoi la feuille de style est une chaîne, et non un lien.
//
// L'hôte remplace DefaultStyle par la sienne pour retrouver sa charte (Renderer.Style). Les
// classes sont donc un contrat : les changer casse la feuille de l'hôte.

// DefaultStyle : une feuille sobre, lisible de loin sur un écran de salle et correcte à
// l'impression. Les couleurs sont neutres : c'est l'hôte qui a une charte, pas le moteur.
const DefaultStyle = `
:root{--trait:#c8c8c8;--doux:#f6f6f4;--texte:#1c1c1c;--faible:#666;--fait:#eef5ee;--cours:#fff7dd;--alerte:#b00}
*{box-sizing:border-box}
body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;margin:16px;color:var(--texte);line-height:1.4}
h1{font-size:1.6rem;margin:0 0 4px}
h2{font-size:1.2rem;margin:28px 0 8px;border-bottom:2px solid var(--trait);padding-bottom:2px}
h3{font-size:1rem;margin:16px 0 6px}
.resume{color:var(--faible);margin:0 0 8px}
table{border-collapse:collapse;margin:0 0 8px}
td,th{border:1px solid var(--trait);padding:3px 8px;text-align:left;vertical-align:top}
th{background:var(--doux);font-weight:600}
.lent{color:var(--alerte);font-weight:600}
.actions{margin:0;padding-left:20px}
.actions li{margin:2px 0}
.actions .alerte{color:var(--alerte)}
.vies{display:flex;gap:16px;align-items:flex-start;flex-wrap:wrap}
.vies>div{flex:1 1 260px}
.arbre{overflow-x:auto;margin:8px 0}
.tables{display:flex;flex-wrap:wrap;gap:6px;margin:0 0 8px;padding:0;list-style:none}
.tables li{border:1px solid var(--trait);border-radius:4px;padding:4px 8px;min-width:150px}
.tables .libre{background:var(--doux);color:var(--faible)}
.tables .hs{background:repeating-linear-gradient(45deg,#eee,#eee 4px,#fff 4px,#fff 8px);color:var(--faible)}
.tables .occupee{background:var(--cours)}
.tables .num{font-weight:700;margin-right:6px}
.appariements td.case{width:70px}
.credit{margin-top:24px;padding-top:6px;border-top:1px solid var(--trait);color:var(--faible);font-size:.85rem}
@media print{
  @page{size:A4;margin:12mm}
  body{margin:0;font-size:10pt}
  h1{font-size:14pt}
  h2{font-size:12pt;margin:8pt 0 4pt}
  td,th{padding:2pt 4pt}
  .appariements td.case{height:22pt}
  .credit{font-size:8pt}
}
`

// DefaultCredit : le crédit du pied de page. Le moteur porte un nom et un auteur ; les pages
// qu'il produit le disent, c'est la contrepartie d'un logiciel donné.
const DefaultCredit = `Nicomaque — moteur de tournoi de Nicolas Harmand`
