package game

import "image"

// Autopilot is an optional player that the game loop consults every tick.
// The game knows nothing about how it decides; see internal/autopilot.
type Autopilot interface {
	// Input may toggle the autopilot and replace the player's input for
	// this tick. It runs before the game steps.
	Input(v View, in Input) Input
	// Observe sees the game after it has stepped.
	Observe(v View)
	// Event reports an outcome: a cannon hit, or how a shot ended.
	Event(ev Event, fromPilot bool)
	// HUD returns status lines drawn in the top-right and bottom-right
	// corners; empty strings draw nothing.
	HUD() (top, bottom string)
}

var autopilot Autopilot

// SetAutopilot plugs in an autopilot. Call it before Start.
func SetAutopilot(a Autopilot) { autopilot = a }

// View is a read-only snapshot of the game for an autopilot. It holds
// copies, so reading it never races with the game loop.
type View struct {
	State           State
	Paused          bool
	CannonExploding bool

	Cannon     image.Rectangle
	BeamActive bool
	Bombs      []image.Rectangle
	Aliens     []Alien           // alive aliens only
	Shields    []image.Rectangle // bounds of shields with material left

	UFOActive bool
	UFO       image.Rectangle
	UFODir    int

	// formation timing, for predicting where the aliens will be
	Tick, NextMoveTick, MoveInterval, AlienDirection int
}

// Alien is one alive alien in a View.
type Alien struct {
	Rect   image.Rectangle
	Column int
}

// Sizes of the sprites an autopilot needs to reason about.
var (
	CannonSize = cannonSprite.Size()
	BeamSize   = beamSprite.Size()
)

// view takes a snapshot of the game.
func (g *Game) view() View {
	v := View{
		State:           g.state,
		Paused:          g.paused,
		CannonExploding: g.cannonExploding,
		Cannon:          g.cannon.rect(),
		BeamActive:      g.beamActive,
		UFOActive:       g.ufoActive,
		UFO:             g.ufo.rect(),
		UFODir:          g.ufoDir,
		Tick:            g.tick,
		NextMoveTick:    g.nextMoveTick,
		MoveInterval:    g.moveInterval,
		AlienDirection:  g.alienDirection,
	}
	for _, b := range g.bombs {
		v.Bombs = append(v.Bombs, b.rect())
	}
	for _, a := range g.aliens {
		if a.Status {
			v.Aliens = append(v.Aliens, Alien{Rect: a.rect(), Column: a.Column})
		}
	}
	for _, s := range g.shields {
		if s.alive() {
			v.Shields = append(v.Shields, s.bounds())
		}
	}
	return v
}
