//go:build !jev

package main

import "image"

// No-op autopilot hooks for builds without the jev tag. Build with
// -tags jev to include the Jev autopilot (see jev_*.go).

func pilotStart()                        {}
func pilotAttach(g *Game)                {}
func pilotInput(g *Game, in Input) Input { return in }
func pilotAfterStep(g *Game)             {}
func pilotHUD(dst *image.RGBA)           {}
func pilotStop()                         {}
