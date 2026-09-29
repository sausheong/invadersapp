package autopilot

import (
	"fmt"
	"image"
	"strings"

	"github.com/sausheong/invadersapp/internal/game"
	"github.com/sausheong/invadersapp/internal/typesafe"
)

// This file turns a game.View into what Jev is asked about. Code lists
// candidate spots for the cannon and, by simulating the falling bombs and
// the moving formation, describes each in plain language: is it safe to
// get there, will an alien be in the line of fire on arrival. The
// simulation starts after Jev's measured latency, so the descriptions are
// about the moment the decision will actually take effect.

const (
	spotStep     = 19  // px between candidate spots across the screen
	afterArrival = 40  // ticks after arrival a spot must stay bomb-free to count as safe
	maxSimTicks  = 200 // cap on how far ahead the simulation looks
	dodgeTicks   = 12  // ticks the cannon needs to sidestep a bomb

	stayKey = "stay"
	keepKey = "keep"
)

var beamStartY = game.CannonY - game.BeamSize.Y

// safety of a candidate spot, from the bomb simulation
type safety int

const (
	safe   safety = iota // no falling bomb hits on the way or for a while after
	risky                // a bomb lands there soon after arrival
	onWay                // a bomb hits the cannon on the way there
	doomed               // a bomb hits before the decision can take effect
)

var safetyText = map[safety]string{
	safe:   "safe: no falling bomb will hit you on the way there or for a while after you arrive",
	risky:  "risky: a falling bomb will land on this spot soon after you arrive",
	onWay:  "deadly: a falling bomb will hit you on the way there",
	doomed: "deadly: a bomb will hit you before you can move",
}

// what a shot would hit first
type target int

const (
	hitNothing target = iota
	hitAlien
	hitUFO
	hitShield
)

var arrivalFireText = map[target]string{
	hitNothing: "nothing will be in your line of fire when you get there",
	hitAlien:   "an alien will be in your line of fire when you get there",
	hitUFO:     "the mystery UFO will be in your line of fire when you get there",
	hitShield:  "your own shield would block your shots here",
}

var nowFireText = map[target]string{
	hitNothing: "a shot fired now would hit nothing",
	hitAlien:   "a shot fired now would hit an alien",
	hitUFO:     "a shot fired now would hit the mystery UFO",
	hitShield:  "a shot fired now would hit your own shield",
}

const (
	laserReady = "ready to fire"
	laserBusy  = "a shot is already in flight, so you cannot fire yet"
)

// spot is one candidate destination for the cannon.
type spot struct {
	Key    string `json:"key"`
	X      int    `json:"x"`
	Safety safety `json:"-"`
	Fire   target `json:"-"`
	Text   string `json:"text"`
}

// pilotState is the part of an observation sent to Jev as state.
type pilotState struct {
	Laser      string `json:"laser"`
	LineOfFire string `json:"line_of_fire"`
}

// observation is everything code worked out for one decision.
type observation struct {
	State pilotState
	Spots []spot
	fire  target // what a shot would hit when the decision takes effect
	seq   uint64 // publish order, so late answers to old observations are ignored
}

// questions builds the two questions asked together for an observation.
func (o *observation) questions() map[string]typesafe.Question {
	options := make(map[string]string, len(o.Spots))
	for _, s := range o.Spots {
		options[s.Key] = s.Text
	}
	return map[string]typesafe.Question{
		"spot": {
			Type: "choice",
			Instructions: map[string]any{
				"role":     "You are playing Space Invaders as the laser cannon at the bottom of the screen. Aliens above drop bombs that fall straight down.",
				"question": "Pick the spot the cannon should go to next, following `priorities` in order.",
				"priorities": []string{
					"1. Survive: pick a spot described as safe if there is one. If no spot is safe, pick a risky spot rather than a deadly one.",
					"2. Attack: among safe spots, pick one where an alien or the mystery UFO will be in your line of fire, even if it is a long move.",
					"3. Commit: if you are already heading to a spot that satisfies 1 and 2, keep heading there.",
					"4. Avoid exposed spots unless they satisfy 2.",
					"5. If no safe spot has an alien or the mystery UFO in your line of fire, stay where you are if that is safe.",
				},
			},
			Criteria: options,
		},
		"fire": {
			Type:         "noul",
			Instructions: "Should the cannon fire its laser right now?",
			Criteria: map[string]string{
				"true":  "`laser` is ready to fire and `line_of_fire` says a shot fired now would hit an alien or the mystery UFO.",
				"false": "`laser` is not ready, or `line_of_fire` says a shot fired now would hit nothing or your own shield.",
			},
		},
	}
}

// observe describes every candidate spot. cur is the spot the cannon is
// currently heading to (-1 for none); lag is Jev's expected latency in
// ticks, during which the cannon keeps heading to cur.
func observe(v game.View, cur, lag int) observation {
	f := newForecast(v)
	x0 := v.Cannon.Min.X
	xl := stepToward(x0, cur, lag) // where the cannon is when the decision lands

	var o observation
	o.fire = f.lineOfFire(xl, lag)
	o.State = pilotState{Laser: laserReady, LineOfFire: nowFireText[o.fire]}
	if v.BeamActive {
		o.State.Laser = laserBusy
	}

	add := func(key, move string, x int) {
		sf, arrive := f.safety(x0, cur, lag, x)
		t := f.lineOfFire(x, arrive)
		text := fmt.Sprintf("%s Safety: %s. Shooting: %s.", move, safetyText[sf], arrivalFireText[t])
		if f.exposed(x, arrive, lag) {
			text += " Exposed: an alien directly above is low enough that a bomb it drops would hit you before you could dodge."
		}
		o.Spots = append(o.Spots, spot{Key: key, X: x, Safety: sf, Fire: t, Text: text})
	}
	add(stayKey, moveText(xl, xl), xl)
	heading := cur >= 0 && abs(cur-xl) >= spotStep/2
	if heading {
		add(keepKey, "Keep heading to your current destination: "+lowerFirst(moveText(xl, cur)), cur)
	}
	maxX := game.Width - game.CannonSize.X
	for i, x := 0, 0; x <= maxX; i, x = i+1, x+spotStep {
		if abs(x-xl) >= spotStep/2 && (!heading || abs(x-cur) >= spotStep/2) {
			add(fmt.Sprintf("spot_%02d", i+1), moveText(xl, x), x)
		}
	}
	return o
}

// moveText describes the move from x to dest in words.
func moveText(x, dest int) string {
	d := dest - x
	if abs(d) < spotStep/2 {
		return "Stay where you are."
	}
	dir := "right"
	if d < 0 {
		dir = "left"
	}
	switch {
	case abs(d) <= 40:
		return "Move a short way to the " + dir + "."
	case abs(d) <= 120:
		return "Move a medium distance to the " + dir + "."
	}
	return "Move a long way to the " + dir + "."
}

// stepToward mirrors how the autopilot steers the cannon: CannonSpeed px
// per tick toward dest (none if dest < 0), stopping once within 2px.
func stepToward(x, dest, ticks int) int {
	for ; ticks > 0 && dest >= 0; ticks-- {
		switch d := dest - x; {
		case d >= 2:
			x += min(game.CannonSpeed, d)
		case d <= -2:
			x -= min(game.CannonSpeed, -d)
		default:
			return x
		}
	}
	return x
}

// forecast predicts bombs, the formation and the UFO a few seconds ahead.
// New bombs can't be predicted, which is why decisions are refreshed
// several times a second.
type forecast struct {
	v       game.View
	bombs   []image.Rectangle // bombs no shield will stop
	offsets []image.Point     // formation displacement after k ticks
}

func newForecast(v game.View) *forecast {
	f := &forecast{v: v}
	for _, b := range v.Bombs {
		if !blockedByShield(v, b) {
			f.bombs = append(f.bombs, b)
		}
	}
	// replay the game's formation stepping, edge reversal included
	f.offsets = make([]image.Point, maxSimTicks+1)
	if len(v.Aliens) > 0 {
		minX, maxX := v.Aliens[0].Rect.Min.X, v.Aliens[0].Rect.Max.X
		for _, a := range v.Aliens {
			minX, maxX = min(minX, a.Rect.Min.X), max(maxX, a.Rect.Max.X)
		}
		dir, next, off := v.AlienDirection, v.NextMoveTick, image.Point{}
		for k := 1; k <= maxSimTicks; k++ {
			if t := v.Tick + k; t >= next {
				next = t + v.MoveInterval
				step := game.AlienStep * dir
				if (dir > 0 && maxX+step > game.Width) || (dir < 0 && minX+step < 0) {
					dir = -dir
					off.Y += game.AlienDrop
				} else {
					off.X += step
					minX += step
					maxX += step
				}
			}
			f.offsets[k] = off
		}
	}
	return f
}

// safety simulates the cannon heading to cur for lag ticks, then to dest,
// against every falling bomb. It returns the spot's safety and the tick the
// cannon arrives.
func (f *forecast) safety(x0, cur, lag, dest int) (safety, int) {
	xl := stepToward(x0, cur, lag)
	arrive := lag + (abs(dest-xl)+game.CannonSpeed-1)/game.CannonSpeed
	horizon := min(arrive+afterArrival, maxSimTicks)
	w, h := game.CannonSize.X, game.CannonSize.Y
	x := x0
	for t := 1; t <= horizon; t++ {
		if t <= lag {
			x = stepToward(x, cur, 1)
		} else {
			x = stepToward(x, dest, 1)
		}
		c := image.Rect(x, game.CannonY, x+w, game.CannonY+h)
		for _, b := range f.bombs {
			if b.Add(image.Pt(0, game.BombSpeed*t)).Overlaps(c) {
				switch {
				case t <= lag:
					return doomed, arrive
				case t <= arrive:
					return onWay, arrive
				}
				return risky, arrive
			}
		}
	}
	return safe, arrive
}

// exposed reports whether, once the cannon is at x (arrive ticks from
// now), a bomb freshly dropped by the alien above would land before the
// autopilot could see it and dodge: roughly one latency plus a sidestep.
func (f *forecast) exposed(x, arrive, lag int) bool {
	t := min(arrive, maxSimTicks)
	c := image.Rect(x, 0, x+game.CannonSize.X, game.Height)
	for _, a := range lowestPerColumn(f.v.Aliens) {
		r := a.Add(f.offsets[t])
		if r.Overlaps(c) && (game.CannonY-r.Max.Y)/game.BombSpeed < lag+dodgeTicks {
			return true
		}
	}
	return false
}

// lineOfFire predicts what a shot fired from cannon position x, delay
// ticks from now, hits first, allowing for the formation and UFO moving
// while the beam climbs.
func (f *forecast) lineOfFire(x, delay int) target {
	v := f.v
	bx := x + game.BeamOffsetX
	col := image.Rect(bx, 0, bx+game.BeamSize.X, game.Height)
	for _, s := range v.Shields {
		if col.Overlaps(s) {
			return hitShield
		}
	}
	for k := 1; beamStartY-game.BeamSpeed*k+game.BeamSize.Y > 0; k++ {
		t := min(delay+k, maxSimTicks)
		y := beamStartY - game.BeamSpeed*k
		beam := image.Rect(col.Min.X, y, col.Max.X, y+game.BeamSize.Y)
		for _, a := range v.Aliens {
			if beam.Overlaps(a.Rect.Add(f.offsets[t])) {
				return hitAlien
			}
		}
		if v.UFOActive && beam.Overlaps(v.UFO.Add(image.Pt(game.UFOSpeed*v.UFODir*t, 0))) {
			return hitUFO
		}
	}
	return hitNothing
}

// blockedByShield reports whether a falling bomb will hit a shield before
// reaching the cannon row, as the game would.
func blockedByShield(v game.View, bomb image.Rectangle) bool {
	for _, s := range v.Shields {
		if bomb.Max.X > s.Min.X && bomb.Min.X < s.Max.X && bomb.Min.Y < s.Max.Y {
			return true
		}
	}
	return false
}

// lowestPerColumn returns the lowest alive alien in each formation column:
// the only ones that can drop bombs.
func lowestPerColumn(aliens []game.Alien) []image.Rectangle {
	best := map[int]image.Rectangle{}
	for _, a := range aliens {
		if cur, ok := best[a.Column]; !ok || a.Rect.Min.Y > cur.Min.Y {
			best[a.Column] = a.Rect
		}
	}
	out := make([]image.Rectangle, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
