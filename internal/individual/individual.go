package individual

import (
	"math/rand"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

// Individual represents a single entity in our world.
type Individual struct {
	ID    string
	Color string // "RED" or "BLUE"
	X, Y  float64

	// Internal velocity
	vx, vy float64
}

// Enforce interface compliance
var _ actor.Actor = (*Individual)(nil)

// NewIndividual creates the struct state (not the actor itself).
func NewIndividual(color string, startX, startY float64) *Individual {
	return &Individual{
		Color: color,
		X:     startX,
		Y:     startY,
		vx:    (rand.Float64() - 0.5) * 2, // Random initial velocity
		vy:    (rand.Float64() - 0.5) * 2,
	}
}

// PreStart initializes the actor.
func (i *Individual) PreStart(ctx *actor.Context) error {
	// We can use this to setup resources, but for now, we just log.
	ctx.Logger().Infof("Born: %s (%s) at %.2f, %.2f", ctx.ActorName(), i.Color, i.X, i.Y)
	return nil
}

// Receive is the brain. It handles messages one by one.
func (i *Individual) Receive(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		// Lifecycle event when actor fully starts

	case *Tick:
		i.updatePosition()

	case *GetState:
		// Respond to the sender with our current state
		// Note: We use ctx.Sender() to reply.
		response := &ActorState{
			Id:        ctx.ActorName(),
			Color:     i.Color,
			PositionX: i.X,
			PositionY: i.Y,
		}
		// Send reply safely
		if ctx.Sender() != actor.NoSender {
			ctx.Response(response)
		}

	default:
		// Always handle unknown messages gracefully
		ctx.Unhandled()
	}
}

// PostStop cleans up.
func (i *Individual) PostStop(ctx *actor.Context) error {
	ctx.Logger().Infof("Died: %s", ctx.ActorName())
	return nil
}

// updatePosition implements the specific "personality" logic
func (i *Individual) updatePosition() {
	speedFactor := 1.0

	if i.Color == "RED" {
		// Aggressive: Random jitters, faster speed
		i.vx += (rand.Float64() - 0.5) * 0.5
		i.vy += (rand.Float64() - 0.5) * 0.5
		speedFactor = 2.0
	} else {
		// Blue: Consensual (more stable, resists change)
		i.vx *= 0.95 // Friction
		i.vy *= 0.95
		speedFactor = 0.5
	}

	// Apply velocity
	i.X += i.vx * speedFactor
	i.Y += i.vy * speedFactor

	// Simple World Bounds (Bounce off walls 0-800)
	if i.X < 0 || i.X > 800 {
		i.vx *= -1
		i.X += i.vx
	}
	if i.Y < 0 || i.Y > 600 {
		i.vy *= -1
		i.Y += i.vy
	}
}
