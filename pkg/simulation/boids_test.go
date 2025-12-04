package simulation

import (
	"testing"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
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
	me := &Entity{
		ID:    "me",
		Color: TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*ActorState{
		{Position: &Vector{X: 1, Y: 0}, Velocity: &Vector{X: 0, Y: 0}},
	}

	force := ComputeBoidUpdate(me, friends, cfg)

	if force.X >= 0 {
		t.Errorf("Expected negative vx (separation), got %f", force.X)
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
	me := &Entity{
		ID:    "me",
		Color: TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*ActorState{
		{Position: &Vector{X: 5, Y: 0}, Velocity: &Vector{X: 0, Y: 0}},
	}

	force := ComputeBoidUpdate(me, friends, cfg)
	if force.X <= 0 {
		t.Errorf("Expected positive vx (cohesion), got %f", force.X)
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
	me := &Entity{
		ID:    "me",
		Color: TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*ActorState{
		{Position: &Vector{X: 5, Y: 0}, Velocity: &Vector{X: 1, Y: 0}},
	}

	force := ComputeBoidUpdate(me, friends, cfg)
	if force.X <= 0 {
		t.Errorf("Expected positive vx (alignment), got %f", force.X)
	}
}
