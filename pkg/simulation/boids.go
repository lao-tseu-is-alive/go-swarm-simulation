package simulation

import "github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"

// ComputeBoidUpdate calculates the new velocity based on boids rules
func ComputeBoidUpdate(me *Entity, friends []*ActorState, cfg *Config) geometry.Vector2D {
	force := geometry.Vector2D{}

	// Initialize force accumulators
	// ... accumulators (can use Vectors now!) ...
	avgVel := geometry.Vector2D{}
	avgPos := geometry.Vector2D{}
	separation := geometry.Vector2D{}
	neighbors := 0.0

	for _, a := range friends {
		other := Entity{
			ID:    a.Id,
			Color: a.Color,
			Pos:   GeomVector2DFromProto(a.Position),
			Vel:   GeomVector2DFromProto(a.Velocity),
		}
		distSq := me.Pos.DistanceSquaredTo(other.Pos)
		// 1. Separation
		if distSq < cfg.ProtectedRange*cfg.ProtectedRange {
			// Push away: (me - other)
			diff := me.Pos.Sub(other.Pos)
			separation = separation.Add(diff)
		}

		// Check visual range for Cohesion/Alignment
		if distSq < cfg.VisualRange*cfg.VisualRange {
			avgVel = avgVel.Add(other.Vel)
			avgPos = avgPos.Add(other.Pos)
			neighbors++
		}
	}

	// Apply Separation weights
	force = force.Add(separation.Mul(cfg.AvoidFactor))

	// Apply Alignment and Cohesion
	if neighbors > 0 {
		avgVel, _ = avgVel.Div(neighbors) // Error handling ignored for brevity (neighbors > 0)
		// Alignment: (AvgVel - MyVel) * Factor
		align := avgVel.Sub(me.Vel).Mul(cfg.MatchingFactor)
		force = force.Add(align)

		avgPos, _ = avgPos.Div(neighbors)
		// Cohesion: (AvgPos - MyPos) * Factor
		cohesion := avgPos.Sub(me.Pos).Mul(cfg.CenteringFactor)
		force = force.Add(cohesion)
	}

	return force
}
