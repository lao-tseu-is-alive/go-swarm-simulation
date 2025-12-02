package simulation

// ComputeBoidUpdate calculates the new velocity based on boids rules
func ComputeBoidUpdate(me *Individual, friends []*ActorState, cfg *Config) (float64, float64) {
	vx, vy := me.vx, me.vy

	// Initialize force accumulators
	closeDx, closeDy := 0.0, 0.0
	xVelAvg, yVelAvg := 0.0, 0.0
	xPosAvg, yPosAvg := 0.0, 0.0
	neighbors := 0.0

	for _, other := range friends {
		dx := me.X - other.PositionX
		dy := me.Y - other.PositionY
		distSq := dx*dx + dy*dy

		// 1. Separation
		if distSq < cfg.ProtectedRange*cfg.ProtectedRange {
			closeDx += dx
			closeDy += dy
		}

		// Check visual range for Cohesion/Alignment
		if distSq < cfg.VisualRange*cfg.VisualRange {
			xVelAvg += other.VelocityX
			yVelAvg += other.VelocityY
			xPosAvg += other.PositionX
			yPosAvg += other.PositionY
			neighbors++
		}
	}

	// Apply Separation
	vx += closeDx * cfg.AvoidFactor
	vy += closeDy * cfg.AvoidFactor

	// Apply Alignment and Cohesion
	if neighbors > 0 {
		xVelAvg /= neighbors
		yVelAvg /= neighbors
		vx += (xVelAvg - vx) * cfg.MatchingFactor
		vy += (yVelAvg - vy) * cfg.MatchingFactor

		xPosAvg /= neighbors
		yPosAvg /= neighbors
		vx += (xPosAvg - me.X) * cfg.CenteringFactor
		vy += (yPosAvg - me.Y) * cfg.CenteringFactor
	}

	return vx, vy
}
