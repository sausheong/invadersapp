package main

import "sync"

// Input is one tick's worth of player input, snapshotted from the shared
// inputState. Left/Right are level-triggered (held); the rest are
// edge-triggered (true only on the tick the key was pressed).
type Input struct {
	Left, Right bool
	Fire        bool
	Pause       bool
	Start       bool
	Quit        bool
	Pilot       bool // toggle the autopilot (only in builds with -tags jev)
	PilotFire   bool // Fire was requested by the autopilot
}

// inputState holds the raw key state shared between the UI thread (webview
// Bind callbacks calling keyDown/keyUp) and the single game loop goroutine.
// It must stay cheap: keyDown/keyUp are called directly on the UI thread.
type inputState struct {
	mu sync.Mutex

	leftHeld, rightHeld bool
	fireEvent           bool
	pauseEvent          bool
	startEvent          bool
	quitEvent           bool
	pilotEvent          bool
}

var input inputState

// JS keyCodes used by the game.
const (
	keyLeft  = 37
	keyRight = 39
	keySpace = 32
	keyS     = 83
	keyQ     = 81
	keyP     = 80
	keyJ     = 74
)

// keyDown handles a key press bound from the webview UI thread. Fast and
// thread-safe: it only ever flips a few booleans behind a mutex.
func keyDown(code int) {
	input.mu.Lock()
	defer input.mu.Unlock()
	switch code {
	case keyLeft:
		input.leftHeld = true
	case keyRight:
		input.rightHeld = true
	case keySpace:
		input.fireEvent = true
	case keyP:
		input.pauseEvent = true
	case keyS:
		input.startEvent = true
	case keyQ:
		input.quitEvent = true
	case keyJ:
		input.pilotEvent = true
	}
}

// keyUp handles a key release bound from the webview UI thread.
func keyUp(code int) {
	input.mu.Lock()
	defer input.mu.Unlock()
	switch code {
	case keyLeft:
		input.leftHeld = false
	case keyRight:
		input.rightHeld = false
	}
}

// snapshot returns the input for one game tick, consuming (clearing) the
// edge-triggered events so each key press is acted on exactly once no
// matter how long the game loop takes to get to it.
func (in *inputState) snapshot() Input {
	in.mu.Lock()
	defer in.mu.Unlock()
	snap := Input{
		Left:  in.leftHeld,
		Right: in.rightHeld,
		Fire:  in.fireEvent,
		Pause: in.pauseEvent,
		Start: in.startEvent,
		Quit:  in.quitEvent,
		Pilot: in.pilotEvent,
	}
	in.fireEvent = false
	in.pauseEvent = false
	in.startEvent = false
	in.quitEvent = false
	in.pilotEvent = false
	return snap
}
