package simulation

import (
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

type Entity struct {
	ID    string
	Color pb.TeamColor
	Pos   geometry.Vector2D
	Vel   geometry.Vector2D

	// You can add fields here that are NEVER sent over the network
	// e.g., energy, health, state-machine-timer
	//Energy float64
}

// UpdatePhysics applies the velocity to Entity position
func (e *Entity) UpdatePhysics() {
	e.Pos = e.Pos.Add(e.Vel)
}

// DistanceTo gives the cartesian distance from this Entity and the other
func (e *Entity) DistanceTo(other *Entity) float64 {
	return e.Pos.Sub(other.Pos).Len()
}

// DistanceSquaredTo gives squared magnitude of the vector from this Entity and the other
func (e *Entity) DistanceSquaredTo(other *Entity) float64 {
	return e.Pos.Sub(other.Pos).LenSqr()
}

// ToProto converts the clean Entity into the Protobuf "Envelope"
func (e *Entity) ToProto() *pb.ActorState {
	return &pb.ActorState{
		Id:       e.ID,
		Color:    e.Color,
		Position: GeomVector2DToProto(e.Pos),
		Velocity: GeomVector2DToProto(e.Vel),
	}
}

// UpdateFromProto updates the entity's state from a Protobuf message
// without allocating new memory.
func (e *Entity) UpdateFromProto(p *pb.ActorState) {
	// We assume ID and Color don't change often, but Position/Velocity do.
	e.Pos = GeomVector2DFromProto(p.Position)
	e.Vel = GeomVector2DFromProto(p.Velocity)
	// Optional: Sync color if dynamic conversion happens outside the world
	e.Color = p.Color
}

func (e *Entity) ClampVelocity(minSpeed, maxSpeed float64) {
	speed := e.Vel.Len()
	if speed > maxSpeed {
		e.Vel = e.Vel.Mul(maxSpeed / speed)
	} else if speed < minSpeed && speed > 0 {
		e.Vel = e.Vel.Mul(minSpeed / speed)
	}
}

func (e *Entity) BounceOffWalls(width, height float64) {
	// Simple integration is usually done before bounce,
	// but assuming UpdatePhysics() called separately:
	if e.Pos.X < 0 {
		e.Pos.X = 0
		e.Vel.X *= -1
	} else if e.Pos.X > width {
		e.Pos.X = width
		e.Vel.X *= -1
	}
	if e.Pos.Y < 0 {
		e.Pos.Y = 0
		e.Vel.Y *= -1
	} else if e.Pos.Y > height {
		e.Pos.Y = height
		e.Vel.Y *= -1
	}
	// Prevent zero velocity
	if e.Vel.X == 0 && e.Vel.Y == 0 {
		e.Vel = geometry.Vector2D{X: 0.1, Y: 0.1}
	}
}

func (e *Entity) SoftBoundaries(width, height, turnFactor float64) {
	margin := 100.0
	if e.Pos.X < margin {
		e.Vel.X += turnFactor
	} else if e.Pos.X > width-margin {
		e.Vel.X -= turnFactor
	}
	if e.Pos.Y < margin {
		e.Vel.Y += turnFactor
	} else if e.Pos.Y > height-margin {
		e.Vel.Y -= turnFactor
	}
}

func (e *Entity) Seek(target geometry.Vector2D, strength, maxSpeed float64) {
	// 1. Desired velocity = vector to target
	desired := target.Sub(e.Pos)

	// 2. Normalize and Scale
	if desired.LenSqr() > 0 {
		desired = desired.Normalize().Mul(strength)
		// 3. Apply Steering Force (simply adding to velocity here for "drift" style)
		e.Vel = e.Vel.Add(desired)

		// 4. Cap Speed immediately so we don't explode
		e.ClampVelocity(0, maxSpeed)
	}
}

// FromProto (if needed) converts incoming messages back to Entities
func FromProto(p *pb.ActorState) *Entity {
	return &Entity{
		ID:    p.Id,
		Color: p.Color,
		Pos:   GeomVector2DFromProto(p.Position),
		Vel:   GeomVector2DFromProto(p.Velocity),
	}
}

// ----------------------------------------------------------------------------
//  Protobuf Mapping Helpers
// ----------------------------------------------------------------------------

// GeomVector2DToProto converts a Domain Vector2D to a Protobuf Vector message.
func GeomVector2DToProto(v geometry.Vector2D) *pb.Vector {
	return &pb.Vector{
		X: v.X,
		Y: v.Y,
	}
}

// GeomVector2DFromProto converts a Protobuf Vector message to a Domain Vector2D.
// It handles nil pointers gracefully (returning a zero vector).
func GeomVector2DFromProto(p *pb.Vector) geometry.Vector2D {
	if p == nil {
		return geometry.Vector2D{X: 0, Y: 0}
	}
	return geometry.Vector2D{
		X: p.X,
		Y: p.Y,
	}
}
