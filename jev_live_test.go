//go:build jev && live

package main

import (
	"context"
	"testing"
	"time"
)

// Live scenario checks against the real TypeSafe API. Run with:
//
//	go test -tags jev,live -run TestJev -v .
func TestJevScenarios(t *testing.T) {
	key := jevAPIKey()
	if key == "" {
		t.Skip("no TYPESAFE_API_KEY")
	}
	c := newJevClient(key)
	still := func(g *Game) { g.moveInterval, g.nextMoveTick = 1000, g.tick+1000 }
	cases := []struct {
		name      string
		setup     func(g *Game)
		wantSpot  func(o observation, s spot) bool
		fire      bool
		checkFire bool
	}{
		{"bomb about to land on you", func(g *Game) {
			clearField(g)
			g.bombs = []Sprite{bombAt(g.cannon.Position.X+4, cannonY-80)}
		}, func(o observation, s spot) bool { return s.Safety == safe }, false, false},
		{"alien far to the left", func(g *Game) {
			clearField(g)
			still(g)
			g.aliens = []Sprite{createAlien(40, 100, alien1Sprite, alien1aSprite, 30, 0)}
		}, func(o observation, s spot) bool { return s.Safety == safe && s.Fire == hitAlien }, false, false},
		{"aligned under an alien", func(g *Game) {
			clearField(g)
			still(g)
			g.aliens = []Sprite{createAlien(g.cannon.Position.X-2, 100, alien1Sprite, alien1aSprite, 30, 0)}
		}, func(o observation, s spot) bool { return s.Key == stayKey }, true, true},
		{"bombs either side, alien overhead", func(g *Game) {
			clearField(g)
			still(g)
			x := g.cannon.Position.X
			g.aliens = []Sprite{createAlien(x-2, 100, alien1Sprite, alien1aSprite, 30, 0)}
			g.bombs = []Sprite{bombAt(x-30, cannonY-60), bombAt(x+40, cannonY-60)}
		}, func(o observation, s spot) bool { return s.Safety == safe }, true, true},
		{"under your own shield", func(g *Game) {
			clearField(g)
			still(g)
			x := g.cannon.Position.X
			g.shields = []*Shield{newShield(x-2, cannonY-40)}
			g.aliens = []Sprite{createAlien(x-2, 100, alien1Sprite, alien1aSprite, 30, 0)}
		}, nil, false, true},
	}
	for _, tc := range cases {
		g := newPlayingGame(t)
		tc.setup(g)
		o := observe(g, -1, 15)
		start := time.Now()
		ans, err := c.ask(context.Background(), o.State, o.questions())
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		pick, fr := ans["spot"], ans["fire"]
		var chosen spot
		for _, s := range o.Spots {
			if s.Key == pick.Choice {
				chosen = s
			}
		}
		t.Logf("%-34s -> %s conf=%.2f fire=%.2f (%dms)\n      %s", tc.name, chosen.Key, pick.Confidence, fr.Noul, time.Since(start).Milliseconds(), chosen.Text)
		if tc.wantSpot != nil && !tc.wantSpot(o, chosen) {
			t.Errorf("%s: picked %s: %s", tc.name, chosen.Key, chosen.Text)
		}
		if tc.checkFire && (fr.Noul >= fireThreshold) != tc.fire {
			t.Errorf("%s: fire = %.2f, want %v", tc.name, fr.Noul, tc.fire)
		}
	}
}
