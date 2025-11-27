package simulation

import (
	"fmt"
	"math"

	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

// WorldActor is the new "Brain." It manages the authoritative state and the spatial grid optimization.
type WorldActor struct {
	actors    map[string]*ActorState
	pids      []*actor.PID // Keep track of children
	uiChannel chan<- *WorldSnapshot
	// Optimization: Spatial Hashing
	// Map "gridX:gridY" -> list of actors in that cell
	grid map[string][]*ActorState
	// Communication with UI
	snapshotCh chan<- *WorldSnapshot
	// Game Settings (received from UI)
	detectionRadius float64
	defenseRadius   float64

	// Config for spawning
	numRed  int
	numBlue int
}

// NewWorldActor creates the world logic unit
func NewWorldActor(snapshotCh chan<- *WorldSnapshot, numRed, numBlue int, detR, defR float64) *WorldActor {
	return &WorldActor{
		actors:          make(map[string]*ActorState),
		grid:            make(map[string][]*ActorState),
		snapshotCh:      snapshotCh,
		numRed:          numRed,
		numBlue:         numBlue,
		detectionRadius: detR,
		defenseRadius:   defR,
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
		name := fmt.Sprintf("Red-%d", i)
		// Spawn using ReceiveContext.Spawn (creates a child)
		pid := ctx.Spawn(name, NewIndividual(ColorRed, 50+float64(i)*20, 100))
		w.pids = append(w.pids, pid)
	}

	for i := 0; i < w.numBlue; i++ {
		name := fmt.Sprintf("Blue-%d-%d", i%10, i/10)
		pid := ctx.Spawn(name, NewIndividual(ColorBlue, 300, 100))
		w.pids = append(w.pids, pid)
	}
}

func (w *WorldActor) rebuildGrid() {
	// Clear grid
	w.grid = make(map[string][]*ActorState)

	cellSize := w.getCellSize()
	for _, a := range w.actors {
		key := fmt.Sprintf("%d:%d", int(a.PositionX/cellSize), int(a.PositionY/cellSize))
		w.grid[key] = append(w.grid[key], a)
	}
}

func (w *WorldActor) getCellSize() float64 {
	cellSize := math.Min(w.detectionRadius, 10)
	return cellSize
}

func (w *WorldActor) getCellKey(x, y float64) string {
	cellSize := w.getCellSize()
	return fmt.Sprintf("%d:%d", int(x/cellSize), int(y/cellSize))
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

		// Optimization: Only check neighbors in current and adjacent grid cells
		// (omitted full neighbor check for brevity, assuming simple loop for now)
		// In a real grid, you'd calculate keys for (x-1, y-1) to (x+1, y+1)

		var visibleTargets []*ActorState

		for _, other := range w.actors {
			if other.Color == ColorBlue {
				distSq := distSquared(red, other)

				// Perception
				if distSq < detSq {
					visibleTargets = append(visibleTargets, other)
				}

				// Collision
				if distSq < contactSq {
					// Check Defense
					defenders := 0
					for _, def := range w.actors {
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

					if defenders >= 3 {
						// Red converts to Blue
						ctx.Tell(myPID, &Convert{TargetColor: ColorBlue})
					} else {
						// Blue converts to Red
						ctx.Tell(targetPID, &Convert{TargetColor: ColorRed})
					}
				}
			}
		}

		// Send Perception
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
