package simulation

import (
	"math"
	"math/rand"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

// Enum Constants for Color
const (
	ColorRed  = "🔴 RED"
	ColorBlue = "🔵 BLUE"
)

type Individual struct {
	ID             string
	Color          string
	X, Y           float64
	vx, vy         float64
	visibleTargets []*ActorState
	cfg            *Config
}

var _ actor.Actor = (*Individual)(nil)

func NewIndividual(color string, startX, startY float64, cfg *Config) *Individual {
	return &Individual{
		Color: color,
		X:     startX,
		Y:     startY,
		// Initialize with random velocity
		vx:  (rand.Float64() - 0.5) * 2,
		vy:  (rand.Float64() - 0.5) * 2,
		cfg: cfg,
	}
}

func (i *Individual) PreStart(ctx *actor.Context) error {
	// ctx.ActorName() might be deprecated in v3 favor of ctx.Name()
	// but let's assume ctx.ActorName() or ctx.Name() works.
	// If Error: use ctx.Self().Name() in Receive
	ctx.ActorSystem().Logger().Infof("Born: %s (%s) at %.2f, %.2f", ctx.ActorName(), i.Color, i.X, i.Y)
	return nil
}

func (i *Individual) Receive(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktpb.PostStart:
		ctx.Logger().Infof("%s started", ctx.Self().Name())
		// Initialize ID here
		i.ID = ctx.Self().Name()

	case *Perception:
		i.visibleTargets = msg.Targets

	// NEW: Handle Conversion
	case *Convert:
		if i.Color != msg.TargetColor {
			ctx.Logger().Infof("%s converting from %s to %s", ctx.Self().Name(), i.Color, msg.TargetColor)
			i.Color = msg.TargetColor
			i.visibleTargets = nil // Clear memory of old enemies

			// Reaction jump: Push them slightly away to visualize the impact
			i.vx *= -1.5
			i.vy *= -1.5
		}

	case *Tick:
		i.updatePosition()
		// Prepare state to send back to World
		state := &ActorState{
			Id:        ctx.Self().Name(),
			Color:     i.Color,
			PositionX: i.X,
			PositionY: i.Y,
		}

		// REPORT TO WORLD
		// Since 'Tick' came from the World, ctx.Sender() IS the World.
		if ctx.Sender() != nil && ctx.Sender() != ctx.ActorSystem().NoSender() {
			ctx.Tell(ctx.Sender(), state)
		}
	case *GetState:
		response := &ActorState{
			Id:        ctx.Self().Name(),
			Color:     i.Color,
			PositionX: i.X,
			PositionY: i.Y,
		}
		ctx.Response(response)

	default:
		ctx.Unhandled()
	}
}

func (i *Individual) PostStop(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Infof("Death : %s", ctx.ActorName())
	return nil
}

func (i *Individual) updatePosition() {
	// 1. Behavior Logic
	if i.Color == ColorRed {
		// Aggressive Chase behavior
		if len(i.visibleTargets) > 0 {
			i.chaseClosestTarget()
		} else {
			// Random jitter if no target
			i.vx += (rand.Float64() - 0.5) * 0.2
			i.vy += (rand.Float64() - 0.5) * 0.2
		}
	} else {
		// Blue: Consensual/Swarm behavior (Cohesion could be added here)
		// For now, they stabilize and drift
		i.vx += 0.05
		i.vy += 0.05
	}

	// 2. Physics Integration
	i.X += i.vx
	i.Y += i.vy

	// 3. World Bounds
	if i.X < 0 {
		i.X = 0
		i.vx *= -1
	}
	if i.X > i.cfg.WorldWidth {
		i.X = i.cfg.WorldWidth
		i.vx *= -1
	}
	if i.Y < 0 {
		i.Y = 0
		i.vy *= -1
	}
	if i.Y > i.cfg.WorldHeight {
		i.Y = i.cfg.WorldHeight
		i.vy *= -1
	}
}

func (i *Individual) chaseClosestTarget() {
	var closest *ActorState
	minDistSq := math.MaxFloat64

	for _, target := range i.visibleTargets {
		dx := target.PositionX - i.X
		dy := target.PositionY - i.Y
		distSq := dx*dx + dy*dy

		if distSq < minDistSq {
			minDistSq = distSq
			closest = target
		}
	}

	if closest != nil {
		dx := closest.PositionX - i.X
		dy := closest.PositionY - i.Y
		length := math.Sqrt(dx*dx + dy*dy)
		if length > 0 {
			dx /= length
			dy /= length
		}

		agression := i.cfg.Agression // Increased aggression for better catching
		i.vx += dx * agression
		i.vy += dy * agression

		// Cap max speed
		maxSpeed := i.cfg.MaxSpeed
		speed := math.Sqrt(i.vx*i.vx + i.vy*i.vy)
		if speed > maxSpeed {
			i.vx = (i.vx / speed) * maxSpeed
			i.vy = (i.vy / speed) * maxSpeed
		}
	}
}
