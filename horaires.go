package tournoi

import "time"

// Deux règles de temps : les micro-rondes, contre la manipulation ; les pauses, pour le repas.
//
// Le moteur n'a pas d'horloge. Le temps entre par ProposeAt(now) — l'hôte donne la sienne — ou,
// à défaut, par l'horodatage du dernier événement du journal (Propose()). C'est la même
// discipline que pour le reste : rien n'est lu ailleurs que dans le journal ou dans ce que
// l'appelant fournit.

// TimeRange est un créneau de la journée : le repas, la remise des prix, la fermeture de la
// salle. Start incluse, End exclue.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// intersects : le créneau [a, b] rencontre la pause.
func (r TimeRange) intersects(a, b time.Time) bool {
	return a.Before(r.End) && b.After(r.Start)
}

// batchDeadline : l'échéance du prochain lot d'appariements, DÉRIVÉE du journal.
//
// L'appariement au fil de l'eau rend le suisse continu manipulable par l'heure d'annonce d'un
// résultat : celui qui finit à 14 h 03 choisit son adversaire en annonçant à 14 h 04 ou à
// 14 h 20. Les micro-rondes ferment cette porte — les joueurs libres attendent l'échéance, et
// tous ceux d'un même groupe de défaites sont appariés d'un coup.
//
// L'horodatage du dernier lot n'est stocké nulle part : c'est le départ du dernier match lancé
// dans la phase. Un lot est exactement cela, un paquet de matchs lancés ensemble ; le déduire
// évite un champ d'état de plus à tenir juste après une correction ou une annulation.
//
// Le premier lot ne s'attend pas : sans match lancé, il n'y a pas de lot précédent.
func (s *State) batchDeadline(ph *PhaseState) (time.Time, bool) {
	if ph.Cfg.BatchMinutes <= 0 {
		return time.Time{}, false
	}
	var dernier time.Time
	for _, id := range s.MatchOrder {
		m := s.Matches[id]
		if m.Phase == ph.Index && m.Status != Cancelled && m.Start.After(dernier) {
			dernier = m.Start
		}
	}
	if dernier.IsZero() {
		return time.Time{}, false
	}
	return dernier.Add(time.Duration(ph.Cfg.BatchMinutes) * time.Minute), true
}

// flagBreaks marque les propositions dont le match rencontrerait une pause.
//
// RIEN N'EST BLOQUÉ. Le directeur sait des choses que le moteur ignore — que ces deux-là
// mangeront après, que la pause est indicative, que la salle ferme de toute façon. On le lui
// dit, il décide. C'est la même règle que pour tous les avertissements du moteur.
//
// L'avertissement tombe dès que le match RENCONTRE la pause, et pas seulement s'il s'y termine :
// un match lancé à midi moins dix et attendu pour 13 h 30 fait manquer le repas tout autant.
func (s *State) flagBreaks(acts []Action, now time.Time) {
	if len(s.Config.Breaks) == 0 {
		return
	}
	for i := range acts {
		if acts[i].Kind != ActStartMatch || acts[i].Length <= 0 {
			continue
		}
		fin := now.Add(s.Expected(acts[i].Length))
		for _, b := range s.Config.Breaks {
			if b.intersects(now, fin) {
				acts[i].Warn = WarnEndsInBreak
				break
			}
		}
	}
}
