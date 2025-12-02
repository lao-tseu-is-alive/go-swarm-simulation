package simulation

import (
	"testing"
)

func TestComputeBoidUpdate_Separation(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 1,0 (very close).
	// Should be pushed away (negative X).
	cfg := &Config{
		VisualRange:     10.0,
		ProtectedRange:  5.0,
		AvoidFactor:     0.1,
		CenteringFactor: 0.0,
		MatchingFactor:  0.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 1, PositionY: 0, VelocityX: 0, VelocityY: 0},
	}

	vx, _ := ComputeBoidUpdate(me, friends, cfg)

	if vx >= 0 {
		t.Errorf("Expected negative vx (separation), got %f", vx)
	}
}

func TestComputeBoidUpdate_Cohesion(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 5,0 (visible).
	// Should be pulled towards (positive X).
	cfg := &Config{
		VisualRange:     10.0,
		ProtectedRange:  2.0,
		AvoidFactor:     0.0,
		CenteringFactor: 0.1,
		MatchingFactor:  0.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 5, PositionY: 0, VelocityX: 0, VelocityY: 0},
	}

	vx, _ := ComputeBoidUpdate(me, friends, cfg)

	if vx <= 0 {
		t.Errorf("Expected positive vx (cohesion), got %f", vx)
	}
}

func TestComputeBoidUpdate_Alignment(t *testing.T) {
	// Setup: Me is moving 0,0. Friend is moving 1,0.
	// Should accelerate X.
	cfg := &Config{
		VisualRange:     10.0,
		ProtectedRange:  2.0,
		AvoidFactor:     0.0,
		CenteringFactor: 0.0,
		MatchingFactor:  0.1,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 5, PositionY: 0, VelocityX: 1, VelocityY: 0},
	}

	vx, _ := ComputeBoidUpdate(me, friends, cfg)

	if vx <= 0 {
		t.Errorf("Expected positive vx (alignment), got %f", vx)
	}
}
