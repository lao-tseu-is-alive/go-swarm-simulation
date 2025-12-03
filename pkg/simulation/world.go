package simulation

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

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
	cfg             *Config
	// --- Benchmark Stats ---
	msgSentCount int
	msgRecvCount int
	lastLogTime  time.Time
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
		msgSentCount:    0,
		msgRecvCount:    0,
		lastLogTime:     time.Now(),
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
		w.msgRecvCount++
		w.actors[msg.Id] = msg

	// 2. The Main Simulation Step (Driven by Game Loop)
	case *Tick:
		// 1. Telemetry
		w.logBenchmarks(ctx)

		// 2. Physics & Logic
		w.rebuildGrid()
		w.broadcastSimulationStep(ctx, msg.DeltaTime)

		// 3. UI Update
		w.pushSnapshot()

	// Handle dynamic slider updates from UI
	case *UpdateConfig: // Defined below or in Proto
		w.detectionRadius = msg.DetectionRadius
		w.defenseRadius = msg.DefenseRadius
	}
}

func (w *WorldActor) logBenchmarks(ctx *actor.ReceiveContext) {
	if time.Since(w.lastLogTime) >= time.Second {
		total := w.msgSentCount + w.msgRecvCount
		ctx.Logger().Infof("📊 MSG RATE: %d/sec (Sent: %d, Recv: %d) | Actors: %d",
			total, w.msgSentCount, w.msgRecvCount, len(w.actors))
		w.msgSentCount = 0
		w.msgRecvCount = 0
		w.lastLogTime = time.Now()
	}
}

func (w *WorldActor) pushSnapshot() {
	select {
	case w.snapshotCh <- w.buildSnapshot():
	default:
		// UI busy, skip frame
	}
}

// broadcastSimulationStep is the "Mega Loop" optimized for single-pass execution.
// It combines Perception gathering, Combat Logic, and Tick dispatching.
func (w *WorldActor) broadcastSimulationStep(ctx *actor.ReceiveContext, dt int64) {
	// Pre-calculate squared ranges to avoid Sqrt() calls in loops
	ranges := struct {
		perceptionSq float64
		detectionSq  float64
		contactSq    float64
	}{
		perceptionSq: w.visualRange * w.visualRange,
		detectionSq:  w.detectionRadius * w.detectionRadius,
		contactSq:    w.cfg.ContactRadius * w.cfg.ContactRadius,
	}

	for id, me := range w.actors {
		// 1. Scan grid for neighbors (Perception + Combat triggers)
		enemies, friends := w.scanNeighbors(ctx, me, ranges)

		// 2. Construct the enriched Tick
		individualTick := &Tick{
			DeltaTime: dt,
			Context: &Perception{
				Targets: enemies,
				Friends: friends,
			},
		}

		// 3. Dispatch
		if pid, ok := w.pidsCache[id]; ok {
			w.msgSentCount++
			ctx.Tell(pid, individualTick)
		}
	}
}

// scanNeighbors iterates the spatial grid around 'me'.
// It populates perception lists AND handles combat interactions inline for efficiency.
func (w *WorldActor) scanNeighbors(ctx *actor.ReceiveContext, me *ActorState, ranges struct{ perceptionSq, detectionSq, contactSq float64 }) ([]*ActorState, []*ActorState) {
	var visibleEnemies []*ActorState
	var visibleFriends []*ActorState

	// Get grid bounds for the largest relevant radius (usually Detection or Perception)
	gx, gy := w.getCellIndices(me.PositionX, me.PositionY)

	// Iterate 3x3 Grid
	for i := gx - 1; i <= gx+1; i++ {
		for j := gy - 1; j <= gy+1; j++ {
			key := gridKey{x: i, y: j}
			actorsInCell, ok := w.grid[key]
			if !ok {
				continue
			}

			for _, other := range actorsInCell {
				if other.Id == me.Id {
					continue
				}

				distSq := distSquared(me, other)

				// --- Logic Branching ---
				if other.Color == me.Color {
					// Friend Logic: Flocking
					if distSq < ranges.perceptionSq {
						visibleFriends = append(visibleFriends, other)
					}
				} else {
					// Enemy Logic: Detection
					if distSq < ranges.detectionSq {
						visibleEnemies = append(visibleEnemies, other)
					}

					// Combat Logic: Red attacks Blue
					// We check this here to avoid re-iterating neighbors later
					if me.Color == ColorRed && other.Color == ColorBlue {
						if distSq < ranges.contactSq {
							w.resolveCombat(ctx, me, other)
						}
					}
				}
			}
		}
	}
	return visibleEnemies, visibleFriends
}

// resolveCombat handles the specific rules of engagement
func (w *WorldActor) resolveCombat(ctx *actor.ReceiveContext, attacker, victim *ActorState) {
	// Optimization: Use the allocation-free counter we built previously
	defenders := w.countFriendsInRadius(
		victim.PositionX,
		victim.PositionY,
		w.defenseRadius,
		ColorBlue, // Target is Blue defenders
		victim.Id, // Exclude the victim themselves
	)

	if defenders >= 3 {
		// Defense Success: Attacker converts to Blue
		w.sendConvert(ctx, attacker.Id, ColorBlue)
	} else {
		// Defense Failed: Victim converts to Red
		w.sendConvert(ctx, victim.Id, ColorRed)
	}
}

func (w *WorldActor) sendConvert(ctx *actor.ReceiveContext, targetID string, newColor string) {
	if pid := w.pidsCache[targetID]; pid != nil {
		w.msgSentCount++
		ctx.Tell(pid, &Convert{TargetColor: newColor})
	}
}

func (w *WorldActor) spawnSwarm(ctx *actor.ReceiveContext) {
	var (
		redX     = w.cfg.WorldWidth / 6
		redY     = w.cfg.WorldHeight / 6
		incRedX  = math.Min(w.cfg.WorldHeight/float64(w.cfg.NumRedAtStart), w.cfg.DetectionRadius)
		incRedY  = math.Min(w.cfg.WorldHeight/float64(w.cfg.NumRedAtStart), w.cfg.DetectionRadius)
		blueX    = (w.cfg.WorldWidth / 4) * 2
		blueY    = (w.cfg.WorldHeight / 4) * 2
		incBlueX = math.Min(w.cfg.WorldHeight/float64(w.cfg.NumBlueAtStart), w.cfg.DefenseRadius)
		incBlueY = math.Min(w.cfg.WorldHeight/float64(w.cfg.NumBlueAtStart), w.cfg.DefenseRadius)
	)
	// 1. SPAWN REDS
	for i := 0; i < w.cfg.NumRedAtStart; i++ {
		name := fmt.Sprintf("Red-%03d", i)
		startX := redX + float64(i)*incRedX*rand.Float64()*2
		startY := redY + float64(i)*incRedY*rand.Float64()*2
		// Bounds check spawn
		if startX > w.cfg.WorldWidth-50 {
			startX = 50 + float64(i)*5
		}
		if startY > w.cfg.WorldHeight-50 {
			startY = 50 + float64(i)*5
		}
		// Calculate Random Velocity HERE
		vx := (rand.Float64() - 0.5) * 2
		vy := (rand.Float64() - 0.5) * 2

		pid := ctx.Spawn(name, NewIndividual(ColorRed, startX, startY, vx, vy, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid

		// We must insert the actor into the map NOW, so the very first Tick loop
		// sees it and sends it a message.
		w.actors[name] = &ActorState{
			Id:        name,
			Color:     ColorRed,
			PositionX: startX,
			PositionY: startY,
			VelocityX: vx,
			VelocityY: vy,
		}
	}

	// 2. SPAWN BLUES
	for i := 0; i < w.cfg.NumBlueAtStart; i++ {
		name := fmt.Sprintf("Blue-%03d", i)

		startX := blueX + float64(i)*incBlueX*rand.Float64()*2
		startY := blueY + (float64(i%5)*incBlueY)*rand.Float64()*2
		// Bounds check spawn
		if startX > w.cfg.WorldWidth-50 {
			startX = 50 + float64(i)*5
		}
		if startY > w.cfg.WorldHeight-50 {
			startY = 50 + float64(i)*5
		}
		vx := (rand.Float64() - 0.5) * 2
		vy := (rand.Float64() - 0.5) * 2

		pid := ctx.Spawn(name, NewIndividual(ColorBlue, startX, startY, vx, vy, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid

		w.actors[name] = &ActorState{
			Id:        name,
			Color:     ColorBlue,
			PositionX: startX,
			PositionY: startY,
			VelocityX: vx,
			VelocityY: vy,
		}
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
			w.msgSentCount++ // COUNT PERCEPTION MSG
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

			// === COMBAT LOGIC ===
			defenders := w.countFriendsInRadius(
				victim.PositionX,
				victim.PositionY,
				w.defenseRadius,
				ColorBlue,
				victim.Id,
			)

			// Apply conversion
			if defenders >= 3 {
				// Defense success: Convert attacker
				if pid := w.pidsCache[attacker.Id]; pid != nil {
					w.msgSentCount++ // <--- COUNT CONVERT MSG
					ctx.Tell(pid, &Convert{TargetColor: ColorBlue})
				}
			} else {
				// Defense failed: Convert victim
				if pid := w.pidsCache[victim.Id]; pid != nil {
					w.msgSentCount++ // <--- COUNT CONVERT MSG
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

// countFriendsInRadius returns the count of actors of 'targetColor' within 'radius', excluding 'excludeID'.
// It performs 0 allocations.
func (w *WorldActor) countFriendsInRadius(x, y, radius float64, targetColor string, excludeID string) int {
	radiusSq := radius * radius
	cellSize := w.getCellSize()

	// Calculate grid bounds
	minGx := int((x - radius) / cellSize)
	maxGx := int((x + radius) / cellSize)
	minGy := int((y - radius) / cellSize)
	maxGy := int((y + radius) / cellSize)

	count := 0

	for gx := minGx; gx <= maxGx; gx++ {
		for gy := minGy; gy <= maxGy; gy++ {
			key := gridKey{x: gx, y: gy}
			if actors, ok := w.grid[key]; ok {
				for _, a := range actors {
					// 1. Check ID and Color FIRST (cheaper than math)
					if a.Color != targetColor || a.Id == excludeID {
						continue
					}

					// 2. Check Distance
					dx := a.PositionX - x
					dy := a.PositionY - y
					if dx*dx+dy*dy < radiusSq {
						count++
					}
				}
			}
		}
	}
	return count
}

// getActorsInRadius returns actors within a specific radius of (x, y)
// More efficient than getNearbyActors when radius << cellSize
func (w *WorldActor) getBlueActorsInRadius(x, y, radius float64) []*ActorState {
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
