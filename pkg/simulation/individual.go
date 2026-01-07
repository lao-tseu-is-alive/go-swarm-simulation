package simulation

import (
	"math"
	"math/rand"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

const (
	// ColorRed is the display name for Red team actors (hunters).
	ColorRed = "🔴 RED"
	// ColorBlue is the display name for Blue team actors (flock).
	ColorBlue = "🔵 BLUE"
)

// Individual represents an actor in the simulation.
// Config values are OWNED (not shared) to avoid race conditions.
// Updates are received via pb.UpdateConfig messages from World.
type Individual struct {
	ID             string
	State          *Entity
	visibleTargets []*pb.ActorState // Enemies
	visibleFriends []*pb.ActorState // Allies

	// =========================================================================
	// OWNED CONFIG VALUES (not shared pointer)
	// These are updated via pb.UpdateConfig messages from World
	// =========================================================================

	// World dimensions
	worldWidth  float64
	worldHeight float64

	// Red actor parameters
	redAggression float64

	// Blue actor (boids) parameters
	blueFlockVision   float64
	bluePersonalSpace float64
	blueCohesion      float64
	blueSeparation    float64
	blueAlignment     float64
	blueEdgeAvoidance float64

	// Shared physics
	maxSpeed float64
	minSpeed float64
}

var _ actor.Actor = (*Individual)(nil)

// NewIndividual creates a new Individual with initial config values copied from cfg.
func NewIndividual(color pb.TeamColor, startX, startY, vx, vy float64, cfg *Config) *Individual {
	return &Individual{
		State: &Entity{
			// ID set in PreStart or derived later
			Color: color,
			Pos:   geometry.Vector2D{X: startX, Y: startY},
			Vel:   geometry.Vector2D{X: vx, Y: vy},
		},
		// Copy config values at creation time
		worldWidth:        cfg.WorldWidth,
		worldHeight:       cfg.WorldHeight,
		redAggression:     cfg.RedAggression,
		blueFlockVision:   cfg.BlueFlockVision,
		bluePersonalSpace: cfg.BluePersonalSpace,
		blueCohesion:      cfg.BlueCohesion,
		blueSeparation:    cfg.BlueSeparation,
		blueAlignment:     cfg.BlueAlignment,
		blueEdgeAvoidance: cfg.BlueEdgeAvoidance,
		maxSpeed:          cfg.MaxSpeed,
		minSpeed:          cfg.MinSpeed,
	}
}

// ============================================================================
// Actor Lifecycle Hooks
// ============================================================================

// PreStart initializes the Individual when the actor starts.
// It sets the ID from the actor name and logs the birth.
func (i *Individual) PreStart(ctx *actor.Context) error {
	i.ID = ctx.ActorName()
	i.State.ID = i.ID // <--- FIX: Ensure State has the ID
	i.Log(ctx.ActorSystem(), "Born: %s (%s) at %s",
		i.ID, i.State.Color, i.State.Pos)
	return nil
}

// PostStop is called when the actor is stopping.
// It logs the death of the actor.
func (i *Individual) PostStop(ctx *actor.Context) error {
	i.Log(ctx.ActorSystem(), "Death: %s", ctx.ActorName())
	return nil
}

// ============================================================================
// Message Routing (Entry Point)
// ============================================================================

// Receive is the initial message handler before behavior is determined.
// It routes to RedBehavior or BlueBehavior based on the actor's current color.
func (i *Individual) Receive(ctx *actor.ReceiveContext) {
	// Route to appropriate behavior based on current color
	if i.State.Color == pb.TeamColor_TEAM_RED {
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

// RedBehavior handles messages when the actor is on the Red (hunter) team.
// Red actors chase Blue targets and use hard wall collisions.
func (i *Individual) RedBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		i.ID = ctx.Self().Name()
		i.State.ID = i.ID // <--- FIX: Ensure State has the ID
		i.Log(ctx.ActorSystem(), "%s started in RED mode", i.ID)

	case *pb.Tick:
		// EXTRACT PERCEPTION
		if msg.Context != nil {
			i.visibleTargets = msg.Context.Targets
			i.visibleFriends = msg.Context.Friends
		}
		i.updateAsRed()
		i.reportState(ctx)

	case *pb.UpdateConfig:
		i.applyConfigUpdate(msg)

	case *pb.Convert:
		i.handleConversion(ctx, msg)

	case *pb.GetState:
		i.respondState(ctx)

	default:
		ctx.Unhandled()
	}
}

// updateAsRed performs Red actor physics: chase targets or wander.
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
	i.State.BounceOffWalls(i.worldWidth, i.worldHeight)
}

// ============================================================================
// BLUE BEHAVIOR: Flocking Prey
// ============================================================================

// BlueBehavior handles messages when the actor is on the Blue (flock) team.
// Blue actors use boids flocking rules and soft edge avoidance.
func (i *Individual) BlueBehavior(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		i.ID = ctx.Self().Name()
		i.State.ID = i.ID // <--- FIX: Ensure State has the ID
		i.Log(ctx.ActorSystem(), "%s started in BLUE mode", i.ID)

	case *pb.Tick:
		// EXTRACT PERCEPTION
		if msg.Context != nil {
			i.visibleTargets = msg.Context.Targets
			i.visibleFriends = msg.Context.Friends
		}
		i.updateAsBlue()
		i.reportState(ctx)

	case *pb.UpdateConfig:
		i.applyConfigUpdate(msg)

	case *pb.Convert:
		i.handleConversion(ctx, msg)

	case *pb.GetState:
		i.respondState(ctx)

	default:
		ctx.Unhandled()
	}
}

// updateAsBlue performs Blue actor physics: boids flocking behavior.
func (i *Individual) updateAsBlue() {
	// DEFENSIVE: Filter friends to match current color.
	// This handles the race condition where World sent perception data
	// before we processed a Convert message (identity crisis fix).
	validFriends := i.visibleFriends[:0] // Reuse slice backing - zero allocation
	for _, friend := range i.visibleFriends {
		if friend.Color == i.State.Color {
			validFriends = append(validFriends, friend)
		}
	}

	// Apply boids flocking rules using owned config values
	force := ComputeBoidUpdate(
		i.State,
		validFriends,
		i.blueFlockVision,
		i.bluePersonalSpace,
		i.blueCohesion,
		i.blueSeparation,
		i.blueAlignment,
	)

	i.State.Vel = i.State.Vel.Add(force) // Apply force
	i.State.SoftBoundaries(i.worldWidth, i.worldHeight, i.blueEdgeAvoidance)
	i.State.ClampVelocity(i.minSpeed, i.maxSpeed)
	i.State.UpdatePhysics()
}

// ============================================================================
// Config Update Handler
// ============================================================================

// applyConfigUpdate updates owned config values from a pb.UpdateConfig message.
// This is the only way config values change after actor creation.
func (i *Individual) applyConfigUpdate(msg *pb.UpdateConfig) {
	// Red params
	i.redAggression = msg.GetAggression()

	// Blue params
	i.blueFlockVision = msg.GetVisualRange()
	i.bluePersonalSpace = msg.GetProtectedRange()
	i.blueCohesion = msg.GetCenteringFactor()
	i.blueSeparation = msg.GetAvoidFactor()
	i.blueAlignment = msg.GetMatchingFactor()
	i.blueEdgeAvoidance = msg.GetTurnFactor()

	// Shared physics
	i.maxSpeed = msg.GetMaxSpeed()
	i.minSpeed = msg.GetMinSpeed()
}

// ============================================================================
// Shared Behaviors
// ============================================================================

// handleConversion switches the actor to a new team color.
// It changes behavior, applies a visual bounce effect, and resets sensory memory.
func (i *Individual) handleConversion(ctx *actor.ReceiveContext, msg *pb.Convert) {
	if msg.TargetColor == i.State.Color {
		return // Already this color
	}

	oldColor := i.State.Color
	i.State.Color = msg.TargetColor

	i.Log(ctx.ActorSystem(), "%s converting: %s → %s",
		ctx.Self().Name(), oldColor, i.State.Color)

	// Switch behavior function
	if i.State.Color == pb.TeamColor_TEAM_RED {
		ctx.Become(i.RedBehavior)
	} else {
		ctx.Become(i.BlueBehavior)
	}

	// Visual feedback: "Explosion" Bounce effect
	i.State.Vel = i.State.Vel.Mul(-1.5)

	// Reset sensory memory
	i.visibleTargets = nil
	i.visibleFriends = nil
}

// reportState sends the actor's current state back to the sender (World).
func (i *Individual) reportState(ctx *actor.ReceiveContext) {
	state := i.makeState()
	// Reply to sender (should be World)
	if ctx.Sender() != nil && ctx.Sender() != ctx.ActorSystem().NoSender() {
		ctx.Tell(ctx.Sender(), state)
	}
}

// respondState sends the actor's current state as a response.
func (i *Individual) respondState(ctx *actor.ReceiveContext) {
	ctx.Response(i.makeState())
}

// makeState creates a protobuf ActorState message from the entity.
func (i *Individual) makeState() *pb.ActorState {
	return i.State.ToProto()
}

// ============================================================================
// Physics / Movement
// ============================================================================

// chaseClosestTarget finds the nearest visible target and steers towards it.
func (i *Individual) chaseClosestTarget() {
	if len(i.visibleTargets) == 0 {
		return
	}

	// Find nearest enemy
	var closest *pb.ActorState
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
	pursuit := GeomVector2DFromProto(closest.Position).Sub(i.State.Pos)
	length := i.State.Pos.DistanceTo(GeomVector2DFromProto(closest.Position))

	if length > 0 {
		pursuit = pursuit.Normalize().Mul(i.redAggression)
		i.State.Vel = i.State.Vel.Add(pursuit)
	}

	// Cap at max speed
	speed := i.State.Vel.Len()
	if speed > i.maxSpeed {
		scale := i.maxSpeed / speed
		i.State.Vel = i.State.Vel.Mul(scale)
	}
}

// ============================================================================
// Utilities
// ============================================================================

// Log writes a debug log message with the actor ID prefix.
func (i *Individual) Log(sys actor.ActorSystem, format string, args ...interface{}) {
	sys.Logger().Debugf("[%s] "+format, append([]interface{}{i.ID}, args...)...)
}
