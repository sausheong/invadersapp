//go:build jev

package main

import (
	"image"
	"strings"
	"testing"
	"time"
)

// bombAt places a bomb whose left edge is x and bottom edge is bottom.
func bombAt(x, bottom int) Sprite {
	return Sprite{size: bombSprite, Position: image.Pt(x, bottom-bombSprite.Dy()), Status: true}
}

// clearField removes aliens, shields and bombs so each test sets up only
// what it needs.
func clearField(g *Game) {
	g.aliens = nil
	g.shields = nil
	g.bombs = nil
}

func TestStepToward(t *testing.T) {
	if x := stepToward(100, 130, 5); x != 115 {
		t.Errorf("5 ticks right: x = %d, want 115", x)
	}
	if x := stepToward(100, 101, 5); x != 100 {
		t.Errorf("within 2px: x = %d, want 100 (no move)", x)
	}
	if x := stepToward(100, 95, 5); x != 95 {
		t.Errorf("overshoot: x = %d, want 95", x)
	}
	if x := stepToward(100, -1, 5); x != 100 {
		t.Errorf("no destination: x = %d, want 100", x)
	}
}

func TestSafetyStayVersusDodge(t *testing.T) {
	g := newPlayingGame(t)
	clearField(g)
	x := g.cannon.Position.X
	g.bombs = []Sprite{bombAt(x+4, cannonY-100)} // lands on the cannon in 25 ticks
	f := newForecast(g)

	if s, _ := f.safety(x, -1, 0, x); s != risky {
		t.Errorf("staying under the bomb: %v, want risky", s)
	}
	if s, _ := f.safety(x, -1, 0, x-60); s != safe {
		t.Errorf("moving 60px left: %v, want safe", s)
	}
	if s, _ := f.safety(x, -1, 30, x-60); s != doomed {
		t.Errorf("with 30 ticks of lag: %v, want doomed", s)
	}
}

func TestSafetyOnTheWay(t *testing.T) {
	g := newPlayingGame(t)
	clearField(g)
	x := g.cannon.Position.X
	g.bombs = []Sprite{bombAt(x+60, cannonY-40)} // lands 60px right in 10 ticks
	f := newForecast(g)
	if s, _ := f.safety(x, -1, 0, x+120); s != onWay {
		t.Errorf("crossing under the bomb: %v, want onWay", s)
	}
}

func TestSafetyIgnoresBombsBlockedByShield(t *testing.T) {
	g := newPlayingGame(t)
	clearField(g)
	g.shields = []*Shield{newShield(0, cannonY-40)}
	g.cannon.Position.X = 2
	g.bombs = []Sprite{bombAt(4, cannonY-60)} // above the shield
	if s, _ := newForecast(g).safety(2, -1, 0, 2); s != safe {
		t.Errorf("under a shield: %v, want safe", s)
	}
}

func TestLineOfFireLeadsMovingFormation(t *testing.T) {
	g := newPlayingGame(t)
	clearField(g)
	x := g.cannon.Position.X
	bx := x + 7
	// an alien well to the left, moving right 3px every tick
	g.aliens = []Sprite{createAlien(bx-49, 100, alien1Sprite, alien1aSprite, 30, 0)}
	g.alienDirection, g.moveInterval, g.nextMoveTick = 1, 1, g.tick+1
	if got := newForecast(g).lineOfFire(x, 0); got != hitAlien {
		t.Errorf("moving formation: %v, want hitAlien", got)
	}
	g.moveInterval, g.nextMoveTick = 1000, g.tick+1000 // formation standing still
	if got := newForecast(g).lineOfFire(x, 0); got != hitNothing {
		t.Errorf("still formation: %v, want hitNothing", got)
	}
	g.shields = []*Shield{newShield(x, cannonY-40)}
	if got := newForecast(g).lineOfFire(x, 0); got != hitShield {
		t.Errorf("under shield: %v, want hitShield", got)
	}
}

func TestObserveSpots(t *testing.T) {
	g := newPlayingGame(t)
	clearField(g)
	g.bombs = []Sprite{bombAt(g.cannon.Position.X+4, cannonY-100)}
	o := observe(g, -1, 0)
	if o.Spots[0].Key != stayKey || o.Spots[0].Safety == safe {
		t.Errorf("first spot should be an unsafe stay: %+v", o.Spots[0])
	}
	keys := map[string]bool{}
	safeSpots := 0
	for _, s := range o.Spots {
		if keys[s.Key] {
			t.Errorf("duplicate key %s", s.Key)
		}
		keys[s.Key] = true
		if s.Safety == safe {
			safeSpots++
		}
		if !strings.Contains(s.Text, "Safety: ") {
			t.Errorf("spot text missing safety: %q", s.Text)
		}
	}
	if safeSpots == 0 || len(o.Spots) < 15 {
		t.Errorf("got %d spots, %d safe", len(o.Spots), safeSpots)
	}
	if len(o.questions()["spot"].Criteria.(map[string]string)) != len(o.Spots) {
		t.Error("every spot should be a choice option")
	}
}

func TestPilotApplySteersAndExpires(t *testing.T) {
	p := autopilot{dest: -1}
	p.enabled.Store(true)
	g := newPlayingGame(t)
	clearField(g)
	now := time.Now()
	x := g.cannon.Position.X
	p.decision, p.pendingX, p.decidedAt = &action{}, x-40, now

	in := p.apply(g, Input{}, now)
	if !in.Left || in.Right {
		t.Errorf("heading left: got %+v", in)
	}
	g.cannon.Position.X = x - 40
	if in = p.apply(g, Input{}, now); in.Left || in.Right {
		t.Errorf("arrived: got %+v", in)
	}
	g.cannon.Position.X = x
	if in = p.apply(g, Input{}, now.Add(2*pilotStale)); in.Left {
		t.Error("stale decision should stop the cannon")
	}
}

func TestPilotDropsStaleFire(t *testing.T) {
	p := autopilot{dest: -1}
	p.enabled.Store(true)
	g := newPlayingGame(t)
	clearField(g)
	p.fire, p.fireAt = true, hitAlien // Jev was told an alien would be hit; none is there now
	if in := p.apply(g, Input{}, time.Now()); in.Fire {
		t.Error("stale fire decision should be dropped")
	}
	if p.stats.Stale != 1 {
		t.Errorf("Stale = %d, want 1", p.stats.Stale)
	}
	p.fire, p.fireAt = true, hitNothing
	if in := p.apply(g, Input{}, time.Now()); !in.Fire || !in.PilotFire {
		t.Error("fresh fire decision should fire")
	}
}
