package game

import (
	"image"
	"testing"
)

func TestNewShieldIsAlive(t *testing.T) {
	s := newShield(10, 10)
	if !s.alive() {
		t.Fatal("expected a freshly built shield to be alive")
	}
}

func TestShieldDamageClearsPixels(t *testing.T) {
	s := newShield(0, 0)
	center := image.Pt(s.Position.X+shieldWidth/2, s.Position.Y+shieldHeight/2)
	if !s.hitAt(center) {
		t.Fatal("expected the shield's center pixel to start intact")
	}
	s.damage(center, 3)
	if s.hitAt(center) {
		t.Error("expected the center pixel to be cleared after damage")
	}
}

func TestShieldDiesWhenFullyDamaged(t *testing.T) {
	s := newShield(0, 0)
	center := image.Pt(s.Position.X+shieldWidth/2, s.Position.Y+shieldHeight/2)
	s.damage(center, shieldWidth+shieldHeight) // radius big enough to clear everything
	if s.alive() {
		t.Error("expected the shield to be dead once every pixel is cleared")
	}
}

func TestShieldEraseUnderOverlap(t *testing.T) {
	s := newShield(0, 0)
	// an alien-sized box overlapping the shield's top-left corner
	rect := image.Rect(-5, -5, 5, 5)
	if !s.overlaps(rect) {
		t.Fatal("expected the rect to overlap the shield")
	}
	s.erase(rect)
	if s.hitAt(image.Pt(2, 2)) {
		t.Error("expected pixels under the overlap to be erased")
	}
}

func TestShieldHitAtOutsideBounds(t *testing.T) {
	s := newShield(50, 50)
	if s.hitAt(image.Pt(0, 0)) {
		t.Error("expected a point outside the shield's bounds never to hit")
	}
}
