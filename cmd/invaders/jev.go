//go:build jev

package main

import (
	"flag"

	"github.com/sausheong/invadersapp/internal/autopilot"
	"github.com/sausheong/invadersapp/internal/game"
)

// Built with -tags jev: plug the Jev autopilot into the game. Without the
// tag this file isn't compiled and the autopilot isn't even imported.

var jevFlag = flag.Bool("jev", false, "start with the Jev autopilot on (needs TYPESAFE_API_KEY)")

func init() {
	p := autopilot.New()
	startHooks = append(startHooks, func() {
		game.SetAutopilot(p)
		if *jevFlag {
			p.Toggle()
		}
	})
	stopHooks = append(stopHooks, p.Stop)
}
