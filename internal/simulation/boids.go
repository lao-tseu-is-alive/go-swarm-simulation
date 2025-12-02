package simulation

import (
	"math"
	"math/rand"
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

		// --- SAFETY CHECK: Prevent Singularity ---
		// If actors are too close (overlapping), apply a random "emergency push"
		// to separate them. Do NOT divide by small numbers.
		if distSq < 1.0 {
			// Random push direction to break the symmetry/overlap
			sepX += (rand.Float64() - 0.5) * 50.0
			sepY += (rand.Float64() - 0.5) * 50.0
			continue
		}

		// Standard Inverse Square Law
		sepX += dx / distSq
		sepY += dy / distSq
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
	}
	// Reynolds Separation is usually: Steering = Desired(Away) - Velocity
	// But often just adding the raw "Away" vector works better for stabilization.
	// Let's stick to Steering:
	sepX -= me.vx
	sepY -= me.vy

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
