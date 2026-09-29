package game

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type highScoreData struct {
	HighScore int `json:"high_score"`
}

// highScorePath returns the path to the persisted high score file, falling
// back to the current directory if os.UserConfigDir isn't available.
func highScorePath() string {
	return filepath.Join(DataDir(), "highscore.json")
}

// DataDir is where the game keeps its files (the high score, and the
// autopilot's logs): an invaders folder in the user config directory,
// or the current directory if there isn't one.
func DataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return "invaders"
	}
	return filepath.Join(dir, "invaders")
}

// loadHighScore reads the persisted high score, tolerating any error by
// returning 0 (no high score yet).
func loadHighScore() int {
	return loadHighScoreFrom(highScorePath())
}

// loadHighScoreFrom is loadHighScore with an explicit path, so tests can
// point it at a t.TempDir() file instead of the real user config dir.
func loadHighScoreFrom(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var data highScoreData
	if err := json.Unmarshal(b, &data); err != nil {
		return 0
	}
	return data.HighScore
}

// saveHighScore persists score, tolerating any error (best-effort: a game
// still works fine without a writable config dir).
func saveHighScore(score int) {
	saveHighScoreTo(highScorePath(), score)
}

// saveHighScoreTo is saveHighScore with an explicit path, so tests can point
// it at a t.TempDir() file instead of the real user config dir.
func saveHighScoreTo(path string, score int) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(highScoreData{HighScore: score})
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}
