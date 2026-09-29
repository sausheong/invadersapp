//go:build live

package autopilot

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/sausheong/invadersapp/internal/game"
	"github.com/sausheong/invadersapp/internal/typesafe"
)

// Live scenario checks against the real TypeSafe API. Run with:
//
//	go test -tags live -run TestJev -v ./internal/autopilot
func TestJevScenarios(t *testing.T) {
	t.Chdir("../..") // find a .env at the repo root
	key := apiKey()
	if key == "" {
		t.Skip("no TYPESAFE_API_KEY")
	}
	c := typesafe.NewClient(key)
	cases := []struct {
		name      string
		setup     func(v *game.View)
		wantSpot  func(s spot) bool
		fire      bool
		checkFire bool
		escape    string // expected escape direction, if checked
	}{
		{"bomb about to land on you", func(v *game.View) {
			v.Bombs = []image.Rectangle{bombAt(v.Cannon.Min.X+4, game.CannonY-80)}
		}, func(s spot) bool { return s.Safety == safe }, false, false, ""},
		{"alien far to the left", func(v *game.View) {
			v.Aliens = []game.Alien{alienAt(40, 100, 0)}
		}, func(s spot) bool { return s.Safety == safe && s.Fire == hitAlien }, false, false, ""},
		{"aligned under an alien", func(v *game.View) {
			v.Aliens = []game.Alien{alienAt(v.Cannon.Min.X-2, 100, 0)}
		}, func(s spot) bool { return s.Key == stayKey }, true, true, ""},
		{"bombs either side, alien overhead", func(v *game.View) {
			x := v.Cannon.Min.X
			v.Aliens = []game.Alien{alienAt(x-2, 100, 0)}
			v.Bombs = []image.Rectangle{bombAt(x-30, game.CannonY-60), bombAt(x+40, game.CannonY-60)}
		}, func(s spot) bool { return s.Safety == safe }, true, true, ""},
		{"under your own shield", func(v *game.View) {
			x := v.Cannon.Min.X
			v.Shields = []image.Rectangle{shieldAt(x-2, game.CannonY-40)}
			v.Aliens = []game.Alien{alienAt(x-2, 100, 0)}
		}, nil, false, true, ""},
		{"escape with the left wall beside you", func(v *game.View) {
			v.Cannon = image.Rect(0, game.CannonY, game.CannonSize.X, game.CannonY+game.CannonSize.Y)
		}, nil, false, false, escapeRight},
		{"escape with a bomb landing on your right", func(v *game.View) {
			x := v.Cannon.Min.X
			for i := 1; i <= escapeSteps; i++ {
				v.Bombs = append(v.Bombs, bombAt(x+i*spotStep+4, game.CannonY-30))
			}
		}, nil, false, false, escapeLeft},
	}
	for _, tc := range cases {
		v := view()
		tc.setup(&v)
		o := observe(v, -1, 15)
		start := time.Now()
		ans, err := c.Ask(context.Background(), o.State, o.questions())
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		pick, fr, esc := ans["spot"], ans["fire"], ans["escape"]
		var chosen spot
		for _, s := range o.Spots {
			if s.Key == pick.Choice {
				chosen = s
			}
		}
		t.Logf("%-40s -> %s conf=%.2f fire=%.2f escape=%s (%dms)\n      %s", tc.name, chosen.Key, pick.Confidence, fr.Noul, esc.Choice, time.Since(start).Milliseconds(), chosen.Text)
		if tc.escape != "" && esc.Choice != tc.escape {
			t.Errorf("%s: escape = %s, want %s (%v)", tc.name, esc.Choice, tc.escape, o.Escape)
		}
		if tc.wantSpot != nil && !tc.wantSpot(chosen) {
			t.Errorf("%s: picked %s: %s", tc.name, chosen.Key, chosen.Text)
		}
		if tc.checkFire && (fr.Noul >= fireThreshold) != tc.fire {
			t.Errorf("%s: fire = %.2f, want %v", tc.name, fr.Noul, tc.fire)
		}
	}
}
