package simulation

import (
	"testing"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

func TestComputeBoidUpdate_Separation(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 1,0 (very close).
	// Should be pushed away (negative X).
	cfg := &Config{
		BlueFlockVision:   10.0,
		BluePersonalSpace: 5.0,
		BlueSeparation:    0.1,
		BlueCohesion:      0.0,
		BlueAlignment:     0.0,
	}
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 1, Y: 0}, Velocity: &pb.Vector{X: 0, Y: 0}},
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
		BlueFlockVision:   10.0,
		BluePersonalSpace: 2.0,
		BlueSeparation:    0.0,
		BlueCohesion:      0.1,
		BlueAlignment:     0.0,
	}
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 5, Y: 0}, Velocity: &pb.Vector{X: 0, Y: 0}},
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
		BlueFlockVision:   10.0,
		BluePersonalSpace: 2.0,
		BlueSeparation:    0.0,
		BlueCohesion:      0.0,
		BlueAlignment:     0.1,
	}
	me := &Entity{
		ID:    "me",
		Color: pb.TeamColor_TEAM_BLUE,
		Pos: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
		Vel: geometry.Vector2D{
			X: 0,
			Y: 0,
		},
	}
	friends := []*pb.ActorState{
		{Position: &pb.Vector{X: 5, Y: 0}, Velocity: &pb.Vector{X: 1, Y: 0}},
	}

	force := ComputeBoidUpdate(me, friends, cfg)
	if force.X <= 0 {
		t.Errorf("Expected positive vx (alignment), got %f", force.X)
	}
}
