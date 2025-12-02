package behavior

import (
	"math"
	"math/rand"
)

// Boid represents a single entity in the flock.
// Boids is an artificial life program, developed by Craig Reynolds in 1986,
// which simulates the flocking behaviour of birds, and related group motion.
// His paper on this topic was published in 1987 in the proceedings of the ACM SIGGRAPH
// conference. The name "boid" corresponds to a shortened version of "bird-oid object",
// which refers to a bird-like object. https://en.wikipedia.org/wiki/Boids
// We export fields (X, Y, VX, VY) so the renderer can read them.
type Boid struct {
	X, Y   float64
	Vx, Vy float64
}

// Settings controls the physics constants for the simulation.
// Passing this into Update allows you to change rules dynamically at runtime.
type Settings struct {
	VisualRange    float64 // How far can they see?
	ProtectedRange float64 // Personal space radius

	CenteringFactor float64 // Cohesion strength
	AvoidFactor     float64 // Separation strength
	MatchingFactor  float64 // Alignment strength
	TurnFactor      float64 // Edge turning strength

	MaxSpeed float64
	MinSpeed float64

	ScreenWidth  float64
	ScreenHeight float64
}

// New creates a boid with random position and velocity.
func New(width, height float64) *Boid {
	return &Boid{
		X:  rand.Float64() * width,
		Y:  rand.Float64() * height,
		Vx: (rand.Float64() * 2) - 1,
		Vy: (rand.Float64() * 2) - 1,
	}
}

// Update calculates the next position based on neighbors and settings.
func (b *Boid) Update(flock []*Boid, s Settings) {
	// Initialize force accumulators
	closeDx, closeDy := 0.0, 0.0
	xVelAvg, yVelAvg := 0.0, 0.0
	xPosAvg, yPosAvg := 0.0, 0.0
	neighbors := 0.0

	for _, other := range flock {
		if b == other {
			continue
		}

		dx := b.X - other.X
		dy := b.Y - other.Y
		distSq := dx*dx + dy*dy

		// 1. Separation
		if distSq < s.ProtectedRange*s.ProtectedRange {
			closeDx += dx
			closeDy += dy
		}

		// Check visual range for Cohesion/Alignment
		if distSq < s.VisualRange*s.VisualRange {
			xVelAvg += other.Vx
			yVelAvg += other.Vy
			xPosAvg += other.X
			yPosAvg += other.Y
			neighbors++
		}
	}

	// Apply Separation
	b.Vx += closeDx * s.AvoidFactor
	b.Vy += closeDy * s.AvoidFactor

	// Apply Alignment and Cohesion
	if neighbors > 0 {
		xVelAvg /= neighbors
		yVelAvg /= neighbors
		b.Vx += (xVelAvg - b.Vx) * s.MatchingFactor
		b.Vy += (yVelAvg - b.Vy) * s.MatchingFactor

		xPosAvg /= neighbors
		yPosAvg /= neighbors
		b.Vx += (xPosAvg - b.X) * s.CenteringFactor
		b.Vy += (yPosAvg - b.Y) * s.CenteringFactor
	}

	// Screen Edges (Soft turn)
	margin := 100.0
	if b.X < margin {
		b.Vx += s.TurnFactor
	}
	if b.X > s.ScreenWidth-margin {
		b.Vx -= s.TurnFactor
	}
	if b.Y < margin {
		b.Vy += s.TurnFactor
	}
	if b.Y > s.ScreenHeight-margin {
		b.Vy -= s.TurnFactor
	}

	// Speed Limits
	speed := math.Sqrt(b.Vx*b.Vx + b.Vy*b.Vy)
	if speed > s.MaxSpeed {
		b.Vx = (b.Vx / speed) * s.MaxSpeed
		b.Vy = (b.Vy / speed) * s.MaxSpeed
	} else if speed < s.MinSpeed {
		b.Vx = (b.Vx / speed) * s.MinSpeed
		b.Vy = (b.Vy / speed) * s.MinSpeed
	}

	// Move
	b.X += b.Vx
	b.Y += b.Vy
}
