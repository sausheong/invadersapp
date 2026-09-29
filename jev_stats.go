//go:build jev

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scoring the Jev autopilot. Every Jev decision becomes an action; its
// move is judged by whether the cannon survives the next outcomeWindow
// ticks, and a shot it fires is judged by what the beam hits.

const outcomeWindow = 50 // ticks (1s) a move decision is held responsible for

// move outcomes
const (
	outcomeEscaped  = "escaped"  // under threat, survived the window
	outcomeSurvived = "survived" // safe, survived the window
	outcomeKilled   = "killed"   // cannon hit within the window
)

// shot outcomes
var shotNames = map[gameEvent]string{
	evShotAlien:     "hit alien",
	evShotUFO:       "hit ufo",
	evShotMissed:    "missed",
	evShotShield:    "hit own shield",
	evShotCancelled: "cancelled",
}

// action is one Jev decision and what came of it.
type action struct {
	ID        int        `json:"id"`
	Tick      int        `json:"tick"`
	Situation string     `json:"situation"` // "threat" if staying put wasn't safe, else "safe"
	State     pilotState `json:"state"`
	Stay      string     `json:"stay"`       // how staying put was described
	Chosen    spot       `json:"chosen"`     // the spot Jev picked
	SafeSpots int        `json:"safe_spots"` // how many spots were described as safe
	Move      string     `json:"move"`       // direction that spot meant
	MoveConf  float64    `json:"move_confidence"`
	FireProb  float64    `json:"fire_probability"`
	Fired     bool       `json:"fired"`
	Outcome   string     `json:"outcome"`
	Shot      string     `json:"shot,omitempty"`

	shotPending bool
}

// pilotStats tallies actions. Actions are added and resolved on the game
// goroutine; the summary is read from the main thread at exit, hence the
// mutex.
type pilotStats struct {
	mu      sync.Mutex
	clock   int       // playing ticks seen, pauses excluded
	pending []*action // moves still inside their outcome window
	shooter *action   // action whose shot is in flight
	logFile *os.File

	Threat, ThreatEscaped int
	Safe, SafeKilled      int
	Moves                 map[string]map[string][2]int // situation -> move -> {n, success}
	Shots                 map[string]int               // shot outcome -> count
	Fired                 int
	Stale                 int // fire decisions dropped: line of fire changed before firing
	Blocked               int // fire decisions dropped: a shot was already in flight
}

// fireStale counts a fire decision dropped because the cannon's line of
// fire changed between Jev's decision and the tick it would have fired.
func (s *pilotStats) fireStale() {
	s.mu.Lock()
	s.Stale++
	s.mu.Unlock()
}

// fireBlocked counts a fire decision dropped because a shot was in flight.
func (s *pilotStats) fireBlocked() {
	s.mu.Lock()
	s.Blocked++
	s.mu.Unlock()
}

// add records a new decision at the current clock.
func (s *pilotStats) add(a *action) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.Tick = s.clock
	if a.Situation == "" {
		a.Situation = "safe"
	}
	s.pending = append(s.pending, a)
}

// tick advances the clock one playing tick and resolves every move whose
// window has passed without the cannon being hit.
func (s *pilotStats) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clock++
	kept := s.pending[:0]
	for _, a := range s.pending {
		if s.clock-a.Tick < outcomeWindow {
			kept = append(kept, a)
			continue
		}
		if a.Situation == "threat" {
			s.resolve(a, outcomeEscaped)
		} else {
			s.resolve(a, outcomeSurvived)
		}
	}
	s.pending = kept
}

// event handles a game outcome reported through Game.onEvent.
func (s *pilotStats) event(ev gameEvent, fromPilot bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch ev {
	case evCannonHit:
		for _, a := range s.pending {
			s.resolve(a, outcomeKilled)
		}
		s.pending = nil
	case evShotFired:
		if s.shooter != nil {
			s.finishShot(shotNames[evShotCancelled])
		}
		if fromPilot {
			// the shot belongs to the latest action that asked to fire
			for i := len(s.pending) - 1; i >= 0; i-- {
				if a := s.pending[i]; a.FireProb >= fireThreshold && !a.Fired {
					a.Fired, a.shotPending = true, true
					s.shooter = a
					s.Fired++
					break
				}
			}
		}
	default:
		if name, ok := shotNames[ev]; ok && fromPilot && s.shooter != nil {
			s.finishShot(name)
		}
	}
}

// finishShot records how the in-flight pilot shot ended.
func (s *pilotStats) finishShot(name string) {
	a := s.shooter
	s.shooter = nil
	a.Shot, a.shotPending = name, false
	if s.Shots == nil {
		s.Shots = map[string]int{}
	}
	s.Shots[name]++
	if a.Outcome != "" {
		s.write(a) // move already resolved; log now that the shot is too
	}
}

// resolve records a move outcome and logs the action unless its shot is
// still in flight (finishShot logs it then).
func (s *pilotStats) resolve(a *action, outcome string) {
	a.Outcome = outcome
	success := outcome != outcomeKilled
	if a.Situation == "threat" {
		s.Threat++
		if success {
			s.ThreatEscaped++
		}
	} else {
		s.Safe++
		if !success {
			s.SafeKilled++
		}
	}
	if s.Moves == nil {
		s.Moves = map[string]map[string][2]int{}
	}
	if s.Moves[a.Situation] == nil {
		s.Moves[a.Situation] = map[string][2]int{}
	}
	m := s.Moves[a.Situation][a.Move]
	m[0]++
	if success {
		m[1]++
	}
	s.Moves[a.Situation][a.Move] = m
	if !a.shotPending {
		s.write(a)
	}
}

// write appends one resolved action to the JSONL log, if one is open.
func (s *pilotStats) write(a *action) {
	if s.logFile == nil {
		return
	}
	if b, err := json.Marshal(a); err == nil {
		s.logFile.Write(append(b, '\n'))
	}
}

// hits returns shots that hit an alien or the UFO, and resolved shots
// (fired shots excluding cancelled and still in flight).
func (s *pilotStats) hits() (hit, total int) {
	hit = s.Shots[shotNames[evShotAlien]] + s.Shots[shotNames[evShotUFO]]
	for name, n := range s.Shots {
		if name != shotNames[evShotCancelled] {
			total += n
		}
	}
	return hit, total
}

// hudText is the compact running score shown during play.
func (s *pilotStats) hudText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Threat == 0 && len(s.Shots) == 0 {
		return ""
	}
	hit, shots := s.hits()
	return fmt.Sprintf("DODGE %d/%d HIT %d/%d", s.ThreatEscaped, s.Threat, hit, shots)
}

// summary is the end-of-session report.
func (s *pilotStats) summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Threat+s.Safe == 0 && s.Fired == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Jev autopilot results\n")
	fmt.Fprintf(&b, "  Moves under threat (staying put unsafe): %d/%d escaped  %s\n", s.ThreatEscaped, s.Threat, pct(s.ThreatEscaped, s.Threat))
	fmt.Fprintf(&b, "  Moves when safe:                         %d/%d survived %s\n", s.Safe-s.SafeKilled, s.Safe, pct(s.Safe-s.SafeKilled, s.Safe))
	for _, sit := range []string{"threat", "safe"} {
		for _, mv := range []string{moveLeft, moveRight, moveStay} {
			if m, ok := s.Moves[sit][mv]; ok {
				fmt.Fprintf(&b, "    %-6s %-5s %4d actions, %s ok\n", sit, mv, m[0], pct(m[1], m[0]))
			}
		}
	}
	hit, shots := s.hits()
	fmt.Fprintf(&b, "  Shots: %d fired, %d/%d hit %s; %d fire decisions dropped as stale, %d while a shot was in flight\n", s.Fired, hit, shots, pct(hit, shots), s.Stale, s.Blocked)
	for _, ev := range []gameEvent{evShotAlien, evShotUFO, evShotMissed, evShotShield, evShotCancelled} {
		if n := s.Shots[shotNames[ev]]; n > 0 {
			fmt.Fprintf(&b, "    %-15s %d\n", shotNames[ev], n)
		}
	}
	return b.String()
}

func pct(n, d int) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("(%.0f%%)", 100*float64(n)/float64(d))
}

// statsLogPath is where resolved actions are logged, one JSON per line.
func statsLogPath() string {
	return filepath.Join(filepath.Dir(highScorePath()), "jev-actions.jsonl")
}

// summaryPath is where the session summary is kept up to date.
func summaryPath() string {
	return filepath.Join(filepath.Dir(statsLogPath()), "jev-summary.txt")
}

// openLog starts appending actions to the log file and keeps the summary
// file current every few seconds, tolerating errors.
func (s *pilotStats) openLog() {
	path := statsLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		s.logFile = f
	}
	go func() {
		for range time.Tick(5 * time.Second) {
			s.writeSummary()
		}
	}()
}

// writeSummary saves the current summary, once there is something to report.
func (s *pilotStats) writeSummary() {
	if sum := s.summary(); sum != "" {
		os.WriteFile(summaryPath(), []byte(sum), 0o644)
	}
}

// close writes the final summary and closes the log.
func (s *pilotStats) close() {
	s.writeSummary()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}
}
