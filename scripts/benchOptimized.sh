#!/bin/bash
# After implementing your optimization
go test -bench=BenchmarkRebuildGrid -benchmem ./pkg/simulation/... -count=5 | tee optimized.txt
# Statistical comparison
benchstat baseline.txt optimized.txt
