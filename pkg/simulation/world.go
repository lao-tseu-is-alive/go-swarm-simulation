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
	detectionRadius float64
	visualRange     float64 // For friends (Blue seeking Blue)
	defenseRadius   float64

	cfg *Config
}

// NewWorldActor creates the world logic unit
func NewWorldActor(snapshotCh chan<- *WorldSnapshot, cfg *Config) *WorldActor {
	return &WorldActor{
		actors:          make(map[string]*ActorState),
		pidsCache:       make(map[string]*actor.PID),
		grid:            make(map[gridKey][]*ActorState),
		snapshotCh:      snapshotCh,
		cfg:             cfg,
		detectionRadius: cfg.DetectionRadius,
		defenseRadius:   cfg.DefenseRadius,
		visualRange:     cfg.VisualRange,
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
		// STEP 1: Rebuild spatial grid with CURRENT positions
		w.rebuildGrid()

		// STEP 2: Calculate and SEND perception FIRST
		//         (so actors have fresh data when they process Tick)
		w.sendPerceptionUpdates(ctx)

		// STEP 3: Process game logic (conversions)
		w.processInteractions(ctx)

		// STEP 4: NOW tell actors to move
		//         They'll use the perception we just sent
		for _, pid := range w.pids {
			ctx.Tell(pid, msg)
		}

		// STEP 5: Push snapshot to UI
		snapshot := w.buildSnapshot()
		select {
		case w.snapshotCh <- snapshot:
		default:
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
	maxRadius = math.Max(maxRadius, w.visualRange)
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

// NEW METHOD: Separate perception broadcasting
func (w *WorldActor) sendPerceptionUpdates(ctx *actor.ReceiveContext) {
	perceptionSq := w.visualRange * w.visualRange
	detectionSq := w.detectionRadius * w.detectionRadius

	for _, actorRef := range w.actors {
		nearby := w.getNearbyActors(actorRef.PositionX, actorRef.PositionY)

		var visibleEnemies []*ActorState
		var visibleFriends []*ActorState

		for _, other := range nearby {
			if other.Id == actorRef.Id {
				continue
			}

			distSq := distSquared(actorRef, other)

			if other.Color == actorRef.Color {
				if distSq < perceptionSq {
					visibleFriends = append(visibleFriends, other)
				}
			} else {
				if distSq < detectionSq {
					visibleEnemies = append(visibleEnemies, other)
				}
			}
		}

		// Send fresh perception BEFORE they move
		if pid, ok := w.pidsCache[actorRef.Id]; ok {
			ctx.Tell(pid, &Perception{
				Targets: visibleEnemies,
				Friends: visibleFriends,
			})
		}
	}
}

// processInteractions  Only handle combat now
func (w *WorldActor) processInteractions(ctx *actor.ReceiveContext) {
	contactSq := w.cfg.ContactRadius * w.cfg.ContactRadius

	// Only iterate Red actors to avoid double-processing
	for _, attacker := range w.actors {
		if attacker.Color != ColorRed {
			continue // Skip Blues
		}

		nearby := w.getNearbyActors(attacker.PositionX, attacker.PositionY)

		for _, victim := range nearby {
			if victim.Color != ColorBlue {
				continue // Only attack Blues
			}

			distSq := distSquared(attacker, victim)
			if distSq >= contactSq {
				continue // Too far for combat
			}

			// === OPTIMIZED DEFENSE CHECK ===
			// Only search actors actually within defense radius
			potentialDefenders := w.getActorsInRadius(
				victim.PositionX,
				victim.PositionY,
				w.defenseRadius,
			)

			// === COMBAT LOGIC ===
			defenders := 0
			for _, def := range potentialDefenders {
				if def.Color == ColorBlue && def.Id != victim.Id {
					defenders++
				}
			}

			// Apply conversion
			if defenders >= 3 {
				// Defense success: Convert attacker
				if pid := w.pidsCache[attacker.Id]; pid != nil {
					ctx.Tell(pid, &Convert{TargetColor: ColorBlue})
				}
			} else {
				// Defense failed: Convert victim
				if pid := w.pidsCache[victim.Id]; pid != nil {
					ctx.Tell(pid, &Convert{TargetColor: ColorRed})
				}
			}
		}
	}
}

func (w *WorldActor) buildSnapshot() *WorldSnapshot {
	snapshot := &WorldSnapshot{
		Actors:    make([]*ActorState, 0, len(w.actors)),
		RedCount:  0,
		BlueCount: 0,
	}

	for _, state := range w.actors {
		snapshot.Actors = append(snapshot.Actors, state)
		if state.Color == ColorRed {
			snapshot.RedCount++
		} else {
			snapshot.BlueCount++
		}
	}

	totalPopulation := snapshot.RedCount + snapshot.BlueCount
	if totalPopulation > 0 {
		if snapshot.RedCount == 0 {
			snapshot.IsGameOver = true
			snapshot.Winner = ColorBlue
		} else if snapshot.BlueCount == 0 {
			snapshot.IsGameOver = true
			snapshot.Winner = ColorRed
		}
	}

	return snapshot
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

// getActorsInRadius returns actors within a specific radius of (x, y)
// More efficient than getNearbyActors when radius << cellSize
func (w *WorldActor) getActorsInRadius(x, y, radius float64) []*ActorState {
	radiusSq := radius * radius
	cellSize := w.getCellSize()

	// Calculate grid bounds that could contain actors within radius
	minGx := int((x - radius) / cellSize)
	maxGx := int((x + radius) / cellSize)
	minGy := int((y - radius) / cellSize)
	maxGy := int((y + radius) / cellSize)

	var result []*ActorState

	// Only scan necessary cells
	for gx := minGx; gx <= maxGx; gx++ {
		for gy := minGy; gy <= maxGy; gy++ {
			key := gridKey{x: gx, y: gy}
			if actors, ok := w.grid[key]; ok {
				for _, a := range actors {
					dx := a.PositionX - x
					dy := a.PositionY - y
					if dx*dx+dy*dy < radiusSq {
						result = append(result, a)
					}
				}
			}
		}
	}

	return result
}
