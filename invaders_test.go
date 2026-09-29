package main

import (
	"image"
	"testing"
)

// collide and createAlien are pure/hermetic: they don't touch the
// filesystem, network, or GUI, so they're tested directly here.

func TestCollideOverlapping(t *testing.T) {
	s1 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(0, 0)}
	s2 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(10, 10)}
	if !collide(s1, s2) {
		t.Fatal("expected overlapping sprites to collide")
	}
}

func TestCollideNonOverlapping(t *testing.T) {
	s1 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(0, 0)}
	s2 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(100, 100)}
	if collide(s1, s2) {
		t.Fatal("expected far-apart sprites not to collide")
	}
}

func TestCollideTouchingEdges(t *testing.T) {
	// sprites that exactly touch at the edge (Max == Min) should not be
	// considered colliding, since the comparisons in collide() are strict.
	s1 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(0, 0)}
	s2 := Sprite{size: image.Rect(0, 0, 20, 20), Position: image.Pt(20, 0)}
	if collide(s1, s2) {
		t.Fatal("expected edge-touching sprites not to collide")
	}
}

// TestCollideDifferingSizes exercises the bug fix: collide used to always
// use s1's size for BOTH sprites' bounding boxes, so a small sprite (like
// the beam) checked against a large one (like an alien) used the wrong box
// for the second sprite. Here s1 is tiny and s2 is large; s1 sits well
// inside s2's box but well outside a box of s1's own (tiny) size placed at
// s2's position, so the old buggy implementation would report no collision.
func TestCollideDifferingSizes(t *testing.T) {
	small := Sprite{size: image.Rect(0, 0, 2, 2), Position: image.Pt(15, 15)}
	large := Sprite{size: image.Rect(0, 0, 30, 30), Position: image.Pt(0, 0)}
	if !collide(small, large) {
		t.Fatal("expected small sprite inside large sprite's box to collide, using each sprite's own size")
	}
}

func TestCollideDifferingSizesNoOverlap(t *testing.T) {
	small := Sprite{size: image.Rect(0, 0, 2, 2), Position: image.Pt(100, 100)}
	large := Sprite{size: image.Rect(0, 0, 30, 30), Position: image.Pt(0, 0)}
	if collide(small, large) {
		t.Fatal("expected sprites with no overlap not to collide")
	}
}

func TestCreateAlien(t *testing.T) {
	sprite := alien1Sprite
	alt := alien1aSprite
	a := createAlien(42, 7, sprite, alt, 30, 3)

	if a.Position != image.Pt(42, 7) {
		t.Errorf("expected position (42,7), got %v", a.Position)
	}
	if a.Points != 30 {
		t.Errorf("expected 30 points, got %d", a.Points)
	}
	if a.Column != 3 {
		t.Errorf("expected column 3, got %d", a.Column)
	}
	if !a.Status {
		t.Error("expected newly created alien to be alive (Status true)")
	}
	if a.size != sprite {
		t.Errorf("expected size %v, got %v", sprite, a.size)
	}
	if a.Filter == nil || a.FilterA == nil || a.FilterE == nil {
		t.Error("expected Filter, FilterA and FilterE to all be set")
	}
}
