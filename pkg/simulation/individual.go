package simulation

import (
	"math"
	"math/rand"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

const (
	ColorRed  = "🔴 RED"
	ColorBlue = "🔵 BLUE"
)

type Individual struct {
	ID             string
	State          *Entity
	visibleTargets []*ActorState // Enemies
	visibleFriends []*ActorState // Allies
	cfg            *Config
}

var _ actor.Actor = (*Individual)(nil)

func NewIndividual(color TeamColor, startX, startY, vx, vy float64, cfg *Config) *Individual {
	return &Individual{
		State: &Entity{
			// ID set in PreStart or derived later
			Color: color,
			Pos:   geometry.Vector2D{X: startX, Y: startY},
			Vel:   geometry.Vector2D{X: vx, Y: vy},
		},
		cfg: cfg,
	}
}

// ============================================================================
// Actor Lifecycle Hooks
// ============================================================================

func (i *Individual) PreStart(ctx *actor.Context) error {
	i.ID = ctx.ActorName()
	i.State.ID = i.ID // <--- FIX: Ensure State has the ID
	i.Log(ctx.ActorSystem(), "Born: %s (%s) at %s",
		i.ID, i.State.Color, i.State.Pos)
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
	if i.State.Color == TeamColor_TEAM_RED {
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
		i.State.ID = i.ID // <--- FIX: Ensure State has the ID
		i.Log(ctx.ActorSystem(), "%s started in RED mode", i.ID)

	case *Tick:
		// EXTRACT PERCEPTION
		if msg.Context != nil {
			i.visibleTargets = msg.Context.Targets
			i.visibleFriends = msg.Context.Friends
		}
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
		jitter := geometry.Vector2D{
			X: (rand.Float64() - 0.5) * 0.15,
			Y: (rand.Float64() - 0.5) * 0.15,
		}
		i.State.Vel = i.State.Vel.Add(jitter)
	}
	i.State.UpdatePhysics() // Pos += Vel
	i.State.BounceOffWalls(i.cfg.WorldWidth, i.cfg.WorldHeight)
}

// ============================================================================
// BLUE BEHAVIOR: Flocking Prey
// ============================================================================

func (i *Individual) BlueBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		i.ID = ctx.Self().Name()
		i.State.ID = i.ID // <--- FIX: Ensure State has the ID
		i.Log(ctx.ActorSystem(), "%s started in BLUE mode", i.ID)

	case *Tick:
		// EXTRACT PERCEPTION
		if msg.Context != nil {
			i.visibleTargets = msg.Context.Targets
			i.visibleFriends = msg.Context.Friends
		}
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
	force := ComputeBoidUpdate(i.State, i.visibleFriends, i.cfg)
	//i.updateSoftTurnPosition()

	i.State.Vel = i.State.Vel.Add(force) // Apply force
	i.State.SoftBoundaries(i.cfg.WorldWidth, i.cfg.WorldHeight, i.cfg.TurnFactor)
	i.State.ClampVelocity(i.cfg.MinSpeed, i.cfg.MaxSpeed)
	i.State.UpdatePhysics()
}

// ============================================================================
// Shared Behaviors
// ============================================================================

func (i *Individual) handleConversion(ctx *actor.ReceiveContext, msg *Convert) {
	if msg.TargetColor == i.State.Color {
		return // Already this color
	}

	oldColor := i.State.Color
	i.State.Color = msg.TargetColor

	i.Log(ctx.ActorSystem(), "%s converting: %s → %s",
		ctx.Self().Name(), oldColor, i.State.Color)

	// Switch behavior function
	if i.State.Color == TeamColor_TEAM_RED {
		ctx.Become(i.RedBehavior)
	} else {
		ctx.Become(i.BlueBehavior)
	}

	// Visual feedback: "Explosion" Bounce effect
	i.State.Vel.Mul(-1.5)

	// Reset sensory memory
	i.visibleTargets = nil
	i.visibleFriends = nil
}

func (i *Individual) reportState(ctx *actor.ReceiveContext) {
	i.Log(ctx.ActorSystem(), "%s reportState i.State.Pos %s \tVel: %s", i.ID, i.State.Pos, i.State.Vel)
	state := i.makeState()
	// Reply to sender (should be World)
	if ctx.Sender() != nil && ctx.Sender() != ctx.ActorSystem().NoSender() {
		ctx.Tell(ctx.Sender(), state)
	}
}

func (i *Individual) respondState(ctx *actor.ReceiveContext) {
	i.Log(ctx.ActorSystem(), "%s respondState i.State.Pos %s \tVel: %s", i.ID, i.State.Pos, i.State.Vel)
	ctx.Response(i.makeState())
}

func (i *Individual) makeState() *ActorState {
	return i.State.ToProto()
}

// ============================================================================
// Physics / Movement
// ============================================================================

func (i *Individual) chaseClosestTarget() {
	if len(i.visibleTargets) == 0 {
		return
	}

	// Find nearest enemy
	var closest *ActorState
	minDistSq := math.MaxFloat64

	for _, target := range i.visibleTargets {
		distSq := i.State.Pos.DistanceSquaredTo(GeomVector2DFromProto(target.Position))

		if distSq < minDistSq {
			minDistSq = distSq
			closest = target
		}
	}

	if closest == nil {
		return
	}

	// Calculate pursuit vector
	pursuit := i.State.Pos.Sub(i.State.Pos)
	length := i.State.Pos.DistanceTo(GeomVector2DFromProto(closest.Position))

	if length > 0 {
		pursuit.Normalize().Mul(i.cfg.Aggression)
		i.State.Vel = i.State.Vel.Add(pursuit)
	}

	// Cap at max speed
	speed := i.State.Vel.Len()
	if speed > i.cfg.MaxSpeed {
		scale := i.cfg.MaxSpeed / speed
		i.State.Vel = i.State.Vel.Mul(scale)
	}
}

// ============================================================================
// Utilities
// ============================================================================

func (i *Individual) Log(sys actor.ActorSystem, format string, args ...interface{}) {
	sys.Logger().Debugf("[%s] "+format, append([]interface{}{i.ID}, args...)...)
}
