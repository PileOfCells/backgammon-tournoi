package rules

import "math/rand"

// ---- L'instantané d'une phase : l'entrée commune de Pair, Rank et RemainingMatches ----

// Format est le genre d'une phase. Les valeurs sont celles de la configuration du moteur.
type Format string

const (
	FormatSwissLives   Format = "swiss_lives"   // suisse à vies, continu ou par rondes
	FormatLivesBracket Format = "lives_bracket" // tableau à exemptions : 2 vies = exempt du premier tour
	FormatGSL          Format = "gsl"           // blocs de groupes GSL, 2 vies
	FormatBracket      Format = "bracket"       // élimination simple, consolante, dernière chance, double élimination
	FormatRoundRobin   Format = "round_robin"   // poules toutes rondes, barrage entre ex æquo
)

// Valeurs de PhaseRules.
const (
	ModeRounds    = "rounds" // suisse par rondes ; vide : continu
	PairingWins   = "wins"   // suisse : à défaites égales, les plus victorieux ensemble
	SeedingRating = "rating" // tableau : placement classique par cote
)

// PhaseRules : la configuration d'une phase, ce que les règles en lisent. Les champs inutiles à
// un format sont ignorés ; les défauts sont déjà remplis par l'appelant.
type PhaseRules struct {
	Lives          int    `json:"lives,omitempty"` // suisse : vies à l'entrée
	Length         int    `json:"length"`          // longueur configurée des matchs
	FinalLength    int    `json:"final_length,omitempty"`
	Lengths        []int  `json:"lengths,omitempty"`        // tableau : du dernier tour vers le premier
	LengthLate     int    `json:"length_late,omitempty"`    // suisse : longueur de la fin de phase…
	LateThreshold  int    `json:"late_threshold,omitempty"` // …quand il reste au plus ce nombre de joueurs en vie
	Mode           string `json:"mode,omitempty"`
	Pairing        string `json:"pairing,omitempty"`
	AvoidClubs     bool   `json:"avoid_clubs,omitempty"`
	AllowRematch   bool   `json:"allow_rematch,omitempty"`
	Target         int    `json:"target,omitempty"` // suisse, GSL : la phase s'arrête quand Σ vies ≤ Target
	Consolation    bool   `json:"consolation,omitempty"`
	LastChance     bool   `json:"last_chance,omitempty"`
	Reconciliation bool   `json:"reconciliation,omitempty"`
	Recharge       bool   `json:"recharge,omitempty"`
	GroupSize      int    `json:"group_size,omitempty"`
	Qualifiers     int    `json:"qualifiers,omitempty"`
	Seeding        string `json:"seeding,omitempty"`
}

// Entrant : un joueur de la phase et son parcours dans la phase.
type Entrant struct {
	Player    Ref     `json:"player"`
	Lives     int     `json:"lives"` // vies à l'entrée de la phase
	Wins      int     `json:"wins,omitempty"`
	Losses    int     `json:"losses,omitempty"`
	Byes      int     `json:"byes,omitempty"`
	Opponents []Ref   `json:"opponents,omitempty"` // adversaires déjà rencontrés, avec répétitions
	Club      string  `json:"club,omitempty"`
	Rating    float64 `json:"rating,omitempty"` // PR, plus bas = meilleur ; 0 = inconnu
}

// Phase est l'instantané d'une phase : tout ce que les règles d'appariement et de classement
// en lisent. L'hôte le construit depuis son propre état ; rien n'y est conservé d'un appel à
// l'autre.
type Phase struct {
	Format Format     `json:"format"`
	Name   string     `json:"name,omitempty"` // nom de la phase : sous-libellé du tirage d'un tableau
	Rules  PhaseRules `json:"rules"`
	// Entrants : dans l'ordre d'entrée dans la phase, qui est l'ordre des classements à égalité
	// et celui des joueurs « en vie ».
	Entrants []Entrant `json:"entrants"`
	// Eliminated : les joueurs éliminés de la phase, dans l'ordre (le dernier est finaliste).
	Eliminated []Ref `json:"eliminated,omitempty"`
	// Withdrawn : les retirés du tournoi. Un retiré n'a plus de vie ; dans un graphe, il perd par
	// forfait le match qui l'attend.
	Withdrawn []Ref `json:"withdrawn,omitempty"`
	Drawn     bool  `json:"drawn,omitempty"`   // tableau, poules : tirage fait ; GSL : un bloc au moins
	Round     int   `json:"round,omitempty"`   // suisse par rondes : dernière ronde ouverte ; GSL : bloc courant
	Length    int   `json:"length"`            // longueur courante (un changement du directeur l'emporte)
	Running   int   `json:"running,omitempty"` // matchs en cours dans la phase
	// Sections : les graphes de la phase, résolus (Resolve).
	Sections []Section `json:"sections,omitempty"`
	// Busy : joueurs qui jouent un match en ce moment. Elsewhere : joueurs occupés hors de ce
	// tournoi (une autre épreuve de la salle) ; ils ne sont pas appariés en barrage.
	Busy      []Ref `json:"busy,omitempty"`
	Elsewhere []Ref `json:"elsewhere,omitempty"`
	// Free : suisse, les joueurs appariables maintenant — en vie, libres, présents —, dans
	// l'ordre des entrants. L'hôte le calcule : la disponibilité est la sienne.
	Free []Ref `json:"free,omitempty"`
	// Barrages : les matchs lancés (non annulés) des sections de barrage, avec leur section et,
	// une fois terminés, leur vainqueur.
	Barrages []Match `json:"barrages,omitempty"`
	// Repechages : poules, qualifié retiré → joueur repêché à sa place.
	Repechages map[Ref]Ref `json:"repechages,omitempty"`
}

// vue : l'instantané et ses index, construits une fois par appel.
type vue struct {
	*Phase
	idx       map[Ref]*Entrant
	withdrawn map[Ref]bool
}

func voir(p *Phase) *vue {
	v := &vue{Phase: p, idx: make(map[Ref]*Entrant, len(p.Entrants)), withdrawn: refSet(p.Withdrawn)}
	for i := range p.Entrants {
		v.idx[p.Entrants[i].Player] = &p.Entrants[i]
	}
	return v
}

func (v *vue) entrant(p Ref) Entrant {
	if e := v.idx[p]; e != nil {
		return *e
	}
	return Entrant{Player: p}
}

// remaining : vies restantes d'un joueur (0 s'il est retiré).
func (v *vue) remaining(p Ref) int {
	e := v.entrant(p)
	return RemainingLives(e.Lives, e.Losses, v.withdrawn[p])
}

// RemainingLives : les vies restantes d'un joueur entré avec lives vies et qui a perdu losses
// matchs ; un retiré n'en a plus.
func RemainingLives(lives, losses int, withdrawn bool) int {
	if withdrawn {
		return 0
	}
	if l := lives - losses; l > 0 {
		return l
	}
	return 0
}

// alive : les entrants encore en vie, dans l'ordre des entrants.
func (v *vue) alive() []Ref {
	var out []Ref
	for _, e := range v.Entrants {
		if v.remaining(e.Player) > 0 {
			out = append(out, e.Player)
		}
	}
	return out
}

// livesLeft : somme des vies restantes.
func (v *vue) livesLeft() int {
	n := 0
	for _, e := range v.Entrants {
		n += v.remaining(e.Player)
	}
	return n
}

// LivesLeft : la somme des vies restantes des entrants — la grandeur qui décide de la fin d'un
// suisse ou d'un GSL à bascule (Σ vies ≤ Target).
func LivesLeft(p Phase) int { return voir(&p).livesLeft() }

// Alive : les entrants encore en vie, dans l'ordre des entrants.
func Alive(p Phase) []Ref { return voir(&p).alive() }

// Seed : ce dont dépend le hasard d'un appariement. Le tirage d'une proposition dépend de la
// graine du tournoi, du nombre d'événements déjà appliqués et de la phase ; celui d'une ronde de
// suisse, de la graine, de la phase et du numéro de ronde seulement — confirmer le premier match
// d'une ronde ne redistribue pas les suivants.
type Seed struct {
	Base   int64 `json:"base"`
	Events int   `json:"events"`
	Phase  int   `json:"phase"`
}

// Draw : la graine d'une proposition.
func (s Seed) Draw() int64 { return s.Base*1000003 + int64(s.Events)*7919 + int64(s.Phase) }

// Round : la graine de la ronde r d'un suisse.
func (s Seed) Round(r int) int64 {
	return s.Base*1000003 + int64(s.Phase)*7919 + int64(r)*104729
}

func generateur(graine int64) *rand.Rand { return rand.New(rand.NewSource(graine)) }

// PhaseDone : la phase n'a plus rien à jouer. Suisse et GSL : aucun match en cours et
// Σ vies ≤ Target, ou un seul joueur en vie (GSL : le bloc courant terminé). Tableau : tiré et
// tous ses nœuds terminés. Poules : de plus, chaque barrage a rendu ses places.
func PhaseDone(p Phase) bool {
	v := voir(&p)
	switch p.Format {
	case FormatSwissLives:
		return v.swissDone()
	case FormatGSL:
		return v.gslDone()
	case FormatRoundRobin:
		return p.Drawn && AllDone(p.Sections) && v.barragesDone()
	}
	return p.Drawn && AllDone(p.Sections)
}

// PairingKind : ce qu'une proposition d'appariement demande de confirmer.
type PairingKind string

const (
	PairMatch PairingKind = "match" // lancer un match entre A et B
	PairBye   PairingKind = "bye"   // exempter A de la ronde (suisse par rondes)
	PairDraw  PairingKind = "draw"  // tirer un tableau (Slots) ou des groupes (Groups)
)

// Pairing est une PROPOSITION : une paire à lancer, une exemption ou un tirage. Ce n'est pas un
// match (Match) : rien n'est lancé tant que le directeur ne l'a pas confirmé, et l'hôte en fait
// alors son propre match, avec sa table et son heure.
type Pairing struct {
	Kind    PairingKind `json:"kind"`
	Section string      `json:"section,omitempty"` // section du graphe, du barrage ou du tirage de barrage
	Key     string      `json:"key,omitempty"`     // nœud du graphe (Node.Key)
	Label   Label       `json:"label"`
	Round   int         `json:"round,omitempty"` // suisse par rondes : la ronde
	A       Ref         `json:"a,omitempty"`
	B       Ref         `json:"b,omitempty"`
	Length  int         `json:"length,omitempty"`
	Slots   []Ref       `json:"slots,omitempty"`  // tirage d'un tableau : places du premier tour (Bye = exemption)
	Groups  [][]Ref     `json:"groups,omitempty"` // tirage de groupes : GSL, poules, barrage
}

// Pair : les appariements à proposer maintenant pour la phase — les paires, leurs longueurs,
// les exemptions, ou le tirage à faire d'abord. Vide quand il n'y a rien à apparier (matchs en
// cours, phase terminée).
//
// Suisse par rondes : Pair apparie la ronde SUIVANTE (Round+1), sur Free. Tant qu'une ronde est
// ouverte, ce qui en reste à confirmer est l'affaire de l'hôte, qui connaît les joueurs appelés
// et ceux déjà engagés : PairSwissRound sur l'état d'avant la ronde, puis PairSwissOrphans.
func Pair(p Phase, seed Seed) []Pairing {
	v := voir(&p)
	switch p.Format {
	case FormatSwissLives:
		return v.pairSwiss(seed)
	case FormatGSL:
		return v.pairGSL(seed)
	case FormatBracket, FormatLivesBracket:
		return v.pairBracket(seed)
	case FormatRoundRobin:
		return v.pairPools(seed)
	}
	return nil
}
