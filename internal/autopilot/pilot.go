// Package autopilot lets Jev, TypeSafe's System One model, play the game.
// Code predicts and describes (forecast.go); Jev picks a spot for the
// cannon and decides whether to fire; code steers. Every decision is
// scored (stats.go).
package autopilot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sausheong/invadersapp/internal/game"
	"github.com/sausheong/invadersapp/internal/typesafe"
)

const (
	minInterval   = 100 * time.Millisecond // a new request at most this often
	inFlight      = 6                      // overlapping requests, so decisions are fresher than one round trip
	escapeMargin  = 5                      // extra ticks of warning before using the planned escape
	staleAfter    = 1500 * time.Millisecond
	timeout       = 2 * time.Second
	startDelay    = 100 // ticks (2s) on the title or game over screen before a new game starts
	fireThreshold = 0.5

	moveLeft  = "left"
	moveRight = "right"
	moveStay  = "stay"
)

// Pilot is the Jev autopilot. It implements game.Autopilot.
type Pilot struct {
	enabled atomic.Bool
	obs     atomic.Pointer[observation]
	lag     atomic.Int64 // expected latency in ticks (moving average)
	seq     atomic.Uint64
	hud     atomic.Value // string shown in the HUD
	once    sync.Once
	client  *typesafe.Client
	stats   stats

	mu         sync.Mutex
	dest       int // spot the cannon is heading to, -1 for none
	decidedAt  time.Time
	fire       bool
	fireAt     target  // what Jev was told a shot would hit
	decision   *action // latest decision, not yet handed to stats
	pendingX   int     // chosen spot of that decision
	pendingEsc int     // escape direction planned with that decision
	escapeDir  int     // Jev's planned escape for the decision in force: -1 left, +1 right
	escaping   bool    // the planned escape is under way
	nextID     int
	lastSeq    uint64 // observation behind the latest decision used
	idle       int    // ticks spent off the playing screen
}

// New returns an autopilot that is switched off until Toggle.
func New() *Pilot {
	p := &Pilot{dest: -1}
	p.lag.Store(15)
	return p
}

// Toggle switches the autopilot on or off, starting its worker the first
// time it's switched on.
func (p *Pilot) Toggle() {
	if p.enabled.Load() {
		p.enabled.Store(false)
		p.hud.Store("")
		return
	}
	key := apiKey()
	if key == "" {
		p.hud.Store("JEV: set TYPESAFE_API_KEY")
		return
	}
	p.once.Do(func() {
		p.client = typesafe.NewClient(key)
		p.stats.openLog()
		go p.run()
	})
	p.hud.Store("JEV")
	p.enabled.Store(true)
}

// apiKey reads the TypeSafe key the usual ways, then from a
// typesafe-api-key file in the game's data directory (apps launched from
// Finder don't inherit shell environment variables).
func apiKey() string {
	if k := typesafe.APIKey(); k != "" {
		return k
	}
	data, err := os.ReadFile(filepath.Join(game.DataDir(), "typesafe-api-key"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Input toggles the autopilot on the j key, starts games when it's idle,
// and otherwise steers toward the latest chosen spot and fires if Jev said
// to. A fire decision is dropped as stale if what the shot would hit has
// changed since Jev saw it; a decision older than staleAfter (e.g. after
// failed requests) stops the cannon.
func (p *Pilot) Input(v game.View, in game.Input) game.Input {
	if in.Pilot {
		p.Toggle()
	}
	if !p.enabled.Load() {
		return in
	}
	if v.State != game.StatePlaying {
		p.idle++
		if p.idle >= startDelay {
			p.idle = 0
			in.Start = true
		}
		return in
	}
	p.idle = 0
	return p.steer(v, in, time.Now())
}

// steer applies the latest decision to this tick's input.
func (p *Pilot) steer(v game.View, in game.Input, now time.Time) game.Input {
	p.mu.Lock()
	defer p.mu.Unlock()
	x := v.Cannon.Min.X
	if p.decision != nil {
		p.dest, p.escapeDir, p.escaping = p.pendingX, p.pendingEsc, false
		p.decision.Move = moveStay
		if d := p.dest - x; d >= 2 {
			p.decision.Move = moveRight
		} else if d <= -2 {
			p.decision.Move = moveLeft
		}
		p.stats.add(p.decision)
		p.decision = nil
	}
	if p.dest >= 0 && now.Sub(p.decidedAt) > staleAfter {
		p.dest = -1
	}
	// A bomb the current plan runs into before a fresh decision could
	// arrive: carry out the escape Jev planned in advance.
	if !p.escaping && p.escapeDir != 0 {
		f := newForecast(v)
		if f.hitSoon(x, p.dest, int(p.lag.Load())+escapeMargin) {
			if ex, _ := f.escapeSpot(x, p.escapeDir); ex >= 0 {
				p.dest, p.escaping = ex, true
				p.stats.escapeUsed()
			}
		}
	}
	if p.dest >= 0 {
		d := p.dest - x
		in.Left, in.Right = d <= -2, d >= 2
	}
	if p.fire {
		p.fire = false
		if v.BeamActive {
			p.stats.fireBlocked()
		} else if newForecast(v).lineOfFire(x, 0) != p.fireAt {
			p.stats.fireStale()
		} else {
			in.Fire, in.PilotFire = true, true
		}
	}
	return in
}

// Observe scores outcomes and publishes a fresh observation, or clears it
// when the cannon can't act (not playing, paused, or exploding).
func (p *Pilot) Observe(v game.View) {
	acting := v.State == game.StatePlaying && !v.Paused && !v.CannonExploding
	if acting {
		p.stats.tick()
	}
	if !p.enabled.Load() || !acting {
		p.obs.Store(nil)
		p.mu.Lock()
		p.dest = -1
		p.mu.Unlock()
		return
	}
	p.mu.Lock()
	cur := p.dest
	p.mu.Unlock()
	o := observe(v, cur, int(p.lag.Load()))
	o.seq = p.seq.Add(1)
	p.obs.Store(&o)
}

// Event passes game outcomes to the stats.
func (p *Pilot) Event(ev game.Event, fromPilot bool) { p.stats.event(ev, fromPilot) }

// HUD returns the status line (top) and running score (bottom).
func (p *Pilot) HUD() (top, bottom string) {
	top, _ = p.hud.Load().(string)
	return top, p.stats.hudText()
}

// Stop prints and saves the session summary.
func (p *Pilot) Stop() {
	if s := p.stats.summary(); s != "" {
		fmt.Print(s)
		fmt.Println("  Per-action log:", logPath())
	}
	p.stats.close()
}

// run starts a request every minInterval, up to inFlight at once, each
// about the latest observation.
func (p *Pilot) run() {
	slots := make(chan struct{}, inFlight)
	var last *observation
	for {
		time.Sleep(minInterval)
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
func (p *Pilot) decide(o *observation) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	answers, err := p.client.Ask(ctx, o.State, o.questions())
	cancel()
	if err != nil {
		p.hud.Store("JEV: request failed")
		return
	}
	took := time.Since(start)
	// moving average of latency, in ticks, for the next simulation
	p.lag.Store((3*p.lag.Load() + took.Milliseconds()/20) / 4)

	pick, fire, esc := answers["spot"], answers["fire"], answers["escape"]
	escDir := 1
	if esc.Choice == escapeLeft {
		escDir = -1
	}
	chosen, stay := o.Spots[0], o.Spots[0]
	for _, s := range o.Spots {
		if s.Key == pick.Choice {
			chosen = s
		}
	}
	a := &action{
		Situation: "safe", State: o.State, Chosen: chosen, Stay: stay.Text,
		MoveConf: pick.Confidence, FireProb: fire.Noul, Escape: esc.Choice,
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
	p.decision, p.pendingX, p.pendingEsc, p.decidedAt = a, chosen.X, escDir, time.Now()
	p.fire = fire.Noul >= fireThreshold && o.State.Laser == laserReady
	p.fireAt = o.fire
	p.mu.Unlock()
	if p.enabled.Load() {
		p.hud.Store(fmt.Sprintf("JEV %s %.2f %dms", arrow(chosen.X-stay.X), pick.Confidence, took.Milliseconds()))
	}
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
