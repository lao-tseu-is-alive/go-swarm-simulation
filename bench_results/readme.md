# Grid Benchmarking and Visualization - Walkthrough

## Summary

Implemented benchmarking harness and grid visualization. **Also tested the incremental grid update hypothesis — it proved slower than the current approach.**

---

## Experimental Results: Incremental Grid Updates

**Hypothesis**: Track which cell each actor is in, only update when they cross cell boundaries.

**Result**: ❌ **Slower by 62-514%**

| Scenario | Baseline | Incremental | Change |
|----------|----------|-------------|--------|
| 10% Moved | 32.4µs | 52.5µs | **+62%** |
| 50% Moved | 35.6µs | 161.4µs | **+354%** |
| 90% Moved | 41.9µs | 256.9µs | **+514%** |

### Why?

1. **Map lookup overhead**: `actorCells[actor.ID]` string hashing adds cost for every actor
2. **Removal cost**: Even with O(1) swap-and-truncate, iterating to find the actor in the cell slice costs time
3. **Current approach is already efficient**: Slice capacity reuse means ~0 allocations

**Conclusion**: The current full-rebuild with slice capacity reuse is **optimal** for this use case. The benchmark infrastructure is now in place to test future optimizations.

**Conclusion (bis)**: "Premature optimization is the root of all evil" - Donald Knuth

---

## Completed Features

### 1. Benchmarking Harness

Added to [world_test.go](file:///home/cgil/cgdev/golang/go-swarm-simulation/pkg/simulation/world_test.go):

- `setupBenchWorld(count, cellSize)` - Creates test world
- `simulateMovement(w, ratio, cellSize)` - Simulates cell crossing
- 7 scenario benchmarks: 10/50/90% movement, small/large cells, high density

**Usage**:
```bash
go test -bench=BenchmarkRebuildGrid -benchmem ./pkg/simulation/... -count=5 | tee results.txt
benchstat baseline.txt results.txt
```

### 2. Grid Visualization

New **"Show Spatial Grid"** checkbox in Settings → Visualization:

| File | Change |
|------|--------|
| [config.go](file:///home/cgil/cgdev/golang/go-swarm-simulation/pkg/simulation/config.go) | Added `DisplaySpatialGrid` |
| [simulation.proto](file:///home/cgil/cgdev/golang/go-swarm-simulation/pb/simulation.proto) | Added `display_spatial_grid` |
| [game.go](file:///home/cgil/cgdev/golang/go-swarm-simulation/pkg/simulation/game.go) | Added checkbox, `drawSpatialGrid()` |

The grid updates live as Detection/Vision sliders change.

---

## Verification

- ✅ All tests pass
- ✅ Build succeeds
- ✅ Benchmarks functional (0 allocs/op)
- ✅ Grid visualization works
