package simulation

import (
	"math"
)

// ComputeFlockingForce calculates the acceleration vector safely
func ComputeFlockingForce(me *Individual, friends []*ActorState, cfg *Config) (float64, float64) {
	if len(friends) == 0 {
		return 0, 0
	}

	var (
		centerX, centerY float64
		avgVelX, avgVelY float64
		sepX, sepY       float64
		count            = float64(len(friends))
	)

	for _, n := range friends {
		// Cohesion & Alignment Accumulators
		centerX += n.PositionX
		centerY += n.PositionY
		avgVelX += n.VelocityX
		avgVelY += n.VelocityY

		// Separation Logic
		dx := me.X - n.PositionX
		dy := me.Y - n.PositionY
		distSq := dx*dx + dy*dy

		// Inside the loop in ComputeFlockingForce
		dist := math.Sqrt(distSq)

		// Calculate how much we are overlapping (0.0 to 1.0)
		// The closer we are, the stronger the push.
		// Using PerceptionRadius as the threshold, or a specific SeparationRadius (better).
		if dist < cfg.PerceptionRadius {
			// Weight the push by how close they are
			strength := (cfg.PerceptionRadius - dist) / cfg.PerceptionRadius

			// Normalize direction (dx/dist) and scale by strength.
			// dx already points AWAY (Me - Neighbor), so we ADD it to separate.
			sepX += (dx / dist) * strength
			sepY += (dy / dist) * strength
		}

	}

	// 1. Cohesion
	centerX /= count
	centerY /= count
	cohX, cohY := steerTowards(me, centerX, centerY, cfg.MaxSpeed)

	// 2. Alignment
	avgVelX /= count
	avgVelY /= count
	alignX, alignY := 0.0, 0.0
	// Normalize average velocity
	avgSpeed := math.Sqrt(avgVelX*avgVelX + avgVelY*avgVelY)
	if avgSpeed > 0 {
		avgVelX = (avgVelX / avgSpeed) * cfg.MaxSpeed
		avgVelY = (avgVelY / avgSpeed) * cfg.MaxSpeed
		alignX = avgVelX - me.vx
		alignY = avgVelY - me.vy
	}

	// 3. Separation
	// Normalize the accumulated separation vector
	sepSpeed := math.Sqrt(sepX*sepX + sepY*sepY)
	if sepSpeed > 0 {
		sepX = (sepX / sepSpeed) * cfg.MaxSpeed
		sepY = (sepY / sepSpeed) * cfg.MaxSpeed

		// Reynolds Separation is usually: Steering = Desired(Away) - Velocity
		sepX -= me.vx
		sepY -= me.vy
	}

	// Apply Weights
	totalX := (cohX * cfg.CohesionWeight) + (alignX * cfg.AlignmentWeight) + (sepX * cfg.SeparationWeight)
	totalY := (cohY * cfg.CohesionWeight) + (alignY * cfg.AlignmentWeight) + (sepY * cfg.SeparationWeight)

	// --- FINAL SAFETY: NaN Check ---
	// If the result is corrupted, return 0 to prevent the actor from disappearing
	if math.IsNaN(totalX) || math.IsInf(totalX, 0) {
		return 0, 0
	}
	if math.IsNaN(totalY) || math.IsInf(totalY, 0) {
		return 0, 0
	}

	return totalX, totalY
}

func steerTowards(me *Individual, targetX, targetY, maxSpeed float64) (float64, float64) {
	desiredX := targetX - me.X
	desiredY := targetY - me.Y
	dist := math.Sqrt(desiredX*desiredX + desiredY*desiredY)

	if dist > 0 {
		desiredX = (desiredX / dist) * maxSpeed
		desiredY = (desiredY / dist) * maxSpeed
	} else {
		return 0, 0
	}
	return desiredX - me.vx, desiredY - me.vy
}
