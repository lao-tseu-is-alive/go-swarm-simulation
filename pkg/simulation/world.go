package simulation

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/lao-tseu-is-alive/go-swarm-simulation/pb"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/geometry"
	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/goaktpb"
)

// gridKey is a spatial hash key for the grid-based neighbor lookup optimization.
type gridKey struct {
	x, y int
}

// WorldActor is the central coordinator of the simulation.
// It manages all entities, handles spatial hashing for efficient neighbor lookups,
// processes physics, and routes messages between actors.
type WorldActor struct {
	entities  map[string]*Entity
	pids      []*actor.PID // Keep track of children
	pidsCache map[string]*actor.PID
	uiChannel chan<- *pb.WorldSnapshot
	// Optimization: Spatial Hashing
	// Map gridKey -> list of entities in that cell
	grid map[gridKey][]*Entity
	// Communication with UI
	snapshotCh chan<- *pb.WorldSnapshot
	// Game Settings (received from UI)
	detectionRadius float64
	visualRange     float64 // For friends (Blue seeking Blue)
	defenseRadius   float64
	cfg             *Config
	// Benchmark Stats
	msgSentCount int
	msgRecvCount int
	lastLogTime  time.Time
}

// NewWorldActor creates the world logic unit
func NewWorldActor(snapshotCh chan<- *pb.WorldSnapshot, cfg *Config) *WorldActor {
	return &WorldActor{
		entities:        make(map[string]*Entity),
		pidsCache:       make(map[string]*actor.PID),
		grid:            make(map[gridKey][]*Entity),
		snapshotCh:      snapshotCh,
		cfg:             cfg,
		detectionRadius: cfg.RedDetectionRange,
		defenseRadius:   cfg.BlueDefenseRange,
		visualRange:     cfg.BlueFlockVision,
		msgSentCount:    0,
		msgRecvCount:    0,
		lastLogTime:     time.Now(),
	}
}

// PreStart is called when the WorldActor starts.
// It logs initialization but spawning happens in PostStart via the Receive handler.
func (w *WorldActor) PreStart(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Info("World is spawning the swarm...")
	return nil
}

// Receive handles messages sent to the WorldActor.
// Supported messages:
//   - PostStart: spawns the initial population
//   - ActorState: updates entity state from Individual actors
//   - Tick: runs one simulation step
//   - UpdateConfig: updates configuration and broadcasts to all actors
func (w *WorldActor) Receive(ctx *actor.ReceiveContext) {
	switch msg := ctx.Message().(type) {

	case *goaktpb.PostStart:
		ctx.Logger().Info("World Started. Spawning Swarm...")
		w.spawnSwarm(ctx)

	// 1. Handle Updates from Individuals
	// You might need to add this message to your Proto or use a wrapper
	case *pb.ActorState:
		w.msgRecvCount++
		if existing, ok := w.entities[msg.Id]; ok {
			existing.UpdateFromProto(msg)
		} else {
			// Only allocate if it's a new actor
			w.entities[msg.Id] = FromProto(msg)
		}

	// 2. The Main Simulation Step (Driven by Game Loop)
	case *pb.Tick:
		// 1. Telemetry
		w.logBenchmarks(ctx)

		// 2. Physics & Logic
		w.rebuildGrid()
		w.broadcastSimulationStep(ctx, msg.DeltaTime)

		// 3. UI Update
		w.pushSnapshot()

		// Handle dynamic config updates from UI
	case *pb.UpdateConfig:
		// Update World's own radii (used for spatial queries)
		w.detectionRadius = msg.GetDetectionRadius()
		w.defenseRadius = msg.GetDefenseRadius()
		w.visualRange = msg.GetVisualRange()

		// Update World's config copy (used for spawning new actors)
		w.cfg.RedDetectionRange = msg.GetDetectionRadius()
		w.cfg.BlueDefenseRange = msg.GetDefenseRadius()
		w.cfg.RedAttackRange = msg.GetContactRadius()
		w.cfg.BlueFlockVision = msg.GetVisualRange()
		w.cfg.BluePersonalSpace = msg.GetProtectedRange()
		w.cfg.MaxSpeed = msg.GetMaxSpeed()
		w.cfg.MinSpeed = msg.GetMinSpeed()
		w.cfg.RedAggression = msg.GetAggression()
		w.cfg.BlueCohesion = msg.GetCenteringFactor()
		w.cfg.BlueSeparation = msg.GetAvoidFactor()
		w.cfg.BlueAlignment = msg.GetMatchingFactor()
		w.cfg.BlueEdgeAvoidance = msg.GetTurnFactor()
		w.cfg.DisplayDetectionCircle = msg.GetDisplayDetectionCircle()
		w.cfg.DisplayDefenseCircle = msg.GetDisplayDefenseCircle()
		w.cfg.DisplaySpatialGrid = msg.GetDisplaySpatialGrid()

		// Population params (require restart)
		w.cfg.NumRedAtStart = int(msg.GetNumRedAtStart())
		w.cfg.NumBlueAtStart = int(msg.GetNumBlueAtStart())

		// BROADCAST to all Individual actors so they update their local copies
		// This is the key to avoiding race conditions - config flows via messages
		for _, pid := range w.pids {
			w.msgSentCount++
			ctx.Tell(pid, msg)
		}
	}
}

// logBenchmarks logs message throughput statistics once per second.
func (w *WorldActor) logBenchmarks(ctx *actor.ReceiveContext) {
	if time.Since(w.lastLogTime) >= time.Second {
		total := w.msgSentCount + w.msgRecvCount
		ctx.Logger().Infof("📊 MSG RATE: %d/sec (Sent: %d, Recv: %d) | Actors: %d",
			total, w.msgSentCount, w.msgRecvCount, len(w.entities))
		w.msgSentCount = 0
		w.msgRecvCount = 0
		w.lastLogTime = time.Now()
	}
}

// pushSnapshot sends the current world state to the UI channel.
// Non-blocking: if the channel is full, this frame is skipped.
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
	// 1. PRE-CACHE all proto representations (done once per tick)
	// This optimization reduces allocations from O(actors × neighbors) to O(actors)
	// by computing each entity's protobuf state once and reusing the pointer.
	protoCache := make(map[string]*pb.ActorState, len(w.entities))
	for id, entity := range w.entities {
		protoCache[id] = entity.ToProto()
	}

	// 2. Pre-calculate squared ranges to avoid Sqrt() calls in loops
	ranges := struct {
		perceptionSq float64
		detectionSq  float64
		contactSq    float64
	}{
		perceptionSq: w.visualRange * w.visualRange,
		detectionSq:  w.detectionRadius * w.detectionRadius,
		contactSq:    w.cfg.RedAttackRange * w.cfg.RedAttackRange,
	}

	for id, me := range w.entities {
		// 3. Scan grid for neighbors (Perception + Combat triggers)
		enemies, friends := w.scanNeighbors(ctx, me, ranges, protoCache)

		// 4. Construct the enriched Tick
		individualTick := &pb.Tick{
			DeltaTime: dt,
			Context: &pb.Perception{
				Targets: enemies,
				Friends: friends,
			},
		}

		// 5. Dispatch
		if pid, ok := w.pidsCache[id]; ok {
			w.msgSentCount++
			ctx.Tell(pid, individualTick)
		}
	}
}

// scanNeighbors iterates the spatial grid around 'me'.
// It populates perception lists AND handles combat interactions inline for efficiency.
// protoCache provides pre-computed protobuf states to avoid repeated allocations.
func (w *WorldActor) scanNeighbors(
	ctx *actor.ReceiveContext,
	me *Entity,
	ranges struct{ perceptionSq, detectionSq, contactSq float64 },
	protoCache map[string]*pb.ActorState,
) ([]*pb.ActorState, []*pb.ActorState) {
	var visibleEnemies []*pb.ActorState
	var visibleFriends []*pb.ActorState

	// Get grid bounds for the largest relevant radius (usually Detection or Perception)
	gx, gy := w.getCellIndices(me.Pos.X, me.Pos.Y)

	// Iterate 3x3 Grid
	for i := gx - 1; i <= gx+1; i++ {
		for j := gy - 1; j <= gy+1; j++ {
			key := gridKey{x: i, y: j}
			actorsInCell, ok := w.grid[key]
			if !ok {
				continue
			}

			for _, other := range actorsInCell {
				if other.ID == me.ID {
					continue
				}

				distSq := me.DistanceSquaredTo(other)

				// --- Logic Branching ---
				if other.Color == me.Color {
					// Friend Logic: Flocking
					if distSq < ranges.perceptionSq {
						// Use cached proto instead of allocating new one
						visibleFriends = append(visibleFriends, protoCache[other.ID])
					}
				} else {
					// Enemy Logic: Detection
					if distSq < ranges.detectionSq {
						// Use cached proto instead of allocating new one
						visibleEnemies = append(visibleEnemies, protoCache[other.ID])
					}
				}

				// Combat Logic: Red attacks Blue
				// We check this here to avoid re-iterating neighbors later
				if me.Color == pb.TeamColor_TEAM_RED && other.Color == pb.TeamColor_TEAM_BLUE {
					if distSq < ranges.contactSq {
						w.resolveCombat(ctx, me, other)
					}
				}
			}
		}
	}
	return visibleEnemies, visibleFriends
}

// resolveCombat handles the specific rules of engagement
func (w *WorldActor) resolveCombat(ctx *actor.ReceiveContext, attacker, victim *Entity) {
	// Optimization: Use the allocation-free counter we built previously
	defenders := w.countFriendsInRadius(
		victim.Pos,
		w.defenseRadius,
		pb.TeamColor_TEAM_BLUE, // Target is Blue defenders
		victim.ID,              // Exclude the victim themselves
	)

	if defenders >= 3 {
		// Defense Success: Attacker converts to Blue
		w.sendConvert(ctx, attacker.ID, pb.TeamColor_TEAM_BLUE)
	} else {
		// Defense Failed: Victim converts to Red
		w.sendConvert(ctx, victim.ID, pb.TeamColor_TEAM_RED)
	}
}

func (w *WorldActor) sendConvert(ctx *actor.ReceiveContext, targetID string, newColor pb.TeamColor) {
	if pid := w.pidsCache[targetID]; pid != nil {
		w.msgSentCount++
		ctx.Tell(pid, &pb.Convert{TargetColor: newColor})
	}
}

func (w *WorldActor) spawnSwarm(ctx *actor.ReceiveContext) {
	// 1. SPAWN REDS
	for i := 0; i < w.cfg.NumRedAtStart; i++ {
		name := fmt.Sprintf("Red-%04d", i)

		// Stochastic Placement: Uniform random distribution across the entire map
		startX := rand.Float64() * w.cfg.WorldWidth
		startY := rand.Float64() * w.cfg.WorldHeight

		// Random Velocity: range [-1, 1]
		vx := (rand.Float64() - 0.5) * 2
		vy := (rand.Float64() - 0.5) * 2

		pid := ctx.Spawn(name, NewIndividual(pb.TeamColor_TEAM_RED, startX, startY, vx, vy, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid

		w.entities[name] = &Entity{
			ID:    name,
			Color: pb.TeamColor_TEAM_RED,
			Pos:   geometry.Vector2D{X: startX, Y: startY},
			Vel:   geometry.Vector2D{X: vx, Y: vy},
		}
	}

	// 2. SPAWN BLUES
	for i := 0; i < w.cfg.NumBlueAtStart; i++ {
		name := fmt.Sprintf("Blue-%04d", i)

		// Stochastic Placement: Uniform random distribution
		startX := rand.Float64() * w.cfg.WorldWidth
		startY := rand.Float64() * w.cfg.WorldHeight

		vx := (rand.Float64() - 0.5) * 2
		vy := (rand.Float64() - 0.5) * 2

		pid := ctx.Spawn(name, NewIndividual(pb.TeamColor_TEAM_BLUE, startX, startY, vx, vy, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid

		w.entities[name] = &Entity{
			ID:    name,
			Color: pb.TeamColor_TEAM_BLUE,
			Pos: geometry.Vector2D{
				X: startX,
				Y: startY,
			},
			Vel: geometry.Vector2D{
				X: vx,
				Y: vy,
			},
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
	for _, a := range w.entities {
		gx, gy := int(a.Pos.X/cellSize), int(a.Pos.Y/cellSize)
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

// getNearbyActors retrieves all the entities in grids located in and around x,y  (3x3 Grid)
func (w *WorldActor) getNearbyActors(x, y float64) []*Entity {
	gx, gy := w.getCellIndices(x, y)
	var neighbors []*Entity

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

func (w *WorldActor) buildSnapshot() *pb.WorldSnapshot {
	snapshot := &pb.WorldSnapshot{
		Actors:    make([]*pb.ActorState, 0, len(w.entities)),
		RedCount:  0,
		BlueCount: 0,
	}

	for _, state := range w.entities {
		snapshot.Actors = append(snapshot.Actors, state.ToProto())
		if state.Color == pb.TeamColor_TEAM_RED {
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

func (w *WorldActor) PostStop(ctx *actor.Context) error {
	ctx.ActorSystem().Logger().Info("World is shutdown...")
	return nil
}

// countFriendsInRadius returns the count of entities of 'targetColor' within 'radius', excluding 'excludeID'.
// It performs 0 allocations.
func (w *WorldActor) countFriendsInRadius(center geometry.Vector2D, radius float64, targetColor pb.TeamColor, excludeID string) int {
	radiusSq := radius * radius
	cellSize := w.getCellSize()

	// Calculate grid bounds
	minGx := int((center.X - radius) / cellSize)
	maxGx := int((center.X + radius) / cellSize)
	minGy := int((center.Y - radius) / cellSize)
	maxGy := int((center.Y + radius) / cellSize)

	count := 0

	for gx := minGx; gx <= maxGx; gx++ {
		for gy := minGy; gy <= maxGy; gy++ {
			key := gridKey{x: gx, y: gy}
			if entities, ok := w.grid[key]; ok {
				for _, e := range entities {
					// 1. Check ID and Color FIRST (cheaper than math)
					if e.Color != targetColor || e.ID == excludeID {
						continue
					}

					// 2. Check Distance
					if e.Pos.DistanceSquaredTo(center) < radiusSq {
						count++
					}
				}
			}
		}
	}
	return count
}

// getActorsInRadius returns entities within a specific radius of (x, y)
// More efficient than getNearbyActors when radius << cellSize
func (w *WorldActor) getBlueActorsInRadius(x, y, radius float64) []*Entity {
	radiusSq := radius * radius
	cellSize := w.getCellSize()
	center := geometry.Vector2D{
		X: x,
		Y: y,
	}

	// Calculate grid bounds that could contain actors within radius
	minGx := int((x - radius) / cellSize)
	maxGx := int((x + radius) / cellSize)
	minGy := int((y - radius) / cellSize)
	maxGy := int((y + radius) / cellSize)

	var result []*Entity

	// Only scan necessary cells
	for gx := minGx; gx <= maxGx; gx++ {
		for gy := minGy; gy <= maxGy; gy++ {
			key := gridKey{x: gx, y: gy}
			if entities, ok := w.grid[key]; ok {
				for _, e := range entities {
					if e.Pos.DistanceSquaredTo(center) < radiusSq {
						result = append(result, e)
					}
				}
			}
		}
	}

	return result
}
