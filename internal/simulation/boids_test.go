package simulation

import (
	"math"
	"testing"
)

func TestComputeFlockingForce_Separation(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 1,0 (very close).
	// Should be pushed away (negative X).
	cfg := &Config{
		PerceptionRadius: 10.0,
		MaxSpeed:         1.0,
		CohesionWeight:   0.0,
		AlignmentWeight:  0.0,
		SeparationWeight: 1.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 1, PositionY: 0, VelocityX: 0, VelocityY: 0},
	}

	ax, ay := ComputeFlockingForce(me, friends, cfg)

	if ax >= 0 {
		t.Errorf("Expected negative ax (separation), got %f", ax)
	}
	if ay != 0 {
		t.Errorf("Expected 0 ay, got %f", ay)
	}
}

func TestComputeFlockingForce_Cohesion(t *testing.T) {
	// Setup: Me is at 0,0. Friend is at 10,0 (far but visible).
	// Should be pulled towards (positive X).
	cfg := &Config{
		PerceptionRadius: 20.0,
		MaxSpeed:         1.0,
		CohesionWeight:   1.0,
		AlignmentWeight:  0.0,
		SeparationWeight: 0.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 10, PositionY: 0, VelocityX: 0, VelocityY: 0},
	}

	ax, ay := ComputeFlockingForce(me, friends, cfg)

	if ax <= 0 {
		t.Errorf("Expected positive ax (cohesion), got %f", ax)
	}
	_ = ay
}

func TestComputeFlockingForce_Alignment(t *testing.T) {
	// Setup: Me is moving 0,0. Friend is moving 1,0.
	// Should accelerate X.
	cfg := &Config{
		PerceptionRadius: 20.0,
		MaxSpeed:         1.0,
		CohesionWeight:   0.0,
		AlignmentWeight:  1.0,
		SeparationWeight: 0.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 5, PositionY: 0, VelocityX: 1, VelocityY: 0},
	}

	ax, ay := ComputeFlockingForce(me, friends, cfg)

	if ax <= 0 {
		t.Errorf("Expected positive ax (alignment), got %f", ax)
	}
	_ = ay
}

func TestComputeFlockingForce_Clustering(t *testing.T) {
	// Simulation of the "Blue Behavior" bug.
	// 4 friends in a tight square around me.
	// Me at 0,0. Friends at 1,1, 1,-1, -1,1, -1,-1.
	// They are all very close. Separation should be HUGE.
	// If they just sit there, force is 0 or weak.
	cfg := &Config{
		PerceptionRadius: 10.0,
		MaxSpeed:         1.0,
		CohesionWeight:   0.1,
		AlignmentWeight:  0.1,
		SeparationWeight: 1.0, // High separation
	}
	me := &Individual{X: 0, Y: 0, vx: 0, vy: 0}
	friends := []*ActorState{
		{PositionX: 0.1, PositionY: 0.1},
		{PositionX: 0.1, PositionY: -0.1},
		{PositionX: -0.1, PositionY: 0.1},
		{PositionX: -0.1, PositionY: -0.1},
	}

	ax, ay := ComputeFlockingForce(me, friends, cfg)

	// In a perfect symmetry, forces might cancel out to 0.
	// But if we move slightly, it should explode.
	// Let's test a single close neighbor to ensure strong repulsion.

	// Reset to single neighbor very close
	friends = []*ActorState{
		{PositionX: 0.01, PositionY: 0},
	}
	ax, ay = ComputeFlockingForce(me, friends, cfg)

	// Force should be very strong, close to max speed or higher before normalization?
	// The function normalizes forces.

	if ax >= 0 {
		t.Errorf("Expected strong negative repulsion, got %f", ax)
	}

	// Check magnitude
	mag := math.Sqrt(ax*ax + ay*ay)
	if mag < 0.1 {
		t.Errorf("Expected strong repulsion magnitude, got %f", mag)
	}
}

func TestComputeFlockingForce_SeparationBrakingBug(t *testing.T) {
	// Setup: Me moving at 1,0.
	// Friend far away (dist > PerceptionRadius).
	// SeparationWeight = 1.0.
	// Should NOT brake.
	cfg := &Config{
		PerceptionRadius: 10.0,
		MaxSpeed:         1.0,
		CohesionWeight:   0.0,
		AlignmentWeight:  0.0,
		SeparationWeight: 1.0,
	}
	me := &Individual{X: 0, Y: 0, vx: 1, vy: 0}
	friends := []*ActorState{
		{PositionX: 100, PositionY: 0}, // Far away
	}

	ax, ay := ComputeFlockingForce(me, friends, cfg)

	// If bug exists, ax will be -1 (braking).
	// If fixed, ax should be 0.
	if ax != 0 {
		t.Errorf("Expected 0 ax (no separation needed), got %f. This indicates the braking bug.", ax)
	}
	_ = ay
}
