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

type gridKey struct {
	x, y int
}

// WorldActor is the new "Brain." It manages the authoritative state and the spatial grid optimization.
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
	// --- Benchmark Stats ---
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
		// Update radii
		w.detectionRadius = msg.GetDetectionRadius()
		w.defenseRadius = msg.GetDefenseRadius()
		w.visualRange = msg.GetVisualRange()

		// Update config for other parameters (these affect new calculations)
		w.cfg.DetectionRadius = msg.GetDetectionRadius()
		w.cfg.DefenseRadius = msg.GetDefenseRadius()
		w.cfg.ContactRadius = msg.GetContactRadius()
		w.cfg.VisualRange = msg.GetVisualRange()
		w.cfg.ProtectedRange = msg.GetProtectedRange()
		w.cfg.MaxSpeed = msg.GetMaxSpeed()
		w.cfg.MinSpeed = msg.GetMinSpeed()
		w.cfg.Aggression = msg.GetAggression()
		w.cfg.CenteringFactor = msg.GetCenteringFactor()
		w.cfg.AvoidFactor = msg.GetAvoidFactor()
		w.cfg.MatchingFactor = msg.GetMatchingFactor()
		w.cfg.TurnFactor = msg.GetTurnFactor()
		w.cfg.DisplayDetectionCircle = msg.GetDisplayDetectionCircle()
		w.cfg.DisplayDefenseCircle = msg.GetDisplayDefenseCircle()

		// Note: Population parameters (NumRedAtStart, NumBlueAtStart)
		// are stored but require a simulation restart to take effect
		w.cfg.NumRedAtStart = int(msg.GetNumRedAtStart())
		w.cfg.NumBlueAtStart = int(msg.GetNumBlueAtStart())
	}
}

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

	for id, me := range w.entities {
		// 1. Scan grid for neighbors (Perception + Combat triggers)
		enemies, friends := w.scanNeighbors(ctx, me, ranges)

		// 2. Construct the enriched Tick
		individualTick := &pb.Tick{
			DeltaTime: dt,
			Context: &pb.Perception{
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
func (w *WorldActor) scanNeighbors(ctx *actor.ReceiveContext, me *Entity, ranges struct{ perceptionSq, detectionSq, contactSq float64 }) ([]*pb.ActorState, []*pb.ActorState) {
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
						visibleFriends = append(visibleFriends, other.ToProto())
					}
				} else {
					// Enemy Logic: Detection
					if distSq < ranges.detectionSq {
						visibleEnemies = append(visibleEnemies, other.ToProto())
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

		pid := ctx.Spawn(name, NewIndividual(pb.TeamColor_TEAM_RED, startX, startY, vx, vy, w.cfg))
		w.pids = append(w.pids, pid)
		w.pidsCache[name] = pid

		// We must insert the actor into the map NOW, so the very first Tick loop
		// sees it and sends it a message.
		w.entities[name] = &Entity{
			ID:    name,
			Color: pb.TeamColor_TEAM_RED,
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

// NEW METHOD: Separate perception broadcasting
func (w *WorldActor) sendPerceptionUpdates(ctx *actor.ReceiveContext) {
	perceptionSq := w.visualRange * w.visualRange
	detectionSq := w.detectionRadius * w.detectionRadius

	for _, entity := range w.entities {
		nearby := w.getNearbyActors(entity.Pos.X, entity.Pos.Y)

		var visibleEnemies []*pb.ActorState
		var visibleFriends []*pb.ActorState

		for _, other := range nearby {
			if other.ID == entity.ID {
				continue
			}

			distSq := entity.DistanceSquaredTo(other)

			if other.Color == entity.Color {
				if distSq < perceptionSq {
					visibleFriends = append(visibleFriends, other.ToProto())
				}
			} else {
				if distSq < detectionSq {
					visibleEnemies = append(visibleEnemies, other.ToProto())
				}
			}
		}

		// Send fresh perception BEFORE they move
		if pid, ok := w.pidsCache[entity.ID]; ok {
			w.msgSentCount++ // COUNT PERCEPTION MSG
			ctx.Tell(pid, &pb.Perception{
				Targets: visibleEnemies,
				Friends: visibleFriends,
			})
		}
	}
}

// processInteractions  Only handle combat now
func (w *WorldActor) processInteractions(ctx *actor.ReceiveContext) {
	contactSq := w.cfg.ContactRadius * w.cfg.ContactRadius

	// Only iterate Red entities to avoid double-processing
	for _, attacker := range w.entities {
		if attacker.Color != pb.TeamColor_TEAM_RED {
			continue // Skip Blues
		}

		nearby := w.getNearbyActors(attacker.Pos.X, attacker.Pos.Y)

		for _, victim := range nearby {
			if victim.Color != pb.TeamColor_TEAM_BLUE {
				continue // Only attack Blues
			}

			distSq := attacker.DistanceSquaredTo(victim)
			if distSq >= contactSq {
				continue // Too far for combat
			}

			// === COMBAT LOGIC ===
			defenders := w.countFriendsInRadius(
				victim.Pos,
				w.defenseRadius,
				pb.TeamColor_TEAM_BLUE,
				victim.ID,
			)

			// Apply conversion
			if defenders >= 3 {
				// Defense success: Convert attacker
				if pid := w.pidsCache[attacker.ID]; pid != nil {
					w.msgSentCount++ // <--- COUNT CONVERT MSG
					ctx.Tell(pid, &pb.Convert{TargetColor: pb.TeamColor_TEAM_BLUE})
				}
			} else {
				// Defense failed: Convert victim
				if pid := w.pidsCache[victim.ID]; pid != nil {
					w.msgSentCount++ // <--- COUNT CONVERT MSG
					ctx.Tell(pid, &pb.Convert{TargetColor: pb.TeamColor_TEAM_RED})
				}
			}
		}
	}
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
