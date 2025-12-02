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
	visibleFriends []*ActorState // Friends (Blue)
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
	// Determine initial behavior based on color
	if i.Color == ColorRed {
		ctx.Become(i.RedBehavior)
		i.RedBehavior(ctx) // process current message
	} else {
		ctx.Become(i.BlueBehavior)
		i.BlueBehavior(ctx)
	}

}

func (i *Individual) PostStop(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Infof("Death : %s", ctx.ActorName())
	return nil
}

// RedBehavior Behavior for RED (Aggressive)
func (i *Individual) RedBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktpb.PostStart:
		ctx.Logger().Infof("%s started", ctx.Self().Name())
		// Initialize ID here
		i.ID = ctx.Self().Name()

	case *Tick:
		// Aggressive Chase behavior
		if len(i.visibleTargets) > 0 {
			i.chaseClosestTarget()
		} else {
			// Random jitter if no target, wander
			i.vx += (rand.Float64() - 0.5) * 0.1
			i.vy += (rand.Float64() - 0.5) * 0.2
		}
		i.updatePosition()
		i.reportState(ctx)

	case *Perception:
		i.visibleTargets = msg.Targets
	// Reds don't care about friends in this version, or maybe they do later for packing?

	// Handle Conversion
	case *Convert:
		if msg.TargetColor == ColorBlue {
			ctx.Logger().Infof("%s converting from %s to %s", ctx.Self().Name(), i.Color, msg.TargetColor)
			i.Color = ColorBlue
			ctx.Become(i.BlueBehavior) // <--- The Magic thank's to Actor behaviors
			i.visibleTargets = nil     // Clear memory of old enemies
			i.visibleFriends = nil     // Reset friends
			// Reaction jump: Push them slightly away to visualize the impact
			i.vx *= -1.5
			i.vy *= -1.5
		}
	case *GetState:
		i.respondState(ctx)

	default:
		ctx.Unhandled()
	}

}

// BlueBehavior  Behavior for BLUE (Flocking)
func (i *Individual) BlueBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktpb.PostStart:
		ctx.Logger().Infof("%s started", ctx.Self().Name())
		// Initialize ID here
		i.ID = ctx.Self().Name()
	case *Tick:
		// Blue: Consensual/Swarm behavior (Cohesion could be added here)
		// For now, they stabilize and drift
		i.vx += 0.05
		i.vy += 0.05
		// === FLOCKING LOGIC ===
		// 1. Calculate Acceleration based on neighbors
		ax := 0.0
		ay := 0.0
		ax, ay = ComputeFlockingForce(i, i.visibleFriends, i.cfg)

		// 2. Add randomness if alone to prevent freezing
		if len(i.visibleFriends) == 0 {
			ax += (rand.Float64() - 0.5) * 0.5
			ay += (rand.Float64() - 0.5) * 0.5
		}

		// 3. Apply Acceleration
		i.vx += ax
		i.vy += ay
		i.updatePosition()
		//i.applyFlocking() // No "if blue" needed!
		i.reportState(ctx)

	case *Perception:
		i.visibleTargets = msg.Targets // Enemies to flee from? (Future feature)
		i.visibleFriends = msg.Friends // Friends to flock wit

	case *Convert:
		if msg.TargetColor == ColorRed {
			ctx.Logger().Infof("%s converting from %s to %s", ctx.Self().Name(), i.Color, msg.TargetColor)
			i.Color = ColorRed
			ctx.Become(i.RedBehavior) // <--- The Magic thank's to Actor behaviors
			i.visibleTargets = nil    // Clear memory of old enemies
			i.visibleFriends = nil
			// Reaction jump: Push them slightly away to visualize the impact
			i.vx *= -1.5
			i.vy *= -1.5
		}
	case *GetState:
		i.respondState(ctx)
	}
}

func (i *Individual) reportState(ctx *actor.ReceiveContext) {
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
}
func (i *Individual) respondState(ctx *actor.ReceiveContext) {
	ctx.Response(i.makeState())
}

func (i *Individual) makeState() *ActorState {
	return &ActorState{
		Id:        i.ID,
		Color:     i.Color,
		PositionX: i.X,
		PositionY: i.Y,
		VelocityX: i.vx, // Send Velocity!
		VelocityY: i.vy,
	}
}
func (i *Individual) updatePosition() {
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
	// CRITICAL: i.vx and i.vy Must not be 0
	if i.vx == 0 {
		i.vx = 0.01
	}
	if i.vy == 0 {
		i.vy = 0.01
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

		aggression := i.cfg.Aggression // Increased aggression for better catching
		i.vx += dx * aggression
		i.vy += dy * aggression

		// Cap max speed
		maxSpeed := i.cfg.MaxSpeed
		speed := math.Sqrt(i.vx*i.vx + i.vy*i.vy)
		if speed > maxSpeed {
			i.vx = (i.vx / speed) * maxSpeed
			i.vy = (i.vy / speed) * maxSpeed
		}
	}
}
