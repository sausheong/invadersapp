package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/gift"
)

var hudRed = color.RGBA{220, 40, 40, 255}
var hudGreen = color.RGBA{80, 220, 80, 255}
var shieldGreen = color.RGBA{40, 220, 90, 255}
var ufoRed = color.RGBA{220, 40, 40, 255}
var sparkOrange = color.RGBA{255, 160, 0, 255}

// render paints the current game state into dst (Width x Height).
// It's the only place that touches images: step() never draws anything,
// which is what makes step() unit-testable without a GUI or real assets.
func (g *Game) render(dst *image.RGBA) {
	switch g.state {
	case StateTitle:
		g.renderTitle(dst)
	case StateGameOver:
		g.renderGameOver(dst)
	default:
		g.renderPlaying(dst)
	}
}

func (g *Game) renderTitle(dst *image.RGBA) {
	if g.startScreen != nil {
		gift.New().Draw(dst, g.startScreen)
	}
	// start.png already bakes in its own "PRESS 'S' TO BEGIN" prompt around
	// y~210-230; drawing our own "Press 's' to start" text there (as before)
	// overlapped it character-for-character into an unreadable smear. Print
	// only the high score, well clear of that band near the bottom.
	printLine(dst, 106, 278, fmt.Sprintf("High score: %d", g.highScore), hudWhite)
}

func (g *Game) renderGameOver(dst *image.RGBA) {
	if g.gameOverScreen != nil {
		gift.New().Draw(dst, g.gameOverScreen)
	}
	printLine(dst, 120, 200, fmt.Sprintf("Your score: %d", g.score), hudRed)
	printLine(dst, 120, 218, fmt.Sprintf("High score: %d", g.highScore), hudRed)
	printLine(dst, 104, 240, "Press 's' to play again", hudRed)
	printLine(dst, 137, 258, "Press 'q' to quit", hudRed)
}

func (g *Game) renderPlaying(dst *image.RGBA) {
	if g.background != nil {
		gift.New().Draw(dst, g.background)
	}

	g.drawShields(dst)

	if g.sprites != nil {
		for _, a := range g.aliens {
			if !a.Status {
				continue
			}
			f := a.Filter
			if g.animFrame && a.FilterA != nil {
				f = a.FilterA
			}
			f.DrawAt(dst, g.sprites, a.Position, gift.OverOperator)
		}
	}

	g.drawEffects(dst)

	if g.sprites != nil {
		for _, b := range g.bombs {
			b.Filter.DrawAt(dst, g.sprites, b.Position, gift.OverOperator)
		}
	}

	if g.ufoActive {
		drawUFO(dst, g.ufo.Position)
	}

	if g.beamActive && g.sprites != nil {
		g.beam.Filter.DrawAt(dst, g.sprites, g.beam.Position, gift.OverOperator)
	}

	if g.sprites != nil {
		if g.cannonExploding {
			g.cannon.FilterE.DrawAt(dst, g.sprites, g.cannon.Position, gift.OverOperator)
		} else {
			g.cannon.Filter.DrawAt(dst, g.sprites, g.cannon.Position, gift.OverOperator)
		}
	}

	g.drawHUD(dst)
}

// drawShields paints each shield's intact pixels as small green blocks.
func (g *Game) drawShields(dst *image.RGBA) {
	for _, s := range g.shields {
		for row := 0; row < shieldHeight; row++ {
			for col := 0; col < shieldWidth; col++ {
				if s.Pixels[row][col] {
					dst.SetRGBA(s.Position.X+col, s.Position.Y+row, shieldGreen)
				}
			}
		}
	}
}

// drawEffects paints transient explosion markers (alien or ufo kills) as a
// small burst of pixels; purely cosmetic, kept separate from game logic.
func (g *Game) drawEffects(dst *image.RGBA) {
	for _, e := range g.effects {
		for y := 0; y < 8; y++ {
			for x := 0; x < 16; x++ {
				if (x+y)%3 == 0 {
					dst.SetRGBA(e.pos.X+x, e.pos.Y+y, sparkOrange)
				}
			}
		}
	}
}

// drawUFO paints a simple procedurally-generated red saucer, since the
// sprite sheet has no spare room for one.
func drawUFO(dst *image.RGBA, pos image.Point) {
	for y := 0; y < 7; y++ {
		for x := 0; x < 16; x++ {
			in := true
			switch y {
			case 0, 6:
				in = x >= 5 && x < 11
			case 1, 5:
				in = x >= 2 && x < 14
			}
			if in {
				dst.SetRGBA(pos.X+x, pos.Y+y, ufoRed)
			}
		}
	}
}
