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

	// Communication channel to the UI
	reportCh chan<- *ActorState
}

// Enforce interface compliance
var _ actor.Actor = (*Individual)(nil)

// NewIndividual creates the struct state (not the actor itself)
func NewIndividual(color string, startX, startY float64, reportCh chan<- *ActorState) *Individual {
	return &Individual{
		Color:    color,
		X:        startX,
		Y:        startY,
		vx:       (rand.Float64() - 0.5) * 2,
		vy:       (rand.Float64() - 0.5) * 2,
		reportCh: reportCh,
	}
}

// PreStart initializes the actor.
func (i *Individual) PreStart(ctx *actor.Context) error {
	// We can use this to setup resources, but for now, we just log.
	ctx.ActorSystem().Logger().Infof("Born: %s (%s) at %.2f, %.2f", ctx.ActorName(), i.Color, i.X, i.Y)
	return nil
}

// Receive is the brain. It handles messages one by one.
func (i *Individual) Receive(ctx *actor.ReceiveContext) {
	switch ctx.Message().(type) {
	case *goaktpb.PostStart:
		ctx.Logger().Infof("%s started", ctx.Self().Name())

	case *Tick:
		i.updatePosition()
		// PUSH: Send our new state to the UI immediately
		i.reportCh <- &ActorState{
			Id:        ctx.Self().Name(),
			Color:     i.Color,
			PositionX: i.X,
			PositionY: i.Y,
		}

	case *GetState:
		// Keep this for debugging if needed
		response := &ActorState{
			Id:        ctx.Self().Name(),
			Color:     i.Color,
			PositionX: i.X,
			PositionY: i.Y,
		}
		ctx.Response(response)
	default:
		// Always handle unknown messages gracefully
		ctx.Unhandled()
	}
}

// PostStop cleans up.
func (i *Individual) PostStop(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Infof("Died: %s", ctx.ActorName())
	return nil
}

// updatePosition implements the specific "personality" logic
func (i *Individual) updatePosition() {
	speedFactor := 1.0

	if i.Color == "🔴 RED" {
		i.vx += (rand.Float64() - 0.5) * 0.5
		i.vy += (rand.Float64() - 0.5) * 0.5
		speedFactor = 2.0
	} else {
		// no randomness in blue individuals just a slower clear predictable path...
		i.vx += 0.05
		i.vy += 0.05
		speedFactor = 1.5
	}

	i.X += i.vx * speedFactor
	i.Y += i.vy * speedFactor

	// Screen bounds (matches Ebiten window size)
	screenWidth, screenHeight := 640.0, 480.0

	if i.X < 0 {
		i.X = 0
		i.vx *= -1
	}
	if i.X > screenWidth {
		i.X = screenWidth
		i.vx *= -1
	}
	if i.Y < 0 {
		i.Y = 0
		i.vy *= -1
	}
	if i.Y > screenHeight {
		i.Y = screenHeight
		i.vy *= -1
	}
}
