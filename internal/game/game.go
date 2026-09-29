package game

import (
	"image"
	"math/rand"
	"sync"
	"time"

	"github.com/disintegration/gift"

	"github.com/sausheong/invadersapp/internal/assets"
)

// Width and Height are the logical frame size the whole game is
// drawn at; the platform code scales the window up from this.
const Width, Height = 400, 300

// QuitFunc is set by main to a thread-safe way of closing the app window.
// It's nil in tests, where quitting is a no-op.
var QuitFunc func()

// formation layout
const (
	aliensPerRow   = 8
	aliensStartCol = 100
	alienSize      = 30
	alienRow1Y     = 30
	alienRow2Y     = 55
	alienRow3Y     = 80

	waveRowStep     = 10 // px each new wave starts lower than the last
	maxWaveRowSteps = 4  // cap so waves don't start on top of the shields
)

// speeds and timing, all in units of one tick (the game runs at 50 ticks/sec)
const (
	CannonSpeed = 3 // px per tick while a direction key is held
	BeamSpeed   = 10
	BombSpeed   = 4
	CannonY     = 250
	BeamOffsetX = 7  // beam starts this far right of the cannon's left edge
	AlienStep   = 3  // px the formation moves sideways each move
	AlienDrop   = 10 // px the formation drops when it reverses at an edge

	baseMoveInterval = 10 // ticks between alien moves, wave 1, full formation
	minMoveInterval  = 2  // fastest the formation can ever move

	alienExplosionTicks  = 10 // how long a kill spark lingers
	cannonExplosionTicks = 50 // ~1s at 50 ticks/sec before respawn

	bombProbabilityPerColumn = 0.01 // per column, per tick

	UFOSpeed       = 2
	ufoY           = 14
	ufoMinCooldown = 600 // ticks (12s) before a ufo can appear again
	ufoJitterTicks = 600 // extra random delay on top of the minimum
	startingLives  = 3
)

type State int

const (
	StateTitle State = iota
	StatePlaying
	StateGameOver
)

// effect is a transient, purely cosmetic marker (an explosion spark) that
// step() schedules and render() draws; it carries no gameplay logic.
type effect struct {
	pos       image.Point
	ticksLeft int
}

// Event is an outcome reported through Game.onEvent.
type Event int

const (
	EvCannonHit Event = iota
	EvShotFired
	EvShotAlien
	EvShotUFO
	EvShotMissed
	EvShotShield
	EvShotCancelled
)

// emit reports an outcome to onEvent, if anyone is listening.
func (g *Game) emit(ev Event) {
	if g.onEvent != nil {
		g.onEvent(ev, g.beamFromPilot)
	}
}

// Game holds all gameplay state. step() advances it one tick from raw input
// (pure logic, no drawing); render() paints the current state into an image
// (no logic). Keeping the two separate is what makes step() unit-testable
// without a GUI or any real assets.
type Game struct {
	state  State
	paused bool

	aliens  []Sprite
	bombs   []Sprite
	shields []*Shield

	ufo       Sprite
	ufoActive bool
	ufoDir    int
	ufoTicks  int // ticks remaining before the ufo may next appear

	cannon        Sprite
	beam          Sprite
	beamActive    bool
	beamFromPilot bool // the beam in flight was fired by the autopilot

	// onEvent, if set, is told about outcomes the autopilot stats track:
	// cannon hits and how each shot ended. Nil in tests.
	onEvent func(ev Event, fromPilot bool)

	cannonExploding bool
	explodeTicks    int

	effects []effect

	alienDirection int
	animFrame      bool
	tick           int
	moveInterval   int
	nextMoveTick   int

	score     int
	highScore int
	lives     int
	wave      int

	rng *rand.Rand

	// persistHighScore is called with the new high score when a game ends
	// with a record; left nil by newGame() so unit tests never touch the real
	// filesystem. runGameLoop wires it to the real saveHighScore.
	persistHighScore func(int)
	savedHighScore   int // last high score handed to persistHighScore

	// images, injected by runGameLoop from assets.Image(); left nil in tests,
	// which never call render() with the default nil rng-seeded Game.
	sprites        image.Image
	background     image.Image
	startScreen    image.Image
	gameOverScreen image.Image
}

// newGame creates a fresh Game sitting on the title screen, with no high
// score loaded and no persistence wired up. It never touches the
// filesystem, images, or any goroutine, which is what keeps it hermetic
// for unit tests; runGameLoop is responsible for loading the real high
// score and wiring up persistHighScore before the game actually runs.
func newGame() *Game {
	return &Game{
		state: StateTitle,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// buildAliens creates a fresh three-row formation for the given wave. Later
// waves start further down the screen, capped so they never start on top of
// the shields.
func buildAliens(wave int) []Sprite {
	steps := wave - 1
	if steps > maxWaveRowSteps {
		steps = maxWaveRowSteps
	}
	yOffset := steps * waveRowStep

	aliens := make([]Sprite, 0, aliensPerRow*3)
	rows := []struct {
		y           int
		sprite, alt image.Rectangle
		points      int
	}{
		{alienRow1Y + yOffset, alien1Sprite, alien1aSprite, 30},
		{alienRow2Y + yOffset, alien2Sprite, alien2aSprite, 20},
		{alienRow3Y + yOffset, alien3Sprite, alien3aSprite, 10},
	}
	for _, row := range rows {
		col := 0
		for x := aliensStartCol; x < aliensStartCol+alienSize*aliensPerRow; x += alienSize {
			aliens = append(aliens, createAlien(x, row.y, row.sprite, row.alt, row.points, col))
			col++
		}
	}
	return aliens
}

// buildShields creates the 4 evenly-spaced destructible bunkers above the
// cannon row.
func buildShields() []*Shield {
	const n = 4
	y := CannonY - 40
	spacing := (Width - n*shieldWidth) / (n + 1)
	shields := make([]*Shield, 0, n)
	for i := 0; i < n; i++ {
		x := spacing*(i+1) + shieldWidth*i
		shields = append(shields, newShield(x, y))
	}
	return shields
}

// startNewGame resets everything: cannon, beam, bombs, score, lives, wave.
func (g *Game) startNewGame() {
	g.score = 0
	g.lives = startingLives
	g.wave = 1
	g.paused = false
	g.cannonExploding = false
	g.explodeTicks = 0
	g.effects = nil
	g.bombs = nil
	g.beamActive = false
	g.ufoActive = false
	g.ufoTicks = g.randomUFOInterval()
	g.tick = 0
	g.nextMoveTick = 0
	g.alienDirection = 1
	g.animFrame = false

	g.cannon = Sprite{
		size:     cannonSprite,
		Filter:   gift.New(gift.Crop(cannonSprite)),
		FilterE:  gift.New(gift.Crop(cannonExplode)),
		Position: image.Pt((Width-cannonSprite.Dx())/2, CannonY),
		Status:   true,
	}
	g.beam = Sprite{size: beamSprite, Filter: gift.New(gift.Crop(beamSprite))}

	g.aliens = buildAliens(g.wave)
	g.shields = buildShields()
	g.moveInterval = alienMoveInterval(len(g.aliens), len(g.aliens), g.wave)
}

// startNextWave keeps score, lives and shields but rebuilds the alien
// formation one step lower (capped) and faster.
func (g *Game) startNextWave() {
	g.wave++
	g.bombs = nil
	g.beamActive = false
	g.ufoActive = false
	g.alienDirection = 1
	g.tick = 0
	g.nextMoveTick = 0
	g.aliens = buildAliens(g.wave)
	g.moveInterval = alienMoveInterval(len(g.aliens), len(g.aliens), g.wave)
}

// step advances the game by exactly one tick given this tick's input. It
// never touches an image: all drawing happens in render().
func (g *Game) step(in Input) {
	switch g.state {
	case StateTitle, StateGameOver:
		if in.Quit {
			if QuitFunc != nil {
				QuitFunc()
			}
			return
		}
		if in.Start {
			g.startNewGame()
			g.state = StatePlaying
		}
	case StatePlaying:
		if in.Quit {
			g.saveHighScore()
			g.state = StateTitle
			return
		}
		if in.Pause {
			g.paused = !g.paused
		}
		if g.paused {
			return
		}
		g.stepPlaying(in)
	}
}

func (g *Game) stepPlaying(in Input) {
	g.tick++
	g.tickEffects()

	if g.cannonExploding {
		g.explodeTicks--
		if g.explodeTicks <= 0 {
			g.cannonExploding = false
			if g.lives <= 0 {
				g.finishGame()
				return
			}
			g.respawnCannon()
		}
		return
	}

	g.moveCannon(in)
	g.handleFire(in)
	g.moveBeam()
	g.moveAliens()
	g.updateShieldsUnderAliens()
	g.moveBombs()
	g.updateUFO()
	g.checkBeamHits()

	if g.state == StatePlaying && aliveAlienCount(g.aliens) == 0 {
		g.startNextWave()
	}
}

// moveCannon applies held-key movement and clamps the cannon inside the
// frame (BUG FIX: the original let it fly off either edge).
func (g *Game) moveCannon(in Input) {
	if in.Left {
		g.cannon.Position.X -= CannonSpeed
	}
	if in.Right {
		g.cannon.Position.X += CannonSpeed
	}
	minX, maxX := 0, Width-g.cannon.size.Dx()
	if g.cannon.Position.X < minX {
		g.cannon.Position.X = minX
	}
	if g.cannon.Position.X > maxX {
		g.cannon.Position.X = maxX
	}
}

// handleFire fires a single beam on a fresh press, as in the original
// (only one beam may be on screen at a time). assets.PlaySound only runs when a
// beam is actually fired, not on every fire keypress.
func (g *Game) handleFire(in Input) {
	if !in.Fire || g.beamActive {
		return
	}
	g.beam.Position = image.Pt(g.cannon.Position.X+BeamOffsetX, g.cannon.Position.Y-beamSprite.Dy())
	g.beamActive = true
	g.beamFromPilot = in.PilotFire
	g.emit(EvShotFired)
	assets.PlaySound("shoot")
}

func (g *Game) moveBeam() {
	if !g.beamActive {
		return
	}
	g.beam.Position.Y -= BeamSpeed
	if g.beam.Position.Y+beamSprite.Dy() < 0 {
		g.beamActive = false
		g.emit(EvShotMissed)
		return
	}
	for _, s := range g.shields {
		if !s.alive() {
			continue
		}
		if p, ok := shieldImpactPoint(g.beam.rect(), s); ok {
			s.damage(p, 2)
			g.beamActive = false
			g.emit(EvShotShield)
			return
		}
	}
}

// shieldImpactPoint returns the center of the overlap between rect (a
// bomb's or the beam's bounding box) and the shield's bounds, used as the
// crater center for a damage() call. Using the actual overlap (rather than
// an edge of rect, which can land outside the shield entirely) guarantees
// the crater is centered on real shield material.
func shieldImpactPoint(rect image.Rectangle, s *Shield) (image.Point, bool) {
	inter := rect.Intersect(s.bounds())
	if inter.Empty() {
		return image.Point{}, false
	}
	return image.Pt((inter.Min.X+inter.Max.X)/2, (inter.Min.Y+inter.Max.Y)/2), true
}

// moveAliens paces formation movement by moveInterval (which shrinks as
// aliens die or waves advance) and lets the lowest alien in each column
// drop bombs every tick.
func (g *Game) moveAliens() {
	if len(g.aliens) == 0 {
		return
	}
	if g.tick >= g.nextMoveTick {
		g.nextMoveTick = g.tick + g.moveInterval
		g.advanceFormation()
	}
	g.dropBombs()
}

// advanceFormation moves the whole alive formation one step, using only
// ALIVE aliens for edge detection and the game-over-by-invasion check (BUG
// FIX: the original always looked at aliens[0] and aliens[aliensPerRow-1],
// which could already be dead).
func (g *Game) advanceFormation() {
	if aliveAlienCount(g.aliens) == 0 {
		return
	}
	minX, maxX := aliveAlienXExtent(g.aliens)
	step := AlienStep * g.alienDirection

	hitEdge := (g.alienDirection > 0 && maxX+step > Width) ||
		(g.alienDirection < 0 && minX+step < 0)

	if hitEdge {
		g.alienDirection *= -1
		for i := range g.aliens {
			if g.aliens[i].Status {
				g.aliens[i].Position.Y += AlienDrop
			}
		}
	} else {
		for i := range g.aliens {
			if g.aliens[i].Status {
				g.aliens[i].Position.X += step
			}
		}
	}
	g.animFrame = !g.animFrame

	if lowestAliveAlienBottom(g.aliens) >= g.cannon.Position.Y {
		g.finishGame()
		return
	}

	g.moveInterval = alienMoveInterval(aliveAlienCount(g.aliens), len(g.aliens), g.wave)
}

// dropBombs lets only the lowest alive alien in each column drop a bomb.
func (g *Game) dropBombs() {
	for _, i := range lowestAlivePerColumn(g.aliens) {
		if g.rng.Float64() >= bombProbabilityPerColumn {
			continue
		}
		a := g.aliens[i]
		g.bombs = append(g.bombs, Sprite{
			size:     bombSprite,
			Filter:   gift.New(gift.Crop(bombSprite)),
			Position: image.Pt(a.Position.X+7, a.Position.Y+a.size.Dy()),
			Status:   true,
		})
	}
}

// moveBombs advances bombs, prunes any that leave the screen, damages
// shields they hit, and ends in a cannon hit if one connects.
func (g *Game) moveBombs() {
	kept := g.bombs[:0]
	for _, b := range g.bombs {
		b.Position.Y += BombSpeed
		if b.Position.Y > Height {
			continue // prune bombs that leave the screen
		}

		blocked := false
		for _, s := range g.shields {
			if !s.alive() {
				continue
			}
			if p, ok := shieldImpactPoint(b.rect(), s); ok {
				s.damage(p, 3)
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		if collide(b, g.cannon) {
			g.hitCannon() // clears every bomb, so stop here
			return
		}
		kept = append(kept, b)
	}
	g.bombs = kept
}

// hitCannon starts the cannon explosion: a life is lost, bombs and the beam
// are cleared, and (after explodeTicks) either a respawn or game over
// follows in stepPlaying.
func (g *Game) hitCannon() {
	if g.beamActive {
		g.emit(EvShotCancelled)
	}
	g.emit(EvCannonHit)
	g.cannonExploding = true
	g.explodeTicks = cannonExplosionTicks
	g.lives--
	g.bombs = nil
	g.beamActive = false
	assets.PlaySound("explosion")
}

// respawnCannon puts the cannon back at center with a clean slate.
func (g *Game) respawnCannon() {
	g.cannon.Position = image.Pt((Width-g.cannon.size.Dx())/2, CannonY)
	g.bombs = nil
	g.beamActive = false
}

// finishGame ends the round and saves the high score if it was beaten.
func (g *Game) finishGame() {
	g.state = StateGameOver
	g.saveHighScore()
}

// saveHighScore persists the high score once per game, only if it went up,
// so the game loop never writes to disk mid-play.
func (g *Game) saveHighScore() {
	if g.highScore > g.savedHighScore && g.persistHighScore != nil {
		g.persistHighScore(g.highScore)
	}
	g.savedHighScore = g.highScore
}

// updateShieldsUnderAliens erases shield material under any alien whose
// bounding box currently overlaps a shield, so the descending formation
// grinds through bunkers instead of just visually clipping them.
func (g *Game) updateShieldsUnderAliens() {
	for i := range g.aliens {
		if !g.aliens[i].Status {
			continue
		}
		r := g.aliens[i].rect()
		for _, s := range g.shields {
			if s.overlaps(r) {
				s.erase(r)
			}
		}
	}
}

func (g *Game) checkBeamHits() {
	if !g.beamActive {
		return
	}
	for i := range g.aliens {
		if !g.aliens[i].Status {
			continue
		}
		if collide(g.beam, g.aliens[i]) {
			g.killAlien(i)
			g.beamActive = false
			g.emit(EvShotAlien)
			return
		}
	}
	if g.ufoActive && collide(g.beam, g.ufo) {
		g.addScore(g.ufo.Points)
		g.spawnEffect(g.ufo.Position)
		g.ufoActive = false
		g.beamActive = false
		g.emit(EvShotUFO)
		assets.PlaySound("invaderkilled")
	}
}

func (g *Game) killAlien(i int) {
	a := &g.aliens[i]
	a.Status = false
	g.addScore(a.Points)
	g.spawnEffect(a.Position)
	assets.PlaySound("invaderkilled")
}

// addScore raises the live score and keeps the on-screen high score in sync
// the instant it's beaten, rather than only at game over. BUG FIX: the HUD
// and title/game-over screens all print g.highScore directly, so without
// this a player who passed the previous high score mid-game would see a
// stale "HIGH" value until they eventually lost their last life.
func (g *Game) addScore(n int) {
	g.score += n
	if g.score > g.highScore {
		g.highScore = g.score
	}
}

func (g *Game) spawnEffect(pos image.Point) {
	g.effects = append(g.effects, effect{pos: pos, ticksLeft: alienExplosionTicks})
}

func (g *Game) tickEffects() {
	kept := g.effects[:0]
	for _, e := range g.effects {
		e.ticksLeft--
		if e.ticksLeft > 0 {
			kept = append(kept, e)
		}
	}
	g.effects = kept
}

func (g *Game) updateUFO() {
	if g.ufoActive {
		g.ufo.Position.X += UFOSpeed * g.ufoDir
		if g.ufo.Position.X < -20 || g.ufo.Position.X > Width+20 {
			g.ufoActive = false
			g.ufoTicks = g.randomUFOInterval()
		}
		return
	}
	g.ufoTicks--
	if g.ufoTicks <= 0 {
		g.spawnUFO()
	}
}

func (g *Game) spawnUFO() {
	dir, x := 1, -20
	if g.rng.Intn(2) == 0 {
		dir, x = -1, Width+20
	}
	points := []int{50, 100, 150, 300}[g.rng.Intn(4)]
	g.ufo = Sprite{size: image.Rect(0, 0, 16, 7), Position: image.Pt(x, ufoY), Status: true, Points: points}
	g.ufoDir = dir
	g.ufoActive = true
	// Reset the cooldown here, at the single point a UFO's lifecycle starts,
	// so however it later becomes inactive (beam kill in checkBeamHits,
	// startNextWave forcing ufoActive=false on a wave clear, or flying fully
	// off-screen in updateUFO) the next spawn is correctly delayed. BUG FIX:
	// previously only the off-screen-exit path reset ufoTicks, so killing a
	// UFO (or clearing a wave while one was on screen) left ufoTicks at the
	// ~0 value that triggered this spawn, causing an immediate re-spawn on
	// the very next tick and letting a player chain-kill UFOs for unbounded
	// score.
	g.ufoTicks = g.randomUFOInterval()
}

func (g *Game) randomUFOInterval() int {
	return ufoMinCooldown + g.rng.Intn(ufoJitterTicks)
}

// alienMoveInterval is how many ticks separate alien moves: fewer aliens
// alive, or a later wave, both make the formation move faster (a lower
// interval), down to a floor so it never becomes literally instant.
func alienMoveInterval(alive, total, wave int) int {
	if total <= 0 {
		return baseMoveInterval
	}
	base := baseMoveInterval - (wave - 1)
	if base < minMoveInterval+4 {
		base = minMoveInterval + 4
	}
	killedFraction := float64(total-alive) / float64(total)
	interval := base - int(killedFraction*float64(base-minMoveInterval))
	if interval < minMoveInterval {
		interval = minMoveInterval
	}
	return interval
}

func aliveAlienCount(aliens []Sprite) int {
	n := 0
	for _, a := range aliens {
		if a.Status {
			n++
		}
	}
	return n
}

// aliveAlienXExtent returns the leftmost and rightmost X (min and max+width)
// among ALIVE aliens only (BUG FIX: formation edge detection used to look at
// fixed indices regardless of whether those aliens were still alive).
func aliveAlienXExtent(aliens []Sprite) (minX, maxX int) {
	minX, maxX = 1<<30, -(1 << 30)
	for _, a := range aliens {
		if !a.Status {
			continue
		}
		if a.Position.X < minX {
			minX = a.Position.X
		}
		if r := a.Position.X + a.size.Dx(); r > maxX {
			maxX = r
		}
	}
	return
}

// lowestAliveAlienBottom returns the largest Y+height among ALIVE aliens, or
// -1 if none are alive.
func lowestAliveAlienBottom(aliens []Sprite) int {
	bottom := -1
	for _, a := range aliens {
		if !a.Status {
			continue
		}
		if b := a.Position.Y + a.size.Dy(); b > bottom {
			bottom = b
		}
	}
	return bottom
}

// lowestAlivePerColumn returns, for each formation column that still has a
// survivor, the index of its lowest (largest Y) alive alien.
func lowestAlivePerColumn(aliens []Sprite) []int {
	best := map[int]int{}
	for i, a := range aliens {
		if !a.Status {
			continue
		}
		cur, ok := best[a.Column]
		if !ok || aliens[cur].Position.Y < a.Position.Y {
			best[a.Column] = i
		}
	}
	idxs := make([]int, 0, len(best))
	for _, i := range best {
		idxs = append(idxs, i)
	}
	return idxs
}

// Start starts the single game loop goroutine, idempotently: calling it
// more than once (main only ever calls it once, but this keeps it safe) has
// no extra effect.
var gameOnce sync.Once

func Start() {
	gameOnce.Do(func() {
		g := newGame()
		go runGameLoop(g)
	})
}

// runGameLoop is the one and only game goroutine: a fixed-rate ticker reads
// input, steps the simulation, renders it, and publishes the frame. Real
// asset loading and high-score persistence are wired up here rather than in
// newGame(), so unit tests can build a Game without touching the
// filesystem.
func runGameLoop(g *Game) {
	g.sprites = assets.Image("images/sprites.png")
	g.background = assets.Image("images/bg.png")
	g.startScreen = assets.Image("images/start.png")
	g.gameOverScreen = assets.Image("images/gameover.png")

	g.highScore = loadHighScore()
	g.savedHighScore = g.highScore
	g.persistHighScore = saveHighScore
	if autopilot != nil {
		g.onEvent = autopilot.Event
	}

	ticker := time.NewTicker(time.Second / 50)
	defer ticker.Stop()
	for range ticker.C {
		in := input.snapshot()
		if autopilot != nil {
			in = autopilot.Input(g.view(), in)
		}
		g.step(in)
		if autopilot != nil {
			autopilot.Observe(g.view())
		}

		dst := image.NewRGBA(image.Rect(0, 0, Width, Height))
		g.render(dst)
		createFrame(dst)
	}
}
