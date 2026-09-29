//go:build jev

package main

import (
	"strings"
	"testing"
)

func TestStatsThreatEscapedAfterWindow(t *testing.T) {
	var s pilotStats
	s.add(&action{Situation: "threat", Move: moveLeft})
	for i := 0; i < outcomeWindow; i++ {
		s.tick()
	}
	if s.Threat != 1 || s.ThreatEscaped != 1 {
		t.Errorf("threat=%d escaped=%d, want 1/1", s.Threat, s.ThreatEscaped)
	}
	if s.Moves["threat"][moveLeft] != [2]int{1, 1} {
		t.Errorf("moves = %v", s.Moves)
	}
}

func TestStatsKilledWithinWindow(t *testing.T) {
	var s pilotStats
	s.add(&action{Situation: "threat", Move: moveStay})
	s.add(&action{Situation: "safe", Move: moveRight})
	s.tick()
	s.event(evCannonHit, false)
	if s.Threat != 1 || s.ThreatEscaped != 0 {
		t.Errorf("threat=%d escaped=%d, want 1/0", s.Threat, s.ThreatEscaped)
	}
	if s.Safe != 1 || s.SafeKilled != 1 {
		t.Errorf("safe=%d killed=%d, want 1/1", s.Safe, s.SafeKilled)
	}
	if len(s.pending) != 0 {
		t.Errorf("pending = %d, want 0", len(s.pending))
	}
}

func TestStatsShotOutcomes(t *testing.T) {
	var s pilotStats
	s.add(&action{Move: moveStay, FireProb: 0.9})
	s.event(evShotFired, true)
	s.event(evShotAlien, true)
	s.add(&action{Move: moveStay, FireProb: 0.9})
	s.event(evShotFired, true)
	s.event(evShotMissed, true)
	s.event(evShotFired, false) // player's own shot isn't counted
	s.event(evShotAlien, false)

	hit, total := s.hits()
	if s.Fired != 2 || hit != 1 || total != 2 {
		t.Errorf("fired=%d hit=%d total=%d, want 2/1/2", s.Fired, hit, total)
	}
	if !strings.Contains(s.hudText(), "HIT 1/2") {
		t.Errorf("hud = %q", s.hudText())
	}
}

func TestStatsCancelledShotNotCountedAsMiss(t *testing.T) {
	var s pilotStats
	s.add(&action{Move: moveStay, FireProb: 0.9})
	s.event(evShotFired, true)
	s.event(evShotCancelled, true)
	s.event(evCannonHit, true)
	if hit, total := s.hits(); hit != 0 || total != 0 {
		t.Errorf("hit=%d total=%d, want 0/0", hit, total)
	}
	if s.Shots["cancelled"] != 1 {
		t.Errorf("shots = %v", s.Shots)
	}
}
