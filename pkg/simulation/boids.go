package simulation

import (
	"math"
	"math/rand/v2"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

// ComputeBoidUpdate calculates the steering force for a Blue actor based on boids rules.
//
// The classic boids algorithm implements three behaviors:
//   - Separation: steer away from nearby neighbors (within personalSpace)
//   - Cohesion: steer towards the average position of neighbors (within flockVision)
//   - Alignment: steer to match the average velocity of neighbors (within flockVision)
//
// Parameters:
//   - me: the entity to compute forces for
//   - friends: slice of visible friendly actors
//   - flockVision: radius for cohesion and alignment calculations
//   - personalSpace: radius for separation calculations
//   - cohesion: strength multiplier for cohesion force
//   - separation: strength multiplier for separation force
//   - alignment: strength multiplier for alignment force
//
// Returns a velocity delta (force) to be added to the entity's current velocity.
func ComputeBoidUpdate(
	me *Entity,
	friends []*pb.ActorState,
	flockVision float64,
	personalSpace float64,
	cohesion float64,
	separation float64,
	alignment float64,
) geometry.Vector2D {
	force := geometry.Vector2D{}

	// Initialize force accumulators
	avgVel := geometry.Vector2D{}
	avgPos := geometry.Vector2D{}
	separationForce := geometry.Vector2D{}
	neighbors := 0.0

	flockVisionSq := flockVision * flockVision
	personalSpaceSq := personalSpace * personalSpace

	for _, a := range friends {
		other := Entity{
			ID:    a.Id,
			Color: a.Color,
			Pos:   GeomVector2DFromProto(a.Position),
			Vel:   GeomVector2DFromProto(a.Velocity),
		}
		distSq := me.Pos.DistanceSquaredTo(other.Pos)

		// 1. Separation - push away if too close
		if distSq < personalSpaceSq {
			diff := me.Pos.Sub(other.Pos)
			if distSq < 0.01 {
				// If entities are at exact same position, Normalize returns (0,0).
				// Give a random push direction to unstick them.
				angle := rand.Float64() * 2 * math.Pi
				diff = geometry.Vector2D{X: math.Cos(angle), Y: math.Sin(angle)}
			}

			diff = diff.Normalize()
			separationForce = separationForce.Add(diff)
		}

		// 2. Check visual range for Cohesion/Alignment
		if distSq < flockVisionSq {
			avgVel = avgVel.Add(other.Vel)
			avgPos = avgPos.Add(other.Pos)
			neighbors++
		}
	}

	// Apply Separation force
	force = force.Add(separationForce.Mul(separation))

	// Apply Alignment and Cohesion only if we have neighbors
	if neighbors > 0 {
		avgVel, _ = avgVel.Div(neighbors)
		// Alignment: steer towards average velocity
		align := avgVel.Sub(me.Vel).Mul(alignment)
		force = force.Add(align)

		avgPos, _ = avgPos.Div(neighbors)
		// Cohesion: steer towards average position
		cohesionForce := avgPos.Sub(me.Pos).Mul(cohesion)
		force = force.Add(cohesionForce)
	}

	return force
}
