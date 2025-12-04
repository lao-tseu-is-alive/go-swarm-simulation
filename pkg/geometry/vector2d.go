package geometry

import (
	"errors"
	"fmt"
	"math"
)

// Epsilon Precision constant.
// In Go, we typically define these at the package level.
// We use a slightly more standard epsilon for float64 comparisons than the one in your TS file.
const (
	Epsilon = 1e-9
)

// Vector2D represents a 2D vector or point in cartesian space.
// We use public fields (X, Y) because they are fundamental data, not internal state.
// This is idiomatic in Go and allows for cleaner literal initialization: v := Vector2D{1, 2}
type Vector2D struct {
	X float64 `json:"x" protobuf:"x,1"`
	Y float64 `json:"y" protobuf:"y,1"`
}

// NewVector creates a new Vector2D.
// Note: In Go, it's often more idiomatic to simply use `Vector2D{X: x, Y: y}` directly,
// avoiding the function call overhead, but this factory is provided for API parity.
func NewVector(x, y float64) Vector2D {
	return Vector2D{X: x, Y: y}
}

// NewVectorPolar creates a new Vector2D from polar coordinates.
// theta is in radians.
func NewVectorPolar(radius, theta float64) Vector2D {
	// In Go, math.Cos/Sin expect radians.
	x := radius * math.Cos(theta)
	y := radius * math.Sin(theta)

	// Handle standard floating point precision issues near zero
	if math.Abs(x) < Epsilon {
		x = 0
	}
	if math.Abs(y) < Epsilon {
		y = 0
	}

	return Vector2D{X: x, Y: y}
}

// ---------------------------------------------------------------------
// Stringer Interface
// ---------------------------------------------------------------------

// String implements the fmt.Stringer interface.
// This allows the Vector2D to be printed cleanly using fmt.Println or %s.
func (v Vector2D) String() string {
	return fmt.Sprintf("(%.2f, %.2f)", v.X, v.Y)
}

// ---------------------------------------------------------------------
// Arithmetic Operations
// These methods use value receivers and return new Values.
// This ensures immutability and is efficient for small structs.
// ---------------------------------------------------------------------

// Add adds two vectors and returns the result.
func (v Vector2D) Add(other Vector2D) Vector2D {
	return Vector2D{v.X + other.X, v.Y + other.Y}
}

// Sub subtracts the other vector from the current vector.
func (v Vector2D) Sub(other Vector2D) Vector2D {
	return Vector2D{v.X - other.X, v.Y - other.Y}
}

// Mul scales the vector by a scalar value.
func (v Vector2D) Mul(scalar float64) Vector2D {
	return Vector2D{v.X * scalar, v.Y * scalar}
}

// Div scales the vector by 1/scalar.
// if scalar is zero it returns a math.Inf vector X:Inf,Y:Inf
// explicit handling or panic depends on requirements; sticking to standard math behavior here).
func (v Vector2D) Div(scalar float64) (Vector2D, error) {
	if scalar == 0 {
		// However, returning Inf is safer than panicking for math libraries.
		return Vector2D{math.Inf(1), math.Inf(1)}, errors.New("vector cannot be divided by zero")
	}
	return Vector2D{v.X / scalar, v.Y / scalar}, nil
}

// ---------------------------------------------------------------------
// Vector2D Products
// ---------------------------------------------------------------------

// Dot calculates the dot product of two vectors.
func (v Vector2D) Dot(other Vector2D) float64 {
	return v.X*other.X + v.Y*other.Y
}

// Cross calculates the 2D scalar cross product (z-component of 3D cross product).
// Useful for determining winding order or signed area.
func (v Vector2D) Cross(other Vector2D) float64 {
	return v.X*other.Y - v.Y*other.X
}

// ---------------------------------------------------------------------
// Magnitude and Normalization
// ---------------------------------------------------------------------

// LenSqr calculates the squared magnitude of the vector.
// This is faster than Len() as it avoids the square root. Use for comparisons.
func (v Vector2D) LenSqr() float64 {
	return v.X*v.X + v.Y*v.Y
}

// Len calculates the magnitude (length) of the vector.
// Note: We use the name 'Len' as it's common in Go (like standard library),
// but 'Magnitude' is also acceptable.
func (v Vector2D) Len() float64 {
	return math.Hypot(v.X, v.Y)
}

// Normalize returns a unit vector in the same direction.
// Returns a zero vector if the length is effectively zero.
func (v Vector2D) Normalize() Vector2D {
	l := v.Len()
	if l < Epsilon {
		return Vector2D{0, 0}
	}
	return v.Mul(1 / l)
}

// ---------------------------------------------------------------------
// Geometric Utilities
// ---------------------------------------------------------------------

// DistanceTo calculates the Euclidean distance to another vector.
func (v Vector2D) DistanceTo(other Vector2D) float64 {
	return v.Sub(other).Len()
}

// DistanceSquaredTo calculates the squared Euclidean distance to another vector.
func (v Vector2D) DistanceSquaredTo(other Vector2D) float64 {
	return v.Sub(other).LenSqr()
}

// Angle returns the angle (in radians) of the vector relative to the X-axis.
// Range: [-Pi, Pi]
func (v Vector2D) Angle() float64 {
	return math.Atan2(v.Y, v.X)
}

// AngleTo calculates the angle (in radians) between the vector and another vector.
func (v Vector2D) AngleTo(other Vector2D) float64 {
	return math.Atan2(other.Y-v.Y, other.X-v.X)
}

// Rotate rotates the vector by angle (in radians) around the origin (0,0).
func (v Vector2D) Rotate(angle float64) Vector2D {
	cosTheta := math.Cos(angle)
	sinTheta := math.Sin(angle)
	return Vector2D{
		X: v.X*cosTheta - v.Y*sinTheta,
		Y: v.X*sinTheta + v.Y*cosTheta,
	}
}

// RotateAround rotates the vector by angle (radians) around a specific center point.
func (v Vector2D) RotateAround(angle float64, center Vector2D) Vector2D {
	// Translate so center is origin
	translated := v.Sub(center)
	// Rotate
	rotated := translated.Rotate(angle)
	// Translate back
	return rotated.Add(center)
}

// Lerp (Linear Interpolate) calculates a point between v and target based on t [0, 1].
func (v Vector2D) Lerp(target Vector2D, t float64) Vector2D {
	// Formula: v + (target - v) * t
	return v.Add(target.Sub(v).Mul(t))
}

// Project projects vector v onto vector on.
func (v Vector2D) Project(on Vector2D) Vector2D {
	scalar := v.Dot(on) / on.LenSqr()
	return on.Mul(scalar)
}

// ---------------------------------------------------------------------
// Comparison
// ---------------------------------------------------------------------

// Eq checks if two vectors are approximately equal using the Epsilon constant.
// This handles floating point inaccuracies.
func (v Vector2D) Eq(other Vector2D) bool {
	return math.Abs(v.X-other.X) <= Epsilon && math.Abs(v.Y-other.Y) <= Epsilon
}
