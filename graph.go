package tournoi

// Les graphes de matchs — constructeurs, résolution, matchs prêts — sont dans rules (graphe.go
// de ce paquet-là) ; le moteur garde ses sections (Section, GMatch, sérialisées dans l'état) et
// les y fait passer (regles.go).

// gmatch retrouve le match de graphe (section, clé).
func (ph *PhaseState) gmatch(section, key string) *GMatch {
	if key == "" {
		return nil
	}
	for _, sec := range ph.Sections {
		if sec.Name != section {
			continue
		}
		for i := range sec.Matches {
			if sec.Matches[i].Key == key {
				return &sec.Matches[i]
			}
		}
	}
	return nil
}

func (ph *PhaseState) section(name string) *Section {
	for _, sec := range ph.Sections {
		if sec.Name == name {
			return sec
		}
	}
	return nil
}
