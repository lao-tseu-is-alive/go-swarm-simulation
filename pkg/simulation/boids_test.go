package simulation

import (
	"testing"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

func TestComputeBoidUpdate_Separation(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 1,0 (very close).
	// Should be pushed away (negative X).
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos:   geometry.Vector2D{X: 0, Y: 0},
		Vel:   geometry.Vector2D{X: 0, Y: 0},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 1, Y: 0}, Velocity: &pb.Vector{X: 0, Y: 0}},
	}

	// Config values: flockVision, personalSpace, cohesion, separation, alignment
	force := ComputeBoidUpdate(me, friends, 10.0, 5.0, 0.0, 0.1, 0.0)

	if force.X >= 0 {
		t.Errorf("Expected negative vx (separation), got %f", force.X)
	}
}

func TestComputeBoidUpdate_Cohesion(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 5,0 (visible, but outside personal space).
	// Should be pulled towards (positive X).
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos:   geometry.Vector2D{X: 0, Y: 0},
		Vel:   geometry.Vector2D{X: 0, Y: 0},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 5, Y: 0}, Velocity: &pb.Vector{X: 0, Y: 0}},
	}

	// Config values: flockVision, personalSpace, cohesion, separation, alignment
	force := ComputeBoidUpdate(me, friends, 10.0, 2.0, 0.1, 0.0, 0.0)

	if force.X <= 0 {
		t.Errorf("Expected positive vx (cohesion), got %f", force.X)
	}
}

func TestComputeBoidUpdate_Alignment(t *testing.T) {
	// Setup: Me is stationary at 0,0. Friend at 5,0 is moving right at (1,0).
	// Should accelerate X to match friend's velocity.
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos:   geometry.Vector2D{X: 0, Y: 0},
		Vel:   geometry.Vector2D{X: 0, Y: 0},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 5, Y: 0}, Velocity: &pb.Vector{X: 1, Y: 0}},
	}

	// Config values: flockVision, personalSpace, cohesion, separation, alignment
	force := ComputeBoidUpdate(me, friends, 10.0, 2.0, 0.0, 0.0, 0.1)

	if force.X <= 0 {
		t.Errorf("Expected positive vx (alignment), got %f", force.X)
	}
}
