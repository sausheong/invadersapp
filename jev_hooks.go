//go:build jev

package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"time"
)

// Hooks the game calls to run the Jev autopilot. Building without the jev
// tag swaps these for the no-ops in nojev.go.

var hudGreen = color.RGBA{80, 220, 80, 255}

var jevFlag = flag.Bool("jev", false, "start with the Jev autopilot on (needs TYPESAFE_API_KEY)")

// pilotStart runs after flag parsing.
func pilotStart() {
	if *jevFlag {
		pilot.toggle()
	}
}

// pilotAttach wires a game's outcome events into the autopilot stats.
func pilotAttach(g *Game) {
	g.onEvent = pilot.stats.event
}

// pilotInput lets the autopilot toggle and drive this tick's input.
func pilotInput(g *Game, in Input) Input {
	if in.Pilot {
		pilot.toggle()
	}
	in = pilot.autoStart(g, in)
	return pilot.apply(g, in, time.Now())
}

// pilotAfterStep scores outcomes and publishes the new observation.
func pilotAfterStep(g *Game) {
	if g.state == statePlaying && !g.paused && !g.cannonExploding {
		pilot.stats.tick()
	}
	pilot.publish(g)
}

// pilotHUD draws the autopilot status and running score.
func pilotHUD(dst *image.RGBA) {
	if s := pilot.hudText(); s != "" {
		printLine(dst, gameWidth-4-len(s)*8, 12, s, hudGreen)
	}
	if s := pilot.stats.hudText(); s != "" {
		printLine(dst, gameWidth-4-len(s)*8, gameHeight-4, s, hudGreen)
	}
}

// pilotStop prints and saves the session summary.
func pilotStop() {
	if s := pilot.stats.summary(); s != "" {
		fmt.Print(s)
		fmt.Println("  Per-action log:", statsLogPath())
	}
	pilot.stats.close()
}
