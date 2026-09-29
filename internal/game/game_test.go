package game

import (
	"image"
	"testing"

	"github.com/disintegration/gift"
)

func newPlayingGame(t *testing.T) *Game {
	t.Helper()
	g := newGame()
	g.step(Input{Start: true})
	if g.state != StatePlaying {
		t.Fatalf("expected state StatePlaying after Start, got %v", g.state)
	}
	return g
}

// --- edge detection must ignore dead aliens ---

func TestAliveAlienXExtentIgnoresDead(t *testing.T) {
	aliens := []Sprite{
		{Status: false, Position: image.Pt(0, 0), size: image.Rect(0, 0, 20, 14)}, // dead, would be the leftmost
		{Status: true, Position: image.Pt(50, 0), size: image.Rect(0, 0, 20, 14)},
		{Status: false, Position: image.Pt(390, 0), size: image.Rect(0, 0, 20, 14)}, // dead, would be the rightmost
		{Status: true, Position: image.Pt(100, 0), size: image.Rect(0, 0, 20, 14)},
	}
	minX, maxX := aliveAlienXExtent(aliens)
	if minX != 50 {
		t.Errorf("expected minX 50 (ignoring dead alien at 0), got %d", minX)
	}
	if maxX != 120 {
		t.Errorf("expected maxX 120 (ignoring dead alien at 390), got %d", maxX)
	}
}

func TestLowestAliveAlienBottomIgnoresDead(t *testing.T) {
	aliens := []Sprite{
		{Status: true, Position: image.Pt(0, 10), size: image.Rect(0, 0, 20, 14)},
		{Status: false, Position: image.Pt(0, 200), size: image.Rect(0, 0, 20, 14)}, // dead, would dominate
	}
	if got := lowestAliveAlienBottom(aliens); got != 24 {
		t.Errorf("expected 24 (10+14, ignoring the dead alien at 200), got %d", got)
	}
}

// --- wave advance when all aliens are dead ---

func TestWaveAdvancesWhenAllAliensDead(t *testing.T) {
	g := newPlayingGame(t)
	startWave := g.wave
	for i := range g.aliens {
		g.aliens[i].Status = false
	}
	g.step(Input{})
	if g.wave != startWave+1 {
		t.Fatalf("expected wave %d, got %d", startWave+1, g.wave)
	}
	if aliveAlienCount(g.aliens) != len(g.aliens) {
		t.Errorf("expected a fresh, fully-alive formation for the new wave")
	}
	if g.state != StatePlaying {
		t.Errorf("expected to still be playing after a wave clear, got state %v", g.state)
	}
}

// --- lives, hits, respawn, game over ---

func bombOnCannon(g *Game) Sprite {
	return Sprite{
		size:     bombSprite,
		Filter:   gift.New(gift.Crop(bombSprite)),
		Position: g.cannon.Position,
		Status:   true,
	}
}

func TestCannonHitDecrementsLivesAndRespawns(t *testing.T) {
	g := newPlayingGame(t)
	startLives := g.lives

	g.bombs = []Sprite{bombOnCannon(g)}
	g.step(Input{})

	if !g.cannonExploding {
		t.Fatal("expected cannon to be exploding right after a hit")
	}
	if g.lives != startLives-1 {
		t.Fatalf("expected lives %d, got %d", startLives-1, g.lives)
	}
	if len(g.bombs) != 0 {
		t.Errorf("expected bombs to be cleared on a hit")
	}

	for i := 0; i < cannonExplosionTicks; i++ {
		g.step(Input{})
	}
	if g.cannonExploding {
		t.Error("expected the cannon explosion to be over")
	}
	if g.state != StatePlaying {
		t.Errorf("expected to respawn into StatePlaying, got %v", g.state)
	}
	wantX := (Width - g.cannon.size.Dx()) / 2
	if g.cannon.Position.X != wantX {
		t.Errorf("expected cannon respawned at center x=%d, got %d", wantX, g.cannon.Position.X)
	}
}

func TestGameOverWhenLivesReachZero(t *testing.T) {
	g := newPlayingGame(t)
	for lives := startingLives; lives > 0; lives-- {
		g.bombs = []Sprite{bombOnCannon(g)}
		g.step(Input{})
		for i := 0; i < cannonExplosionTicks; i++ {
			g.step(Input{})
		}
	}
	if g.state != StateGameOver {
		t.Fatalf("expected StateGameOver once lives run out, got %v", g.state)
	}
}

func TestGameOverByInvasion(t *testing.T) {
	g := newPlayingGame(t)
	for i := range g.aliens {
		g.aliens[i].Position.Y = g.cannon.Position.Y
	}
	g.advanceFormation()
	if g.state != StateGameOver {
		t.Fatalf("expected invasion to end the game, got state %v", g.state)
	}
}

// --- cannon clamping ---

func TestCannonClampedInsideFrame(t *testing.T) {
	g := newPlayingGame(t)
	for i := 0; i < 200; i++ {
		g.step(Input{Left: true})
	}
	if g.cannon.Position.X < 0 {
		t.Errorf("expected cannon clamped at x>=0, got %d", g.cannon.Position.X)
	}
	for i := 0; i < 200; i++ {
		g.step(Input{Right: true})
	}
	maxX := Width - g.cannon.size.Dx()
	if g.cannon.Position.X > maxX {
		t.Errorf("expected cannon clamped at x<=%d, got %d", maxX, g.cannon.Position.X)
	}
}

// --- bomb pruning ---

func TestBombsOffScreenArePruned(t *testing.T) {
	g := newPlayingGame(t)
	g.bombs = []Sprite{{
		size:     bombSprite,
		Filter:   gift.New(gift.Crop(bombSprite)),
		Position: image.Pt(200, Height+5),
		Status:   true,
	}}
	g.moveBombs()
	if len(g.bombs) != 0 {
		t.Errorf("expected off-screen bomb to be pruned, got %d bombs left", len(g.bombs))
	}
}

// --- only the lowest alive alien per column may bomb ---

func TestLowestAlivePerColumn(t *testing.T) {
	aliens := []Sprite{
		{Column: 0, Status: true, Position: image.Pt(10, 30)},  // column 0, higher up
		{Column: 0, Status: true, Position: image.Pt(10, 80)},  // column 0, lowest -> should be picked
		{Column: 1, Status: true, Position: image.Pt(40, 55)},  // column 1, only survivor
		{Column: 1, Status: false, Position: image.Pt(40, 90)}, // column 1, dead, must be ignored
	}
	idxs := lowestAlivePerColumn(aliens)
	if len(idxs) != 2 {
		t.Fatalf("expected 2 columns with a bomber, got %d", len(idxs))
	}
	picked := map[int]bool{}
	for _, i := range idxs {
		picked[i] = true
	}
	if !picked[1] {
		t.Error("expected the lower alien (index 1) to be the column-0 bomber")
	}
	if !picked[2] {
		t.Error("expected the only survivor (index 2) to be the column-1 bomber")
	}
	if picked[0] {
		t.Error("did not expect the higher, non-lowest alien to be a bomber")
	}
}

// --- shield damage from gameplay (bomb impact) ---

func TestBombDamagesShield(t *testing.T) {
	g := newPlayingGame(t)
	if len(g.shields) == 0 {
		t.Fatal("expected shields to be present")
	}
	s := g.shields[0]
	if !s.alive() {
		t.Fatal("expected a fresh shield to be alive")
	}
	center := image.Pt(s.Position.X+shieldWidth/2, s.Position.Y+shieldHeight/2)
	g.bombs = []Sprite{{
		size:     bombSprite,
		Filter:   gift.New(gift.Crop(bombSprite)),
		Position: image.Pt(center.X, center.Y-BombSpeed), // moveBombs adds BombSpeed before checking
		Status:   true,
	}}
	before := countIntact(s)
	g.moveBombs()
	after := countIntact(s)
	if after >= before {
		t.Errorf("expected the shield to lose pixels after a bomb impact, before=%d after=%d", before, after)
	}
}

func countIntact(s *Shield) int {
	n := 0
	for _, row := range s.Pixels {
		for _, v := range row {
			if v {
				n++
			}
		}
	}
	return n
}

// --- speed-up as fewer aliens remain ---

func TestAlienMoveIntervalSpeedsUpWithFewerAliens(t *testing.T) {
	full := alienMoveInterval(24, 24, 1)
	half := alienMoveInterval(12, 24, 1)
	almostGone := alienMoveInterval(1, 24, 1)
	if !(full > half && half > almostGone) {
		t.Errorf("expected move interval to strictly decrease as aliens die: full=%d half=%d almostGone=%d", full, half, almostGone)
	}
	if almostGone < minMoveInterval {
		t.Errorf("expected move interval never to drop below the floor %d, got %d", minMoveInterval, almostGone)
	}
}

func TestAlienMoveIntervalSpeedsUpWithWave(t *testing.T) {
	wave1 := alienMoveInterval(24, 24, 1)
	wave3 := alienMoveInterval(24, 24, 3)
	if wave3 >= wave1 {
		t.Errorf("expected a later wave to move faster: wave1=%d wave3=%d", wave1, wave3)
	}
}

// --- pause stops the simulation ---

func TestPauseStopsSimulation(t *testing.T) {
	g := newPlayingGame(t)
	g.step(Input{Pause: true})
	if !g.paused {
		t.Fatal("expected pause to be toggled on")
	}
	before := g.cannon.Position
	beforeTick := g.tick
	for i := 0; i < 10; i++ {
		g.step(Input{Right: true})
	}
	if g.cannon.Position != before {
		t.Errorf("expected cannon not to move while paused, before=%v after=%v", before, g.cannon.Position)
	}
	if g.tick != beforeTick {
		t.Errorf("expected the tick counter to be frozen while paused")
	}

	g.step(Input{Pause: true})
	if g.paused {
		t.Fatal("expected pause to be toggled off")
	}
	g.step(Input{Right: true})
	if g.cannon.Position.X <= before.X {
		t.Errorf("expected the cannon to move again after unpausing")
	}
}

// --- quit during play returns to the title screen; quit on title/gameover quits ---

func TestQuitDuringPlayReturnsToTitle(t *testing.T) {
	g := newPlayingGame(t)
	g.step(Input{Quit: true})
	if g.state != StateTitle {
		t.Fatalf("expected quitting during play to return to the title screen, got %v", g.state)
	}
}

func TestQuitOnTitleCallsQuitFunc(t *testing.T) {
	called := false
	old := QuitFunc
	QuitFunc = func() { called = true }
	defer func() { QuitFunc = old }()

	g := newGame()
	g.step(Input{Quit: true})
	if !called {
		t.Error("expected QuitFunc to be called when quitting from the title screen")
	}
}
