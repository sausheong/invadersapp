package main

import (
	"image"

	"github.com/disintegration/gift"
)

// sprite rectangles within the sprite sheet (public/images/sprites.png)
var cannonSprite = image.Rect(20, 47, 38, 59)
var cannonExplode = image.Rect(0, 47, 16, 57)
var alien1Sprite = image.Rect(0, 0, 20, 14)
var alien1aSprite = image.Rect(20, 0, 40, 14)
var alien2Sprite = image.Rect(0, 14, 20, 26)
var alien2aSprite = image.Rect(20, 14, 40, 26)
var alien3Sprite = image.Rect(0, 27, 20, 40)
var alien3aSprite = image.Rect(20, 27, 40, 40)
var alienExplode = image.Rect(0, 60, 16, 68)
var beamSprite = image.Rect(20, 60, 22, 65)
var bombSprite = image.Rect(0, 70, 10, 79)

// Sprite represents a drawable game entity: a crop of the sprite sheet (or,
// for procedurally-drawn entities like the UFO and shields, no filter at
// all) plus its position and status. size is used for both drawing and
// collision detection, so it must always match what's actually drawn.
type Sprite struct {
	size     image.Rectangle // sprite bounding box, used for collisions
	Filter   *gift.GIFT      // normal filter used to draw the sprite (nil if procedurally drawn)
	FilterA  *gift.GIFT      // alternate filter (animation frame), may be nil
	FilterE  *gift.GIFT      // exploded filter, may be nil
	Position image.Point     // top-left position of the sprite
	Status   bool            // alive or dead
	Points   int             // score awarded if destroyed
	Column   int             // formation column (aliens only), used to pick the bomber per column
}

// createAlien builds an alien Sprite at (x,y) with its two animation frames
// and the shared explosion frame.
func createAlien(x, y int, sprite, alt image.Rectangle, points, column int) (s Sprite) {
	s = Sprite{
		size:     sprite,
		Filter:   gift.New(gift.Crop(sprite)),
		FilterA:  gift.New(gift.Crop(alt)),
		FilterE:  gift.New(gift.Crop(alienExplode)),
		Position: image.Pt(x, y),
		Status:   true,
		Points:   points,
		Column:   column,
	}
	return
}

// rect returns a sprite's current bounding box in world coordinates.
func (s Sprite) rect() image.Rectangle {
	return image.Rect(s.Position.X, s.Position.Y, s.Position.X+s.size.Dx(), s.Position.Y+s.size.Dy())
}

// collide reports whether two sprites' bounding boxes overlap. BUG FIX: the
// original always used s1's size for both boxes, so a small sprite (e.g. the
// beam) colliding with a large one (e.g. an alien) used the wrong box for
// the second sprite. Each sprite now uses its own size.
func collide(s1, s2 Sprite) bool {
	a, b := s1.rect(), s2.rect()
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X &&
		a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y
}
