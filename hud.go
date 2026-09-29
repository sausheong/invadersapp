package main

import (
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/gift"
	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
)

var hudWhite = color.RGBA{255, 255, 255, 255}

// printLine draws a line of bitmap text onto img at (x,y) in the given
// color, using the built-in inconsolata font (no font files to load).
func printLine(img *image.RGBA, x, y int, label string, col color.RGBA) {
	point := fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: inconsolata.Bold8x16,
		Dot:  point,
	}
	d.DrawString(label)
}

// drawHUD renders the score (top-left), high score (top-center) and
// remaining lives (bottom-left, as small text markers) during play.
func (g *Game) drawHUD(dst *image.RGBA) {
	printLine(dst, 4, 12, fmt.Sprintf("SCORE %d", g.score), hudWhite)

	hs := fmt.Sprintf("HIGH %d", g.highScore)
	printLine(dst, gameWidth/2-len(hs)*4, 12, hs, hudWhite)

	// Remaining lives as small cannon-sprite icons rather than a placeholder
	// letter glyph, echoing the original arcade's row of ship icons.
	if g.sprites != nil {
		f := gift.New(gift.Crop(cannonSprite))
		for i := 0; i < g.lives; i++ {
			f.DrawAt(dst, g.sprites, image.Pt(4+i*22, gameHeight-16), gift.OverOperator)
		}
	}
}
