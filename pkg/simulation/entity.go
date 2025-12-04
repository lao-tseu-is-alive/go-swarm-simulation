package simulation

import "github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"

type Entity struct {
	ID    string
	Color TeamColor
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
func (e *Entity) ToProto() *ActorState {
	return &ActorState{
		Id:        e.ID,
		Color:     e.Color,
		PositionX: e.Pos.X,
		PositionY: e.Pos.Y,
		VelocityX: e.Vel.X,
		VelocityY: e.Vel.Y,
	}
}

// FromProto (if needed) converts incoming messages back to Entities
func FromProto(p *ActorState) *Entity {
	return &Entity{
		ID:    p.Id,
		Color: p.Color,
		Pos:   geometry.Vector2D{X: p.PositionX, Y: p.PositionY},
		Vel:   geometry.Vector2D{X: p.VelocityX, Y: p.VelocityY},
	}
}
