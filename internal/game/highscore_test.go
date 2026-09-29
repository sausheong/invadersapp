package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHighScoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invaders", "highscore.json")

	if got := loadHighScoreFrom(path); got != 0 {
		t.Fatalf("expected 0 for a missing high score file, got %d", got)
	}

	saveHighScoreTo(path, 4200)
	if got := loadHighScoreFrom(path); got != 4200 {
		t.Fatalf("expected 4200 after saving, got %d", got)
	}

	saveHighScoreTo(path, 9999)
	if got := loadHighScoreFrom(path); got != 9999 {
		t.Fatalf("expected 9999 after overwriting, got %d", got)
	}
}

func TestLoadHighScoreFromGarbageFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "highscore.json")
	// write invalid JSON directly
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadHighScoreFrom(path); got != 0 {
		t.Errorf("expected 0 for a corrupt high score file, got %d", got)
	}
}
