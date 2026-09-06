// Package tournoi — moteur de tournoi de backgammon fondé sur un journal d'événements.
//
// # Intégration dans un logiciel hôte (Go, même processus)
//
// L'hôte conserve le journal (tournoi.Journal, un tableau JSON) et l'état courant peut être
// reconstruit à tout moment par Replay. Boucle type côté serveur :
//
//	cfg := tournoi.Config{Name: "Open", Phases: []tournoi.PhaseConfig{
//	    {Kind: tournoi.KindSwissLives, Length: 7, Target: 16},          // suisse 2 vies continu, bascule à Σvies = 16
//	    {Kind: tournoi.KindLivesBracket, Length: 9, FinalLength: 11},    // tableau à exemptions
//	}}
//	st, created, _ := tournoi.New(cfg, seed, time.Now())
//	journal := tournoi.Journal{created}
//	// inscriptions
//	ev := tournoi.Event{Kind: tournoi.EvPlayerAdded, Time: time.Now(), Player: &p}
//	st.Apply(ev); journal = append(journal, ev)
//	// boucle du TD
//	for _, a := range st.Propose() {            // « lancer P3 contre P8 en 7 points, table 4 », etc.
//	    ev, _ := st.EventFromAction(a, time.Now()) // le TD confirme
//	    st.Apply(ev); journal = append(journal, ev)
//	}
//	// résultat saisi par le TD
//	ev = tournoi.ResultEvent("M12", "P3", 7, 4, time.Now())
//	st.Apply(ev); journal = append(journal, ev)
//
// Corrections : ajouter un événement EvResultCorrected (jamais modifier le journal) ; l'état est
// recalculé et State.Warnings signale ce qui n'est plus cohérent (par exemple un match de tableau
// joué par le mauvais joueur). Forfaits : EvPlayerWithdrawn. Reprise après panne : Replay(journal).
//
// Affichage : le paquet render fournit les composants HTML/SVG (arbres, tableau des vies,
// matchs en cours, classement, page complète). Prévision de fin : sim.Forecast(journal, now, K, …).
// Import de joueurs : players.FromCSV.
//
// Déterminisme : Propose dépend seulement du journal (graine + nombre d'événements) ; le contenu
// des tirages est écrit dans les événements EvDraw, donc rejouer un journal ne dépend pas de
// l'algorithme d'appariement.
package tournoi
