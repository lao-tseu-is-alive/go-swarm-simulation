package simulation

import (
	"fmt"
	"math"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

type gridKey struct {
	x, y int
}

// WorldActor is the new "Brain." It manages the authoritative state and the spatial grid optimization.
type WorldActor struct {
	actors    map[string]*ActorState
	pids      []*actor.PID // Keep track of children
	uiChannel chan<- *WorldSnapshot
	// Optimization: Spatial Hashing
	// Map gridKey -> list of actors in that cell
	grid map[gridKey][]*ActorState
	// Communication with UI
	snapshotCh chan<- *WorldSnapshot
	// Game Settings (received from UI)
	detectionRadius float64
	defenseRadius   float64

	// Config for spawning
	numRed  int
	numBlue int
	// Store World Dimensions
	width  float64
	height float64
}

// NewWorldActor creates the world logic unit
func NewWorldActor(snapshotCh chan<- *WorldSnapshot, numRed, numBlue int, detR, defR, w, h float64) *WorldActor {
	return &WorldActor{
		actors:          make(map[string]*ActorState),
		grid:            make(map[gridKey][]*ActorState),
		snapshotCh:      snapshotCh,
		numRed:          numRed,
		numBlue:         numBlue,
		detectionRadius: detR,
		defenseRadius:   defR,
		width:           w,
		height:          h,
	}
}

func (w *WorldActor) PreStart(ctx *actor.Context) error {
	// 1. WE SPAWN THE POPULATION HERE NOW
	// The World is responsible for creating its inhabitants
	// Actually, Individuals need a way to talk back.
	// In this refactor, Individuals should send to ctx.Parent() (the World).
	ctx.ActorSystem().Logger().Info("World is spawning the swarm...")

	return nil
}

func (w *WorldActor) Receive(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		ctx.Logger().Info("World Started. Spawning Swarm...")
		w.spawnSwarm(ctx)

	// 1. Handle Updates from Individuals
	// You might need to add this message to your Proto or use a wrapper
	case *ActorState:
		w.actors[msg.Id] = msg
		// We could update the grid here incrementally, or rebuild it in Tick

	// 2. The Main Simulation Step (Driven by Game Loop)
	case *Tick:
		// A. Rebuild Spatial Grid for O(1) lookups
		w.rebuildGrid()

		// B. Run Game Logic (Collision/Defense)
		w.processInteractions(ctx)

		// C. Tick all children so they move
		for _, pid := range w.pids {
			// Forward the Tick to children
			ctx.Tell(pid, msg)
		}

		// D. Push Snapshot to UI
		// Convert map to slice for the renderer
		snapshot := &WorldSnapshot{
			Actors: make([]*ActorState, 0, len(w.actors)),
		}
		for _, state := range w.actors {
			snapshot.Actors = append(snapshot.Actors, state)
		}

		// Non-blocking send to avoid slowing down simulation if UI is slow
		select {
		case w.snapshotCh <- snapshot:
		default:
			// UI is busy, skip this frame update
		}

	// Handle dynamic slider updates from UI
	case *UpdateConfig: // Defined below or in Proto
		w.detectionRadius = msg.DetectionRadius
		w.defenseRadius = msg.DefenseRadius
	}
}

func (w *WorldActor) spawnSwarm(ctx *actor.ReceiveContext) {
	for i := 0; i < w.numRed; i++ {
		name := fmt.Sprintf("Red-%03d", i)
		// Spawn using ReceiveContext.Spawn (creates a child)
		pid := ctx.Spawn(name, NewIndividual(ColorRed, 50+float64(i)*20, 150, w.width, w.height))
		w.pids = append(w.pids, pid)
	}

	for i := 0; i < w.numBlue; i++ {
		name := fmt.Sprintf("Blue-%03d", i)
		pid := ctx.Spawn(name, NewIndividual(ColorBlue, float64(i)+300, 250, w.width, w.height))
		w.pids = append(w.pids, pid)
	}
}

func (w *WorldActor) rebuildGrid() {
	// Clear grid to reuse memory (Go 1.21+)
	clear(w.grid)

	cellSize := w.getCellSize()
	for _, a := range w.actors {
		gx, gy := int(a.PositionX/cellSize), int(a.PositionY/cellSize)
		key := gridKey{x: gx, y: gy}
		w.grid[key] = append(w.grid[key], a)
	}
}

func (w *WorldActor) getCellSize() float64 {
	// Use the largest radius to ensure our 3x3 grid check covers everything
	maxRadius := math.Max(w.detectionRadius, w.defenseRadius)
	// Clamp to a minimum of 10 to avoid tiny grids or div by zero
	return math.Max(maxRadius, 10.0)
}
func (w *WorldActor) getCellIndices(x, y float64) (int, int) {
	cs := w.getCellSize()
	return int(x / cs), int(y / cs)
}

// getNearbyActors retrieves all the actors in grids located in and around x,y  (3x3 Grid)
func (w *WorldActor) getNearbyActors(x, y float64) []*ActorState {
	gx, gy := w.getCellIndices(x, y)
	var neighbors []*ActorState

	// Loop through X-1 to X+1 and Y-1 to Y+1
	for i := gx - 1; i <= gx+1; i++ {
		for j := gy - 1; j <= gy+1; j++ {
			key := gridKey{x: i, y: j}
			if actors, ok := w.grid[key]; ok {
				neighbors = append(neighbors, actors...)
			}
		}
	}
	return neighbors
}

func (w *WorldActor) processInteractions(ctx *actor.ReceiveContext) {
	detSq := w.detectionRadius * w.detectionRadius
	defSq := w.defenseRadius * w.defenseRadius
	contactSq := 12.0 * 12.0

	// Iterate Red actors (Predators)
	for _, red := range w.actors {
		if red.Color != ColorRed {
			continue
		}

		var visibleTargets []*ActorState
		// OPTIMIZATION: Get only nearby actors (Prey Candidates)
		// This replaces `range w.actors` with a much smaller list
		potentialPrey := w.getNearbyActors(red.PositionX, red.PositionY)

		for _, other := range potentialPrey {
			if other.Color == ColorBlue {
				distSq := distSquared(red, other)

				// Perception
				if distSq < detSq {
					visibleTargets = append(visibleTargets, other)
				}

				// Collision
				if distSq < contactSq {
					// === COMBAT LOGIC ===
					// Check Defense
					defenders := 0
					// OPTIMIZATION: Get nearby actors around the VICTIM
					// We need to look around 'other', not 'red', though they are close.
					potentialDefenders := w.getNearbyActors(other.PositionX, other.PositionY)
					for _, def := range potentialDefenders {
						// A defender must be Blue, not the victim itself, and close enough
						if def.Color == ColorBlue && def.Id != other.Id {
							if distSquared(other, def) < defSq {
								defenders++
							}
						}
					}

					// Conversion Logic
					// Note: Lookups by ID string are slow; in production, cache PIDs or use the Grid
					// For now, we use the System to find the local actor by name (ID)
					targetPID, _ := ctx.ActorSystem().LocalActor(other.Id)
					myPID, _ := ctx.ActorSystem().LocalActor(red.Id) // Inefficient lookup, better to cache PIDs

					// Defense Mechanism
					if defenders >= 3 {
						// DEFENSE SUCCESS: Red converts to Blue
						if myPID != nil {
							ctx.Tell(myPID, &Convert{TargetColor: ColorBlue})
						}
					} else {
						// DEFENSE FAILED: Blue converts to Red
						if targetPID != nil {
							ctx.Tell(targetPID, &Convert{TargetColor: ColorRed})
						}
					}
				}
			}
		}

		// Send Perception Update
		if len(visibleTargets) > 0 {
			myPID, _ := ctx.ActorSystem().LocalActor(red.Id)
			ctx.Tell(myPID, &Perception{Targets: visibleTargets})
		}
	}
}

func distSquared(a, b *ActorState) float64 {
	dx := a.PositionX - b.PositionX
	dy := a.PositionY - b.PositionY
	return dx*dx + dy*dy
}

func (w *WorldActor) PostStop(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Info("World is shutdown...")
	return nil
}
