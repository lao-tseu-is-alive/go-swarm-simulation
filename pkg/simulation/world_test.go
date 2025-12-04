package simulation

import (
	"testing"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
)

func TestWorldActor_rebuildGrid(t *testing.T) {
	// 1. Setup
	// We need a WorldActor with specific dimensions and radii to determine cell size = max(detection, defense, 10)
	// Let's use detection=100, defense=50 -> cell size = 100
	cfg := &Config{
		WorldWidth:      1000,
		WorldHeight:     1000,
		DetectionRadius: 100,
		DefenseRadius:   50,
	}
	w := NewWorldActor(nil, cfg)

	// Create some entities
	a1 := &Entity{ID: "a1", Pos: geometry.Vector2D{X: 50, Y: 50}}   // Grid 0,0
	a2 := &Entity{ID: "a2", Pos: geometry.Vector2D{X: 150, Y: 50}}  // Grid 1,0
	a3 := &Entity{ID: "a3", Pos: geometry.Vector2D{X: 50, Y: 150}}  // Grid 0,1
	a4 := &Entity{ID: "a4", Pos: geometry.Vector2D{X: 250, Y: 250}} // Grid 2,2

	w.entities["a1"] = a1
	w.entities["a2"] = a2
	w.entities["a3"] = a3
	w.entities["a4"] = a4

	// 2. Execute
	w.rebuildGrid()

	// 3. Verify
	// Helper to check if actor is in list
	contains := func(list []*Entity, id string) bool {
		for _, a := range list {
			if a.ID == id {
				return true
			}
		}
		return false
	}

	// Check 0,0
	key00 := gridKey{x: 0, y: 0}
	if list, ok := w.grid[key00]; !ok || !contains(list, "a1") {
		t.Errorf("Expected a1 in grid 0,0, got %v", list)
	}

	// Check 1,0
	key10 := gridKey{x: 1, y: 0}
	if list, ok := w.grid[key10]; !ok || !contains(list, "a2") {
		t.Errorf("Expected a2 in grid 1,0, got %v", list)
	}

	// Check 0,1
	key01 := gridKey{x: 0, y: 1}
	if list, ok := w.grid[key01]; !ok || !contains(list, "a3") {
		t.Errorf("Expected a3 in grid 0,1, got %v", list)
	}

	// Check 2,2
	key22 := gridKey{x: 2, y: 2}
	if list, ok := w.grid[key22]; !ok || !contains(list, "a4") {
		t.Errorf("Expected a4 in grid 2,2, got %v", list)
	}

	// Ensure no cross-contamination
	if contains(w.grid[key00], "a2") {
		t.Errorf("Did not expect a2 in grid 0,0")
	}
}

func TestWorldActor_getNearbyActors(t *testing.T) {
	// Setup: Cell size = 100
	cfg := &Config{
		WorldWidth:      1000,
		WorldHeight:     1000,
		DetectionRadius: 100,
		DefenseRadius:   50,
	}
	w := NewWorldActor(nil, cfg)

	// Populate grid manually for precise control
	// Center is 1,1 (x=150, y=150)
	// Neighbors: 0,0; 1,0; 2,0; 0,1; 2,1; 0,2; 1,2; 2,2

	// Inside 3x3 block centered at 1,1
	center := &Entity{ID: "center", Pos: geometry.Vector2D{X: 150, Y: 150}}   // 1,1
	neighbor := &Entity{ID: "neighbor", Pos: geometry.Vector2D{X: 50, Y: 50}} // 0,0

	// Outside 3x3 block
	farAway := &Entity{ID: "far", Pos: geometry.Vector2D{X: 350, Y: 350}} // 3,3

	w.grid[gridKey{x: 1, y: 1}] = []*Entity{center}
	w.grid[gridKey{x: 0, y: 0}] = []*Entity{neighbor}
	w.grid[gridKey{x: 3, y: 3}] = []*Entity{farAway}

	// Execute: Query near center (150, 150) -> Grid 1,1
	// Should return contents of 0,0 to 2,2
	result := w.getNearbyActors(150, 150)

	// Verify
	foundCenter := false
	foundNeighbor := false
	foundFar := false

	for _, a := range result {
		if a.ID == "center" {
			foundCenter = true
		}
		if a.ID == "neighbor" {
			foundNeighbor = true
		}
		if a.ID == "far" {
			foundFar = true
		}
	}

	if !foundCenter {
		t.Error("Expected to find center actor")
	}
	if !foundNeighbor {
		t.Error("Expected to find neighbor actor (in 0,0)")
	}
	if foundFar {
		t.Error("Should NOT find far actor (in 3,3)")
	}
}

func BenchmarkWorldActor_rebuildGrid(b *testing.B) {
	// Setup: 1000 entities
	cfg := &Config{
		WorldWidth:      1000,
		WorldHeight:     1000,
		DetectionRadius: 100,
		DefenseRadius:   50,
	}
	w := NewWorldActor(nil, cfg)
	for i := 0; i < 1000; i++ {
		id := string(rune(i))
		w.entities[id] = &Entity{ID: id, Pos: geometry.Vector2D{X: float64(i), Y: float64(i)}}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.rebuildGrid()
	}
}

func BenchmarkWorldActor_getNearbyActors(b *testing.B) {
	// Setup: Populated grid
	cfg := &Config{
		WorldWidth:      1000,
		WorldHeight:     1000,
		DetectionRadius: 100,
		DefenseRadius:   50,
	}
	w := NewWorldActor(nil, cfg)
	// Fill grid with some entities
	for i := 0; i < 1000; i++ {
		id := string(rune(i))
		a := &Entity{ID: id, Pos: geometry.Vector2D{X: float64(i % 1000), Y: float64(i % 1000)}}
		w.entities[id] = a
	}
	w.rebuildGrid()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Query middle of the map
		w.getNearbyActors(500, 500)
	}
}
