package main

import (
	"image"
	"testing"
)

// collide and createAlien are pure/hermetic enough to unit test directly:
// they don't touch the filesystem, network, or GUI. Asset loading now lives
// in loadAssets(), called explicitly from main(), so importing/testing this
// package doesn't try to read image or sound files relative to the test
// binary's location.

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

func TestCreateAlien(t *testing.T) {
	sprite := alien1Sprite
	alt := alien1aSprite
	a := createAlien(42, 7, sprite, alt, 30)

	if a.Position != image.Pt(42, 7) {
		t.Errorf("expected position (42,7), got %v", a.Position)
	}
	if a.Points != 30 {
		t.Errorf("expected 30 points, got %d", a.Points)
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
