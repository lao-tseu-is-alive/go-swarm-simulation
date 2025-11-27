#!/bin/bash
# Generate Go code
protoc --go_out=./internal/simulation/ --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    pb/simulation.proto