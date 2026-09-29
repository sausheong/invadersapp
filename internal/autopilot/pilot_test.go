package autopilot

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/sausheong/invadersapp/internal/game"
)

// view returns a playing game with the cannon centred, nothing else on
// screen, and the formation standing still.
func view() game.View {
	x := (game.Width - game.CannonSize.X) / 2
	return game.View{
		State:        game.StatePlaying,
		Cannon:       image.Rect(x, game.CannonY, x+game.CannonSize.X, game.CannonY+game.CannonSize.Y),
		MoveInterval: 1000, NextMoveTick: 1000, AlienDirection: 1,
	}
}

// bombAt is a bomb whose left edge is x and bottom edge is bottom.
func bombAt(x, bottom int) image.Rectangle { return image.Rect(x, bottom-9, x+10, bottom) }

func alienAt(x, y, col int) game.Alien {
	return game.Alien{Rect: image.Rect(x, y, x+20, y+14), Column: col}
}

func shieldAt(x, y int) image.Rectangle { return image.Rect(x, y, x+22, y+16) }

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
	v := view()
	x := v.Cannon.Min.X
	v.Bombs = []image.Rectangle{bombAt(x+4, game.CannonY-100)} // lands on the cannon in 25 ticks
	f := newForecast(v)
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
	v := view()
	x := v.Cannon.Min.X
	v.Bombs = []image.Rectangle{bombAt(x+60, game.CannonY-40)} // lands 60px right in 10 ticks
	if s, _ := newForecast(v).safety(x, -1, 0, x+120); s != onWay {
		t.Errorf("crossing under the bomb: %v, want onWay", s)
	}
}

func TestSafetyIgnoresBombsBlockedByShield(t *testing.T) {
	v := view()
	v.Shields = []image.Rectangle{shieldAt(0, game.CannonY-40)}
	v.Bombs = []image.Rectangle{bombAt(4, game.CannonY-60)} // above the shield
	if s, _ := newForecast(v).safety(2, -1, 0, 2); s != safe {
		t.Errorf("under a shield: %v, want safe", s)
	}
}

func TestLineOfFireLeadsMovingFormation(t *testing.T) {
	v := view()
	x := v.Cannon.Min.X
	bx := x + game.BeamOffsetX
	// an alien well to the left, moving right 3px every tick
	v.Aliens = []game.Alien{alienAt(bx-49, 100, 0)}
	v.MoveInterval, v.NextMoveTick = 1, 1
	if got := newForecast(v).lineOfFire(x, 0); got != hitAlien {
		t.Errorf("moving formation: %v, want hitAlien", got)
	}
	v.MoveInterval, v.NextMoveTick = 1000, 1000 // standing still
	if got := newForecast(v).lineOfFire(x, 0); got != hitNothing {
		t.Errorf("still formation: %v, want hitNothing", got)
	}
	v.Shields = []image.Rectangle{shieldAt(x, game.CannonY-40)}
	if got := newForecast(v).lineOfFire(x, 0); got != hitShield {
		t.Errorf("under shield: %v, want hitShield", got)
	}
}

func TestObserveSpots(t *testing.T) {
	v := view()
	v.Bombs = []image.Rectangle{bombAt(v.Cannon.Min.X+4, game.CannonY-100)}
	o := observe(v, -1, 0)
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

func TestObserveOffersKeep(t *testing.T) {
	v := view()
	o := observe(v, v.Cannon.Min.X-80, 0)
	if o.Spots[1].Key != keepKey || !strings.HasPrefix(o.Spots[1].Text, "Keep heading") {
		t.Errorf("second spot should be keep: %+v", o.Spots[1])
	}
}

func TestSteerAndExpire(t *testing.T) {
	p := New()
	v := view()
	now := time.Now()
	x := v.Cannon.Min.X
	p.decision, p.pendingX, p.decidedAt = &action{}, x-40, now

	in := p.steer(v, game.Input{}, now)
	if !in.Left || in.Right {
		t.Errorf("heading left: got %+v", in)
	}
	v.Cannon = v.Cannon.Add(image.Pt(-40, 0))
	if in = p.steer(v, game.Input{}, now); in.Left || in.Right {
		t.Errorf("arrived: got %+v", in)
	}
	v.Cannon = v.Cannon.Add(image.Pt(40, 0))
	if in = p.steer(v, game.Input{}, now.Add(2*staleAfter)); in.Left {
		t.Error("stale decision should stop the cannon")
	}
}

func TestSteerDropsStaleFire(t *testing.T) {
	p := New()
	v := view()
	p.fire, p.fireAt = true, hitAlien // Jev was told an alien would be hit; none is there now
	if in := p.steer(v, game.Input{}, time.Now()); in.Fire {
		t.Error("stale fire decision should be dropped")
	}
	if p.stats.Stale != 1 {
		t.Errorf("Stale = %d, want 1", p.stats.Stale)
	}
	p.fire, p.fireAt = true, hitNothing
	if in := p.steer(v, game.Input{}, time.Now()); !in.Fire || !in.PilotFire {
		t.Error("fresh fire decision should fire")
	}
}

func TestAutoStart(t *testing.T) {
	p := New()
	p.enabled.Store(true)
	v := game.View{State: game.StateTitle}
	var in game.Input
	for i := 0; i < startDelay; i++ {
		in = p.Input(v, game.Input{})
	}
	if !in.Start {
		t.Error("autopilot should start a game after startDelay ticks on the title screen")
	}
}

func TestEscapeSpot(t *testing.T) {
	v := view()
	x := v.Cannon.Min.X
	f := newForecast(v)
	if ex, s := f.escapeSpot(x, -1); ex >= x || s != safe {
		t.Errorf("open field, run left: x=%d safety=%v", ex, s)
	}
	if ex, _ := newForecast(v).escapeSpot(0, -1); ex != -1 {
		t.Errorf("against the left wall: x=%d, want -1", ex)
	}
	// a bomb landing on the first spot left soon after the cannon would get
	// there makes it risky, so the escape goes one spot further
	v.Bombs = []image.Rectangle{bombAt(x-spotStep+4, game.CannonY-80)}
	if ex, s := newForecast(v).escapeSpot(x, -1); s != safe || ex >= x-spotStep {
		t.Errorf("bomb on the first spot: x=%d safety=%v, want a safe spot further left", ex, s)
	}
}

func TestSteerUsesPlannedEscape(t *testing.T) {
	p := New()
	v := view()
	x := v.Cannon.Min.X
	now := time.Now()
	p.decision, p.pendingX, p.pendingEsc, p.decidedAt = &action{}, x, 1, now
	p.steer(v, game.Input{}, now) // decision says stay, escape right if needed

	v.Bombs = []image.Rectangle{bombAt(x+4, game.CannonY-20)} // a new bomb, landing in 5 ticks
	in := p.steer(v, game.Input{}, now)
	if !in.Right || !p.escaping || p.stats.Escapes != 1 {
		t.Errorf("should run right on the planned escape: in=%+v escaping=%v", in, p.escaping)
	}
	p.steer(v, game.Input{}, now)
	if p.stats.Escapes != 1 {
		t.Error("an escape under way shouldn't be counted again")
	}
}

func TestObserveDescribesEscapes(t *testing.T) {
	v := view()
	v.Cannon = image.Rect(0, game.CannonY, game.CannonSize.X, game.CannonY+game.CannonSize.Y)
	o := observe(v, -1, 0)
	if !strings.Contains(o.Escape[escapeLeft], "wall") {
		t.Errorf("left escape against the wall: %q", o.Escape[escapeLeft])
	}
	if !strings.Contains(o.Escape[escapeRight], "safe spot") {
		t.Errorf("right escape in the open: %q", o.Escape[escapeRight])
	}
}
