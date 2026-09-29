package game

import "image"

// shields are small destructible bunkers drawn as a bitmap of green pixel
// blocks, not sprite-sheet crops.
const shieldWidth = 22
const shieldHeight = 16

// Shield is a destructible bunker: a small pixel bitmap where true means an
// intact (undamaged) block.
type Shield struct {
	Position image.Point
	Pixels   [shieldHeight][shieldWidth]bool
}

// newShield creates a shield at (x,y) with a classic arch shape: rounded top
// corners and a notch cut out of the bottom middle for the cannon to fire
// through the gap.
func newShield(x, y int) *Shield {
	s := &Shield{Position: image.Pt(x, y)}
	for row := 0; row < shieldHeight; row++ {
		for col := 0; col < shieldWidth; col++ {
			s.Pixels[row][col] = true
		}
	}
	// round off the top corners
	for row := 0; row < 3; row++ {
		w := 3 - row
		for col := 0; col < w; col++ {
			s.Pixels[row][col] = false
			s.Pixels[row][shieldWidth-1-col] = false
		}
	}
	// carve the arch notch out of the bottom middle
	archWidth := 8
	archStart := (shieldWidth - archWidth) / 2
	for row := shieldHeight - 6; row < shieldHeight; row++ {
		for col := archStart; col < archStart+archWidth; col++ {
			s.Pixels[row][col] = false
		}
	}
	return s
}

// bounds returns the shield's bounding rectangle in world coordinates.
func (s *Shield) bounds() image.Rectangle {
	return image.Rect(s.Position.X, s.Position.Y, s.Position.X+shieldWidth, s.Position.Y+shieldHeight)
}

// alive reports whether the shield still has any intact pixels.
func (s *Shield) alive() bool {
	for row := range s.Pixels {
		for col := range s.Pixels[row] {
			if s.Pixels[row][col] {
				return true
			}
		}
	}
	return false
}

// hitAt reports whether the given world point overlaps an intact pixel.
func (s *Shield) hitAt(p image.Point) bool {
	if !p.In(s.bounds()) {
		return false
	}
	col, row := p.X-s.Position.X, p.Y-s.Position.Y
	return s.Pixels[row][col]
}

// damage clears pixels within radius of the given world point: a small
// crater, used for bomb and beam impacts.
func (s *Shield) damage(p image.Point, radius int) {
	cx, cy := p.X-s.Position.X, p.Y-s.Position.Y
	for row := 0; row < shieldHeight; row++ {
		for col := 0; col < shieldWidth; col++ {
			dx, dy := col-cx, row-cy
			if dx*dx+dy*dy <= radius*radius {
				s.Pixels[row][col] = false
			}
		}
	}
}

// erase clears every pixel that overlaps rect (in world coordinates), used
// to grind away shield material under aliens as they descend through it.
func (s *Shield) erase(rect image.Rectangle) {
	inter := rect.Intersect(s.bounds())
	if inter.Empty() {
		return
	}
	for y := inter.Min.Y; y < inter.Max.Y; y++ {
		for x := inter.Min.X; x < inter.Max.X; x++ {
			s.Pixels[y-s.Position.Y][x-s.Position.X] = false
		}
	}
}

// overlaps reports whether rect intersects the shield's bounding box.
func (s *Shield) overlaps(rect image.Rectangle) bool {
	return rect.Overlaps(s.bounds())
}
