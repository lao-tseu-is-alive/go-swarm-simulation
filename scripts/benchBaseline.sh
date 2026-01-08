#!/bin/bash
go test -bench=BenchmarkRebuildGrid -benchmem ./pkg/simulation/... -count=5 | tee baseline.txt
