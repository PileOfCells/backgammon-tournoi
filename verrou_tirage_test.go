package tournoi_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tournoi "github.com/PileOfCells/backgammon-tournoi"
)

// tableauTire : un tableau de 8 joueurs dont le tirage est fait, rien de lancé.
func tableauTire(t *testing.T, cfg tournoi.Config) *tournoi.State {
	t.Helper()
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	st := tournoiDeTest(t, cfg, 8)
	for _, a := range st.ProposeAt(now) {
		if a.Kind != tournoi.ActDraw {
			t.Fatalf("un tirage attendu, %s", a)
		}
		ev, err := st.EventFromAction(a, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	if !st.Phases[0].Drawn {
		t.Fatal("le tableau devrait être tiré")
	}
	return st
}

func tableauSeul() tournoi.Config {
	return tournoi.Config{Name: "T", Phases: []tournoi.PhaseConfig{{Kind: tournoi.KindBracket, Length: 7}}}
}

// TestConsolanteApresLeTirageRefusee (N16) : cocher la consolante d'un tableau déjà tiré ne peut
// plus rien construire — le graphe est fait. La configuration est refusée, en nommant la phase
// et le champ, au lieu d'être acceptée sans effet.
func TestConsolanteApresLeTirageRefusee(t *testing.T) {
	for _, champ := range []string{"consolation", "last_chance", "reconciliation", "recharge", "seeding"} {
		t.Run(champ, func(t *testing.T) {
			avant := tableauSeul()
			if champ == "last_chance" || champ == "reconciliation" || champ == "recharge" {
				avant.Phases[0].Consolation = true
			}
			if champ == "recharge" {
				avant.Phases[0].Reconciliation = true
			}
			st := tableauTire(t, avant)
			next := st.Config
			next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
			switch champ {
			case "consolation":
				next.Phases[0].Consolation = true
			case "last_chance":
				next.Phases[0].LastChance = true
			case "reconciliation":
				next.Phases[0].Reconciliation = true
			case "recharge":
				next.Phases[0].Recharge = true
			case "seeding":
				next.Phases[0].Seeding = tournoi.SeedingRating
			}
			err := st.CheckConfig(next)
			if err == nil {
				t.Fatalf("%s après le tirage : CheckConfig doit refuser", champ)
			}
			var refus *tournoi.ConfigRefusal
			if !errors.As(err, &refus) {
				t.Fatalf("refus non typé : %T %v", err, err)
			}
			if refus.Phase != 0 || refus.Field != champ || refus.Reason != tournoi.RefusalDrawn {
				t.Errorf("refus %+v, attendu phase 0, champ %s, raison drawn", refus, champ)
			}
			if !strings.Contains(err.Error(), champ) {
				t.Errorf("le message ne nomme pas le champ : %v", err)
			}
			if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err == nil {
				t.Fatal("Apply doit refuser aussi")
			}
			if champ == "consolation" && st.Config.Phases[0].Consolation {
				t.Error("la configuration a changé malgré le refus")
			}
		})
	}
}

// TestConsolanteAvantLeTirageAcceptee : avant le tirage, rien n'est figé, et la consolante est
// construite au tirage.
func TestConsolanteAvantLeTirageAcceptee(t *testing.T) {
	st := tournoiDeTest(t, tableauSeul(), 8)
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Consolation = true
	if err := st.CheckConfig(next); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply(tournoi.ConfigChangedEvent(next, st.Last)); err != nil {
		t.Fatal(err)
	}
	for _, a := range st.Propose() {
		ev, _ := st.EventFromAction(a, st.Last)
		if err := st.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.Phases[0].Sections) < 2 {
		t.Fatalf("la consolante devait être construite au tirage : %d section(s)", len(st.Phases[0].Sections))
	}
}

// TestVerrouDuTirageEtVieuxJournaux : un journal écrit avant le verrou (version 1) qui cochait la
// consolante après le tirage se rejoue comme il a été joué — accepté, sans effet.
func TestVerrouDuTirageEtVieuxJournaux(t *testing.T) {
	st := tableauTire(t, tableauSeul())
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Phases[0].Consolation = true
	ev := tournoi.ConfigChangedEvent(next, st.Last)
	ev.Version = 1
	if err := st.Apply(ev); err != nil {
		t.Fatalf("un journal de version 1 doit se rejouer : %v", err)
	}
	if len(st.Phases[0].Sections) != 1 {
		t.Error("un vieux journal rejoué ne construit pas de consolante après coup")
	}
}

// TestAutresChampsApresLeTirage : ce qui garde un effet après le tirage — les tables, la
// dotation, les pauses — reste modifiable.
func TestAutresChampsApresLeTirage(t *testing.T) {
	st := tableauTire(t, tableauSeul())
	next := st.Config
	next.Phases = append([]tournoi.PhaseConfig(nil), next.Phases...)
	next.Tables = tournoi.Tables{Count: 4, Unavailable: []int{2}}
	if err := st.CheckConfig(next); err != nil {
		t.Fatal(err)
	}
}
