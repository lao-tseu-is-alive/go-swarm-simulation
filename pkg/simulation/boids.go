package simulation

import (
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

// ComputeBoidUpdate calculates the new velocity based on boids rules.
// Config values are passed as parameters to avoid shared state.
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
