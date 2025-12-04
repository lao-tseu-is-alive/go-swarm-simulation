package geometry

import (
	"math"
	"testing"
)

// floatEquals is a helper for testing scalar float values with epsilon.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) <= Epsilon
}

func TestNewVector(t *testing.T) {
	v := NewVector(1, 2)
	if v.X != 1 || v.Y != 2 {
		t.Errorf("NewVector(1, 2) = %v; want (1, 2)", v)
	}
}

func TestNewVectorPolar(t *testing.T) {
	tests := []struct {
		name   string
		radius float64
		theta  float64
		want   Vector2D
	}{
		{"Zero radius", 0, 0, Vector2D{0, 0}},
		{"Zero angle (X-axis)", 10, 0, Vector2D{10, 0}},
		{"90 degrees (Y-axis)", 10, math.Pi / 2, Vector2D{0, 10}},
		{"180 degrees (Negative X)", 10, math.Pi, Vector2D{-10, 0}},
		{"45 degrees", math.Sqrt(2), math.Pi / 4, Vector2D{1, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewVectorPolar(tt.radius, tt.theta)
			if !got.Eq(tt.want) {
				t.Errorf("NewVectorPolar(%v, %v) = %v; want %v", tt.radius, tt.theta, got, tt.want)
			}
		})
	}
}

func TestVector_String(t *testing.T) {
	v := Vector2D{1.234, 5.678}
	want := "(1.23, 5.68)" // Expecting rounding to 2 decimals based on implementation
	if got := v.String(); got != want {
		t.Errorf("Vector2D.String() = %q; want %q", got, want)
	}
}

func TestVector_Arithmetic(t *testing.T) {
	v1 := Vector2D{1, 2}
	v2 := Vector2D{3, 4}

	t.Run("Add", func(t *testing.T) {
		want := Vector2D{4, 6}
		if got := v1.Add(v2); !got.Eq(want) {
			t.Errorf("%v.Add(%v) = %v; want %v", v1, v2, got, want)
		}
	})

	t.Run("Sub", func(t *testing.T) {
		want := Vector2D{-2, -2}
		if got := v1.Sub(v2); !got.Eq(want) {
			t.Errorf("%v.Sub(%v) = %v; want %v", v1, v2, got, want)
		}
	})

	t.Run("Mul", func(t *testing.T) {
		want := Vector2D{2, 4}
		if got := v1.Mul(2); !got.Eq(want) {
			t.Errorf("%v.Mul(2) = %v; want %v", v1, got, want)
		}
	})

	t.Run("Div", func(t *testing.T) {
		want := Vector2D{0.5, 1}
		got, err := v1.Div(2)
		if err != nil {
			t.Errorf("%v.Div(2), generated error :%v but it shouldn't result= %v; want %v", v1, err, got, want)
		}
		if !got.Eq(want) {
			t.Errorf("%v.Div(2) = %v; want %v", v1, got, want)
		}
	})

	t.Run("DivByZero", func(t *testing.T) {
		got, err := v1.Div(0)
		if err == nil {
			t.Errorf("%v.Div(0), should have generated error,  but it didn't result=%v", v1, got)
		}
		if !math.IsInf(got.X, 0) || !math.IsInf(got.Y, 0) {
			t.Errorf("Div(0) should result in Inf coordinates, got %v", got)
		}
	})
}

func TestVector_Products(t *testing.T) {
	v1 := Vector2D{1, 0}
	v2 := Vector2D{0, 1}
	v3 := Vector2D{1, 1}

	t.Run("Dot", func(t *testing.T) {
		// Orthogonal
		if got := v1.Dot(v2); got != 0 {
			t.Errorf("Dot orthogonal = %v; want 0", got)
		}
		// Parallel
		if got := v1.Dot(Vector2D{2, 0}); got != 2 {
			t.Errorf("Dot parallel = %v; want 2", got)
		}
	})

	t.Run("Cross", func(t *testing.T) {
		// Z-component of cross product of X and Y unit vectors is 1
		if got := v1.Cross(v2); got != 1 {
			t.Errorf("Cross X,Y = %v; want 1", got)
		}
		// Parallel vectors cross is 0
		if got := v3.Cross(v3); got != 0 {
			t.Errorf("Cross self = %v; want 0", got)
		}
	})
}

func TestVector_Magnitude(t *testing.T) {
	v := Vector2D{3, 4} // 3-4-5 triangle

	t.Run("Len", func(t *testing.T) {
		if got := v.Len(); got != 5 {
			t.Errorf("Len = %v; want 5", got)
		}
	})

	t.Run("LenSqr", func(t *testing.T) {
		if got := v.LenSqr(); got != 25 {
			t.Errorf("LenSqr = %v; want 25", got)
		}
	})

	t.Run("Normalize", func(t *testing.T) {
		got := v.Normalize()
		want := Vector2D{0.6, 0.8}
		if !got.Eq(want) {
			t.Errorf("Normalize = %v; want %v", got, want)
		}
		if !floatEquals(got.Len(), 1.0) {
			t.Errorf("Normalize length = %v; want 1", got.Len())
		}
	})

	t.Run("NormalizeZero", func(t *testing.T) {
		zero := Vector2D{0, 0}
		got := zero.Normalize()
		if !got.Eq(zero) {
			t.Errorf("Normalize(0,0) = %v; want (0,0)", got)
		}
	})
}

func TestVector_Distance(t *testing.T) {
	v1 := Vector2D{1, 1}
	v2 := Vector2D{4, 5} // dx=3, dy=4, dist=5

	if got := v1.DistanceTo(v2); got != 5 {
		t.Errorf("DistanceTo = %v; want 5", got)
	}

	if got := v1.DistanceSquaredTo(v2); got != 25 {
		t.Errorf("DistanceSquaredTo = %v; want 25", got)
	}
}

func TestVector_Angles(t *testing.T) {
	t.Run("Angle", func(t *testing.T) {
		tests := []struct {
			v    Vector2D
			want float64
		}{
			{Vector2D{1, 0}, 0},
			{Vector2D{0, 1}, math.Pi / 2},
			{Vector2D{-1, 0}, math.Pi}, // math.Atan2 returns Pi for (-1, 0)
			{Vector2D{0, -1}, -math.Pi / 2},
		}
		for _, tt := range tests {
			if got := tt.v.Angle(); !floatEquals(got, tt.want) {
				t.Errorf("%v.Angle() = %v; want %v", tt.v, got, tt.want)
			}
		}
	})

	t.Run("AngleTo", func(t *testing.T) {
		v1 := Vector2D{1, 1}
		v2 := Vector2D{1, 2} // Directly above v1
		// Vector2D from v1 to v2 is (0, 1), angle should be Pi/2
		got := v1.AngleTo(v2)
		if !floatEquals(got, math.Pi/2) {
			t.Errorf("AngleTo = %v; want %v", got, math.Pi/2)
		}
	})
}

func TestVector_Transformations(t *testing.T) {
	t.Run("Rotate", func(t *testing.T) {
		v := Vector2D{1, 0}
		// Rotate 90 deg
		got := v.Rotate(math.Pi / 2)
		want := Vector2D{0, 1}
		if !got.Eq(want) {
			t.Errorf("Rotate(90) = %v; want %v", got, want)
		}
	})

	t.Run("RotateAround", func(t *testing.T) {
		// Rotate (2, 1) around (1, 1) by 90 degrees
		// Relative vector is (1, 0). Rotated 90 is (0, 1).
		// Add center (1, 1) -> Result (1, 2)
		v := Vector2D{2, 1}
		center := Vector2D{1, 1}
		got := v.RotateAround(math.Pi/2, center)
		want := Vector2D{1, 2}
		if !got.Eq(want) {
			t.Errorf("RotateAround = %v; want %v", got, want)
		}
	})
}

func TestVector_Utilities(t *testing.T) {
	t.Run("Lerp", func(t *testing.T) {
		v1 := Vector2D{0, 0}
		v2 := Vector2D{10, 10}
		got := v1.Lerp(v2, 0.5)
		want := Vector2D{5, 5}
		if !got.Eq(want) {
			t.Errorf("Lerp(0.5) = %v; want %v", got, want)
		}
	})

	t.Run("Project", func(t *testing.T) {
		// Project (3, 3) onto X-axis (5, 0)
		v := Vector2D{3, 3}
		on := Vector2D{5, 0}
		got := v.Project(on)
		want := Vector2D{3, 0}
		if !got.Eq(want) {
			t.Errorf("Project = %v; want %v", got, want)
		}
	})
}

func TestVector_Eq(t *testing.T) {
	v := Vector2D{1, 2}

	// Exact match
	if !v.Eq(Vector2D{1, 2}) {
		t.Error("Eq exact match failed")
	}

	// Epsilon match
	vClose := Vector2D{1 + Epsilon/2, 2 - Epsilon/2}
	if !v.Eq(vClose) {
		t.Error("Eq epsilon match failed")
	}

	// No match
	vDiff := Vector2D{1.1, 2}
	if v.Eq(vDiff) {
		t.Error("Eq mismatch failed")
	}
}
