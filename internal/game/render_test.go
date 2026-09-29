package game

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestRenderSnapshots exercises the real rendering path end to end, using
// the actual game assets decoded straight from disk (never assets.Image,
// which belongs to the platform code and isn't available to a hermetic
// test). It's skipped unless INVADERS_SNAPSHOT_DIR is set, since its whole
// purpose is to write PNG snapshots for a human (or another verifier) to
// look at, not to assert anything itself.
func TestRenderSnapshots(t *testing.T) {
	dir := os.Getenv("INVADERS_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("INVADERS_SNAPSHOT_DIR not set; skipping snapshot generation")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	load := func(name string) image.Image {
		f, err := os.Open(filepath.Join("..", "assets", "public", "images", name))
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		return img
	}

	g := newGame()
	g.sprites = load("sprites.png")
	g.background = load("bg.png")
	g.startScreen = load("start.png")
	g.gameOverScreen = load("gameover.png")

	save := func(name string) {
		t.Helper()
		dst := image.NewRGBA(image.Rect(0, 0, Width, Height))
		g.render(dst)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		defer f.Close()
		if err := png.Encode(f, dst); err != nil {
			t.Fatalf("encode %s: %v", name, err)
		}
	}

	// title screen
	save("title.png")

	// start playing and run a few hundred ticks, moving and firing
	g.step(Input{Start: true})
	for i := 0; i < 300; i++ {
		in := Input{}
		if i%15 == 0 {
			in.Fire = true
		}
		if (i/30)%2 == 0 {
			in.Right = true
		} else {
			in.Left = true
		}
		g.step(in)
		switch i {
		case 40:
			save("play-early.png")
		case 150:
			save("play-mid.png")
		case 280:
			save("play-late.png")
		}
	}

	// force a UFO on screen for a dedicated snapshot (natural spawn cooldown
	// is 600+ ticks, longer than this test's scripted play).
	g.ufoActive = true
	g.ufoDir = 1
	g.ufo = Sprite{size: image.Rect(0, 0, 16, 7), Position: image.Pt(Width/2-8, ufoY), Status: true, Points: 100}
	g.step(Input{})
	save("play-ufo.png")
	g.ufoActive = false

	// force a real cannon hit deterministically (calling hitCannon directly
	// rather than hoping a synthesized bomb collides within N ticks) so the
	// explosion sprite and the respawn are both actually exercised.
	g.cannonExploding = false
	g.lives = 2
	g.hitCannon()
	g.step(Input{})
	save("cannon-hit.png")
	for i := 1; i < cannonExplosionTicks; i++ {
		g.step(Input{})
	}
	save("respawned.png")

	// force a real, deterministic game over: one life left, hit again, and
	// run the explosion out so finishGame() actually fires and state
	// becomes StateGameOver (the previous version of this test relied on a
	// bomb colliding with the cannon within cannonExplosionTicks, which
	// could silently fail to land if a natural mid-air bomb had already put
	// the cannon into its explosion state during the scripted play above,
	// producing a "gameover.png" that was actually still StatePlaying).
	g.lives = 1
	g.hitCannon()
	for i := 0; i < cannonExplosionTicks; i++ {
		g.step(Input{})
	}
	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver after running out lives, got state=%v", g.state)
	}
	save("gameover.png")
}
