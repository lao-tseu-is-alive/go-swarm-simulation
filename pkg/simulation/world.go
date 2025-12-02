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
	pidsCache map[string]*actor.PID
	uiChannel chan<- *WorldSnapshot
	// Optimization: Spatial Hashing
	// Map gridKey -> list of actors in that cell
	grid map[gridKey][]*ActorState
	// Communication with UI
	snapshotCh chan<- *WorldSnapshot
	// Game Settings (received from UI)
	detectionRadius  float64
	perceptionRadius float64 // For friends (Blue seeking Blue)
	defenseRadius    float64

	cfg *Config
}

// NewWorldActor creates the world logic unit
func NewWorldActor(snapshotCh chan<- *WorldSnapshot, cfg *Config) *WorldActor {
	return &WorldActor{
		actors:           make(map[string]*ActorState),
		pidsCache:        make(map[string]*actor.PID),
		grid:             make(map[gridKey][]*ActorState),
		snapshotCh:       snapshotCh,
		cfg:              cfg,
		detectionRadius:  cfg.DetectionRadius,
		defenseRadius:    cfg.DefenseRadius,
		perceptionRadius: cfg.PerceptionRadius,
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
			Actors:    make([]*ActorState, 0, len(w.actors)),
			RedCount:  0,
			BlueCount: 0,
		}
		for _, state := range w.actors {
			snapshot.Actors = append(snapshot.Actors, state)
			// "Free" calculation during iteration
			// We could increment/decrement a counter every time a Convert message happens.
			// However, in distributed actor systems, state drift is a common bug
			// (e.g., an actor dies without reporting, or a conversion message is lost).
			// Recalculating from the source of truth (w.actors map) every frame ensures
			// UI never desynchronizes from the actual simulation state.
			if state.Color == ColorRed {
				snapshot.RedCount++
			} else {
				snapshot.BlueCount++
			}
		}
		// We add a check: (snapshot.RedCount + snapshot.BlueCount > 0)
		// This ensures we don't trigger Game Over during the first few frames
		// when the map is still empty/initializing.
		totalPopulation := snapshot.RedCount + snapshot.BlueCount

		if totalPopulation > 0 {
			// Check Victory Condition
			if snapshot.RedCount == 0 {
				snapshot.IsGameOver = true
				snapshot.Winner = ColorBlue
			} else if snapshot.BlueCount == 0 {
				snapshot.IsGameOver = true
				snapshot.Winner = ColorRed
			}
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
	for i := 0; i < w.cfg.NumRedAtStart; i++ {
		name := fmt.Sprintf("Red-%03d", i)
		// Spawn using ReceiveContext.Spawn (creates a child)
		pid := ctx.Spawn(name, NewIndividual(ColorRed, 50+float64(i)*20, 150, 0.2, 0.2, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid
	}

	// BLUE SPAWN
	for i := 0; i < w.cfg.NumBlueAtStart; i++ {
		name := fmt.Sprintf("Blue-%03d", i)

		// SPREAD THEM OUT:
		// X: 300 + (i * 20) -> 300, 320, 340...
		// Y: 250 + (random jitter) -> prevents perfect vertical alignment
		startX := 300.0 + float64(i)*10.0
		startY := 250.0 + (float64(i%5) * 10.0) // Small zigzag

		// Bounds check spawn
		if startX > w.cfg.WorldWidth-50 {
			startX = 50 + float64(i)*5
		}

		pid := ctx.Spawn(name, NewIndividual(ColorBlue, startX, startY, 0.2, 0.2, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid
	}
}

func (w *WorldActor) rebuildGrid() {
	// 1. Reset slices to length 0, but keep capacity! it's better then clear(w.grid)
	// This allows to reuse the underlying arrays of the slices,
	// reducing memory allocation to almost zero during runtime.
	for k := range w.grid {
		w.grid[k] = w.grid[k][:0]
	}

	cellSize := w.getCellSize()
	for _, a := range w.actors {
		gx, gy := int(a.PositionX/cellSize), int(a.PositionY/cellSize)
		key := gridKey{x: gx, y: gy}

		// append will reuse the existing array capacity if available
		w.grid[key] = append(w.grid[key], a)
	}
}

func (w *WorldActor) getCellSize() float64 {
	// Use the largest radius to ensure our 3x3 grid check covers everything
	maxRadius := math.Max(w.detectionRadius, w.defenseRadius)
	maxRadius = math.Max(maxRadius, w.perceptionRadius)
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
	// Pre-calculate squared radii for performance
	detectionSq := w.detectionRadius * w.detectionRadius
	perceptionSq := w.perceptionRadius * w.perceptionRadius
	contactSq := w.cfg.ContactRadius * w.cfg.ContactRadius
	defSq := w.defenseRadius * w.defenseRadius

	// Iterate over every actor to calculate what they see and handle interactions
	for _, actorRef := range w.actors {
		// Optimization: Only check relevant grid cells
		nearby := w.getNearbyActors(actorRef.PositionX, actorRef.PositionY)

		var visibleEnemies []*ActorState
		var visibleFriends []*ActorState

		for _, other := range nearby {
			if other.Id == actorRef.Id {
				continue // Skip self
			}

			distSq := distSquared(actorRef, other)

			// 1. Is it a Friend? (For Flocking)
			if other.Color == actorRef.Color {
				if distSq < perceptionSq {
					visibleFriends = append(visibleFriends, other)
				}
			} else {
				// 2. Is it an Enemy? (For Detection)
				if distSq < detectionSq {
					visibleEnemies = append(visibleEnemies, other)
				}

				// 3. Combat Logic (Red attacks Blue)
				// We only execute this if 'actorRef' is Red and 'other' is Blue
				// to avoid double-processing the collision or processing Blue-on-Red (passive)
				if actorRef.Color == ColorRed && other.Color == ColorBlue {
					if distSq < contactSq {
						// === COMBAT LOGIC ===
						// Check Defense
						defenders := 0
						// Look for defenders around the VICTIM (other)
						// We perform a new grid lookup around the victim to be precise
						potentialDefenders := w.getNearbyActors(other.PositionX, other.PositionY)

						for _, def := range potentialDefenders {
							// A defender must be Blue, not the victim itself, and close enough
							if def.Color == ColorBlue && def.Id != other.Id {
								if distSquared(other, def) < defSq { // <--- defSq USED HERE
									defenders++
								}
							}
						}

						// Conversion Logic
						targetPID := w.pidsCache[other.Id] // Blue (Victim)
						myPID := w.pidsCache[actorRef.Id]  // Red (Attacker)

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
		}

		// Send Perception Update - always send to clear stale data
		if pid, ok := w.pidsCache[actorRef.Id]; ok {
			ctx.Tell(pid, &Perception{
				Targets: visibleEnemies, // Enemies (may be empty)
				Friends: visibleFriends, // Friends (may be empty)
			})
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
