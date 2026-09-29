//go:build jev

package main

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The Jev autopilot. Code lists candidate spots for the cannon and, by
// simulating the falling bombs and the moving formation, describes each
// in plain language: is it safe to get there, will an alien be in the line
// of fire on arrival. Jev (a text model, not a physics engine) judges which
// spot to go to and whether to fire; code then steers the cannon there.
// The simulation starts after Jev's measured latency, so the descriptions
// are about the moment the decision will actually take effect.

const (
	pilotMinInterval = 100 * time.Millisecond // a new request at most this often
	pilotInFlight    = 3                      // overlapping requests, so decisions are fresher than one round trip
	pilotStale       = 1500 * time.Millisecond
	pilotTimeout     = 2 * time.Second
	pilotStartDelay  = 100 // ticks (2s) on the title or game over screen before the autopilot starts a game

	spotStep      = 19  // px between candidate spots across the screen
	afterArrival  = 40  // ticks after arrival a spot must stay bomb-free to count as safe
	maxSimTicks   = 200 // cap on how far ahead the simulation looks
	beamStartY    = cannonY - 5
	fireThreshold = 0.5

	moveLeft  = "left"
	moveRight = "right"
	moveStay  = "stay"
	stayKey   = "stay"
	keepKey   = "keep"

	dodgeTicks = 12 // ticks the cannon needs to sidestep a bomb
)

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

// question builds the two questions asked together for an observation.
func (o *observation) questions() map[string]jevQuestion {
	options := make(map[string]string, len(o.Spots))
	for _, s := range o.Spots {
		options[s.Key] = s.Text
	}
	return map[string]jevQuestion{
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
func observe(g *Game, cur, lag int) observation {
	f := newForecast(g)
	x0 := g.cannon.Position.X
	xl := stepToward(x0, cur, lag) // where the cannon is when the decision lands

	var o observation
	o.fire = f.lineOfFire(xl, lag)
	o.State = pilotState{Laser: laserReady, LineOfFire: nowFireText[o.fire]}
	if g.beamActive {
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
	maxX := gameWidth - g.cannon.size.Dx()
	for i, x := 0, 0; x <= maxX; i, x = i+1, x+spotStep {
		if abs(x-xl) >= spotStep/2 && (!heading || abs(x-cur) >= spotStep/2) {
			add(fmt.Sprintf("spot_%02d", i+1), moveText(xl, x), x)
		}
	}
	return o
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
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

// stepToward mirrors how apply steers the cannon: cannonSpeed px per tick
// toward dest (none if dest < 0), stopping once within 2px.
func stepToward(x, dest, ticks int) int {
	for ; ticks > 0 && dest >= 0; ticks-- {
		switch d := dest - x; {
		case d >= 2:
			x += min(cannonSpeed, d)
		case d <= -2:
			x -= min(cannonSpeed, -d)
		default:
			return x
		}
	}
	return x
}

// forecast predicts bombs, the formation and the UFO a few seconds ahead
// from the current game state. New bombs can't be predicted, which is why
// decisions are refreshed several times a second.
type forecast struct {
	g       *Game
	bombs   []image.Rectangle // bombs no shield will stop
	offsets []image.Point     // formation displacement after k ticks
}

func newForecast(g *Game) *forecast {
	f := &forecast{g: g}
	for _, b := range g.bombs {
		if r := b.rect(); !blockedByShield(g, r) {
			f.bombs = append(f.bombs, r)
		}
	}
	// replay advanceFormation's stepping (edge reversal included)
	f.offsets = make([]image.Point, maxSimTicks+1)
	if aliveAlienCount(g.aliens) > 0 {
		minX, maxX := aliveAlienXExtent(g.aliens)
		dir, next, off := g.alienDirection, g.nextMoveTick, image.Point{}
		for k := 1; k <= maxSimTicks; k++ {
			if t := g.tick + k; t >= next {
				next = t + g.moveInterval
				step := 3 * dir
				if (dir > 0 && maxX+step > gameWidth) || (dir < 0 && minX+step < 0) {
					dir = -dir
					off.Y += 10
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
	arrive := lag + (abs(dest-xl)+cannonSpeed-1)/cannonSpeed
	horizon := min(arrive+afterArrival, maxSimTicks)
	w, h := cannonSprite.Dx(), cannonSprite.Dy()
	x := x0
	for t := 1; t <= horizon; t++ {
		if t <= lag {
			x = stepToward(x, cur, 1)
		} else {
			x = stepToward(x, dest, 1)
		}
		c := image.Rect(x, cannonY, x+w, cannonY+h)
		for _, b := range f.bombs {
			if b.Add(image.Pt(0, bombSpeed*t)).Overlaps(c) {
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
	c := image.Rect(x, 0, x+cannonSprite.Dx(), gameHeight)
	for _, i := range lowestAlivePerColumn(f.g.aliens) {
		a := f.g.aliens[i].rect().Add(f.offsets[t])
		if a.Overlaps(c) && (cannonY-a.Max.Y)/bombSpeed < lag+dodgeTicks {
			return true
		}
	}
	return false
}

// lineOfFire predicts what a shot fired from cannon position x, delay
// ticks from now, hits first, allowing for the formation and UFO moving
// while the beam climbs.
func (f *forecast) lineOfFire(x, delay int) target {
	g := f.g
	col := image.Rect(x+7, 0, x+7+beamSprite.Dx(), gameHeight)
	for _, s := range g.shields {
		if s.alive() && col.Overlaps(s.bounds()) {
			return hitShield
		}
	}
	for k := 1; beamStartY-beamSpeed*k+beamSprite.Dy() > 0; k++ {
		t := min(delay+k, maxSimTicks)
		beam := image.Rect(col.Min.X, beamStartY-beamSpeed*k, col.Max.X, beamStartY-beamSpeed*k+beamSprite.Dy())
		for _, a := range g.aliens {
			if a.Status && beam.Overlaps(a.rect().Add(f.offsets[t])) {
				return hitAlien
			}
		}
		if g.ufoActive && beam.Overlaps(g.ufo.rect().Add(image.Pt(ufoSpeed*g.ufoDir*t, 0))) {
			return hitUFO
		}
	}
	return hitNothing
}

// blockedByShield reports whether a falling bomb will hit a live shield
// before reaching the cannon row, as moveBombs would.
func blockedByShield(g *Game, bomb image.Rectangle) bool {
	for _, s := range g.shields {
		sb := s.bounds()
		if s.alive() && bomb.Max.X > sb.Min.X && bomb.Min.X < sb.Max.X && bomb.Min.Y < sb.Max.Y {
			return true
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// autopilot runs Jev in its own goroutine. The game loop publishes an
// observation every tick and steers by the latest decision.
type autopilot struct {
	enabled atomic.Bool
	obs     atomic.Pointer[observation]
	lag     atomic.Int64 // expected latency in ticks (moving average)
	seq     atomic.Uint64
	hud     atomic.Value // string shown in the HUD
	once    sync.Once
	client  *jevClient
	stats   pilotStats

	mu        sync.Mutex
	dest      int // spot the cannon is heading to, -1 for none
	decidedAt time.Time
	fire      bool
	fireAt    target  // what Jev was told a shot would hit
	decision  *action // latest decision, not yet handed to stats
	pendingX  int     // chosen spot of that decision
	nextID    int
	lastSeq   uint64 // observation behind the latest decision used
	idle      int    // ticks spent off the playing screen
}

var pilot = autopilot{dest: -1}

func init() { pilot.lag.Store(15) }

// toggle switches the autopilot on or off, starting its worker the first
// time it's enabled.
func (p *autopilot) toggle() {
	if p.enabled.Load() {
		p.enabled.Store(false)
		p.hud.Store("")
		return
	}
	key := jevAPIKey()
	if key == "" {
		p.hud.Store("JEV: set TYPESAFE_API_KEY")
		return
	}
	p.once.Do(func() {
		p.client = newJevClient(key)
		p.stats.openLog()
		go p.run()
	})
	p.hud.Store("JEV")
	p.enabled.Store(true)
}

// publish records the latest observation, or clears it when the cannon
// can't act (not playing, paused, or exploding).
func (p *autopilot) publish(g *Game) {
	if !p.enabled.Load() || g.state != statePlaying || g.paused || g.cannonExploding {
		p.obs.Store(nil)
		p.mu.Lock()
		p.dest = -1
		p.mu.Unlock()
		return
	}
	p.mu.Lock()
	cur := p.dest
	p.mu.Unlock()
	o := observe(g, cur, int(p.lag.Load()))
	o.seq = p.seq.Add(1)
	p.obs.Store(&o)
}

// autoStart presses start once the autopilot has sat on the title or game
// over screen for pilotStartDelay ticks, so it can play unattended.
func (p *autopilot) autoStart(g *Game, in Input) Input {
	if !p.enabled.Load() || g.state == statePlaying {
		p.idle = 0
		return in
	}
	p.idle++
	if p.idle >= pilotStartDelay {
		p.idle = 0
		in.Start = true
	}
	return in
}

// apply steers the cannon toward the latest chosen spot and fires if Jev
// said to. A fire decision is dropped as stale if what the shot would hit
// has changed since Jev saw it; a decision older than pilotStale (e.g.
// after failed requests) stops the cannon.
func (p *autopilot) apply(g *Game, in Input, now time.Time) Input {
	if !p.enabled.Load() {
		return in
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.decision != nil {
		p.dest = p.pendingX
		p.decision.Move = moveStay
		if d := p.dest - g.cannon.Position.X; d >= 2 {
			p.decision.Move = moveRight
		} else if d <= -2 {
			p.decision.Move = moveLeft
		}
		p.stats.add(p.decision)
		p.decision = nil
	}
	if p.dest >= 0 && now.Sub(p.decidedAt) > pilotStale {
		p.dest = -1
	}
	if p.dest >= 0 {
		d := p.dest - g.cannon.Position.X
		in.Left, in.Right = d <= -2, d >= 2
	}
	if p.fire {
		p.fire = false
		if g.beamActive {
			p.stats.fireBlocked()
		} else if newForecast(g).lineOfFire(g.cannon.Position.X, 0) != p.fireAt {
			p.stats.fireStale()
		} else {
			in.Fire, in.PilotFire = true, true
		}
	}
	return in
}

// run starts a request every pilotMinInterval, up to pilotInFlight at
// once, each about the latest observation.
func (p *autopilot) run() {
	slots := make(chan struct{}, pilotInFlight)
	var last *observation
	for {
		time.Sleep(pilotMinInterval)
		o := p.obs.Load()
		if !p.enabled.Load() || o == nil || o == last {
			continue
		}
		last = o
		slots <- struct{}{}
		go func() {
			defer func() { <-slots }()
			p.decide(o)
		}()
	}
}

// decide asks Jev about one observation and records the decision, unless a
// decision about a newer observation has already been recorded.
func (p *autopilot) decide(o *observation) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), pilotTimeout)
	answers, err := p.client.ask(ctx, o.State, o.questions())
	cancel()
	if err != nil {
		p.hud.Store("JEV: request failed")
		return
	}
	took := time.Since(start)
	// moving average of latency, in ticks, for the next simulation
	p.lag.Store((3*p.lag.Load() + took.Milliseconds()/20) / 4)

	pick, fire := answers["spot"], answers["fire"]
	chosen, stay := o.Spots[0], o.Spots[0]
	for _, s := range o.Spots {
		if s.Key == pick.Choice {
			chosen = s
		}
	}
	a := &action{
		Situation: "safe", State: o.State, Chosen: chosen, Stay: stay.Text,
		MoveConf: pick.Confidence, FireProb: fire.Noul,
	}
	if stay.Safety != safe {
		a.Situation = "threat"
	}
	for _, s := range o.Spots {
		if s.Safety == safe {
			a.SafeSpots++
		}
	}
	p.mu.Lock()
	if o.seq <= p.lastSeq {
		p.mu.Unlock()
		return // a fresher decision already arrived
	}
	p.lastSeq = o.seq
	p.nextID++
	a.ID = p.nextID
	p.decision, p.pendingX, p.decidedAt = a, chosen.X, time.Now()
	p.fire = fire.Noul >= fireThreshold && o.State.Laser == laserReady
	p.fireAt = o.fire
	p.mu.Unlock()
	if p.enabled.Load() {
		p.hud.Store(fmt.Sprintf("JEV %s %.2f %dms", arrow(chosen.X-stay.X), pick.Confidence, took.Milliseconds()))
	}
}

// hudText returns the autopilot status line, or "" when there's nothing
// to show.
func (p *autopilot) hudText() string {
	s, _ := p.hud.Load().(string)
	return s
}

func arrow(d int) string {
	switch {
	case d <= -2:
		return "<"
	case d >= 2:
		return ">"
	}
	return "="
}
