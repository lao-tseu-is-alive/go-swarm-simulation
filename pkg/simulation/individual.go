package simulation

import (
	"math"
	"math/rand"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

const (
	ColorRed  = "🔴 RED"
	ColorBlue = "🔵 BLUE"
)

type Individual struct {
	ID             string
	Color          string
	X, Y           float64
	vx, vy         float64
	visibleTargets []*ActorState // Enemies
	visibleFriends []*ActorState // Allies
	cfg            *Config
}

var _ actor.Actor = (*Individual)(nil)

func NewIndividual(color string, startX, startY, vx, vy float64, cfg *Config) *Individual {
	return &Individual{
		Color: color,
		X:     startX,
		Y:     startY,
		vx:    vx + (rand.Float64()-0.5)*2,
		vy:    vy + (rand.Float64()-0.5)*2,
		cfg:   cfg,
	}
}

// ============================================================================
// Actor Lifecycle Hooks
// ============================================================================

func (i *Individual) PreStart(ctx *actor.Context) error {
	i.ID = ctx.ActorName()
	i.Log(ctx.ActorSystem(), "Born: %s (%s) at (%.1f, %.1f)",
		i.ID, i.Color, i.X, i.Y)
	return nil
}

func (i *Individual) PostStop(ctx *actor.Context) error {
	i.Log(ctx.ActorSystem(), "Death: %s", ctx.ActorName())
	return nil
}

// ============================================================================
// Message Routing (Entry Point)
// ============================================================================

func (i *Individual) Receive(ctx *actor.ReceiveContext) {
	// Route to appropriate behavior based on current color
	if i.Color == ColorRed {
		ctx.Become(i.RedBehavior)
		i.RedBehavior(ctx)
	} else {
		ctx.Become(i.BlueBehavior)
		i.BlueBehavior(ctx)
	}
}

// ============================================================================
// RED BEHAVIOR: Aggressive Hunter
// ============================================================================

func (i *Individual) RedBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		i.ID = ctx.Self().Name()
		i.Log(ctx.ActorSystem(), "%s started in RED mode", i.ID)

	case *Perception:
		// Update sensory data BEFORE movement
		i.visibleTargets = msg.Targets
		i.visibleFriends = msg.Friends

	case *Tick:
		i.updateAsRed()
		i.reportState(ctx)

	case *Convert:
		i.handleConversion(ctx, msg)

	case *GetState:
		i.respondState(ctx)

	default:
		ctx.Unhandled()
	}
}

func (i *Individual) updateAsRed() {
	if len(i.visibleTargets) > 0 {
		i.chaseClosestTarget()
	} else {
		// Wander when no targets visible
		i.vx += (rand.Float64() - 0.5) * 0.15
		i.vy += (rand.Float64() - 0.5) * 0.15
	}
	i.updateBouncePosition()
}

// ============================================================================
// BLUE BEHAVIOR: Flocking Prey
// ============================================================================

func (i *Individual) BlueBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		i.ID = ctx.Self().Name()
		i.Log(ctx.ActorSystem(), "%s started in BLUE mode", i.ID)

	case *Perception:
		// Update sensory data BEFORE movement
		i.visibleTargets = msg.Targets // Predators to flee from (future feature?)
		i.visibleFriends = msg.Friends // Flock-mates

	case *Tick:
		i.updateAsBlue()
		i.reportState(ctx)

	case *Convert:
		i.handleConversion(ctx, msg)

	case *GetState:
		i.respondState(ctx)

	default:
		ctx.Unhandled()
	}
}

func (i *Individual) updateAsBlue() {
	// Apply boids flocking rules
	vx, vy := ComputeBoidUpdate(i, i.visibleFriends, i.cfg)
	i.vx = vx
	i.vy = vy
	i.updateSoftTurnPosition()
}

// ============================================================================
// Shared Behaviors
// ============================================================================

func (i *Individual) handleConversion(ctx *actor.ReceiveContext, msg *Convert) {
	if msg.TargetColor == i.Color {
		return // Already this color
	}

	oldColor := i.Color
	i.Color = msg.TargetColor

	i.Log(ctx.ActorSystem(), "%s converting: %s → %s",
		ctx.Self().Name(), oldColor, i.Color)

	// Switch behavior function
	if i.Color == ColorRed {
		ctx.Become(i.RedBehavior)
	} else {
		ctx.Become(i.BlueBehavior)
	}

	// Visual feedback: "Explosion" effect
	i.vx *= -1.5
	i.vy *= -1.5

	// Reset sensory memory
	i.visibleTargets = nil
	i.visibleFriends = nil
}

func (i *Individual) reportState(ctx *actor.ReceiveContext) {
	state := i.makeState()

	// Reply to sender (should be World)
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
		VelocityX: i.vx,
		VelocityY: i.vy,
	}
}

// ============================================================================
// Physics / Movement
// ============================================================================

func (i *Individual) updateBouncePosition() {
	// Integrate velocity
	i.X += i.vx
	i.Y += i.vy

	// Bounce off walls
	if i.X < 0 {
		i.X = 0
		i.vx *= -1
	} else if i.X > i.cfg.WorldWidth {
		i.X = i.cfg.WorldWidth
		i.vx *= -1
	}

	if i.Y < 0 {
		i.Y = 0
		i.vy *= -1
	} else if i.Y > i.cfg.WorldHeight {
		i.Y = i.cfg.WorldHeight
		i.vy *= -1
	}

	// Prevent zero velocity (would break angle calculations)
	const minVel = 0.01
	if i.vx == 0 {
		i.vx = minVel
	}
	if i.vy == 0 {
		i.vy = minVel
	}
}

func (i *Individual) updateSoftTurnPosition() {
	// Apply screen-edge avoidance
	i.applySoftBoundaries()

	// Clamp speed
	i.applySpeedLimits()

	// Integrate velocity
	i.X += i.vx
	i.Y += i.vy
}

func (i *Individual) applySoftBoundaries() {
	const margin = 100.0

	if i.X < margin {
		i.vx += i.cfg.TurnFactor
	} else if i.X > i.cfg.WorldWidth-margin {
		i.vx -= i.cfg.TurnFactor
	}

	if i.Y < margin {
		i.vy += i.cfg.TurnFactor
	} else if i.Y > i.cfg.WorldHeight-margin {
		i.vy -= i.cfg.TurnFactor
	}
}

func (i *Individual) applySpeedLimits() {
	speed := math.Sqrt(i.vx*i.vx + i.vy*i.vy)

	if speed > i.cfg.MaxSpeed {
		scale := i.cfg.MaxSpeed / speed
		i.vx *= scale
		i.vy *= scale
	} else if speed < i.cfg.MinSpeed && speed > 0 {
		scale := i.cfg.MinSpeed / speed
		i.vx *= scale
		i.vy *= scale
	}
}

func (i *Individual) chaseClosestTarget() {
	if len(i.visibleTargets) == 0 {
		return
	}

	// Find nearest enemy
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

	if closest == nil {
		return
	}

	// Calculate pursuit vector
	dx := closest.PositionX - i.X
	dy := closest.PositionY - i.Y
	length := math.Sqrt(dx*dx + dy*dy)

	if length > 0 {
		// Normalize and scale by aggression
		dx = (dx / length) * i.cfg.Aggression
		dy = (dy / length) * i.cfg.Aggression

		i.vx += dx
		i.vy += dy
	}

	// Cap at max speed
	speed := math.Sqrt(i.vx*i.vx + i.vy*i.vy)
	if speed > i.cfg.MaxSpeed {
		scale := i.cfg.MaxSpeed / speed
		i.vx *= scale
		i.vy *= scale
	}
}

// ============================================================================
// Utilities
// ============================================================================

func (i *Individual) Log(sys actor.ActorSystem, format string, args ...interface{}) {
	sys.Logger().Infof("[%s] "+format, append([]interface{}{i.ID}, args...)...)
}
