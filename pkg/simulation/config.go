package simulation

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Config struct {
	// World Dimensions
	// WorldWidth is the width of the simulation world in pixels.
	WorldWidth float64 `json:"worldWidth"`
	// WorldHeight is the height of the simulation world in pixels.
	WorldHeight float64 `json:"worldHeight"`

	// Population
	// NumRedAtStart is the initial number of Red (Aggressive) actors.
	NumRedAtStart int `json:"numRedAtStart"`
	// NumBlueAtStart is the initial number of Blue (Flocking) actors.
	NumBlueAtStart int `json:"numBlueAtStart"`

	// Interaction Radii
	// DetectionRadius is the radius within which Red actors can detect Blue actors.
	DetectionRadius float64 `json:"detectionRadius"`
	// DefenseRadius is the radius within which Blue actors can defend each other.
	DefenseRadius float64 `json:"defenseRadius"`
	// ContactRadius is the radius for close-range interactions (e.g., combat/conversion).
	ContactRadius float64 `json:"contactRadius"`

	// Physics / Behavior
	// MaxSpeed is the maximum speed an actor can travel per tick.
	MaxSpeed float64 `json:"maxSpeed"`
	// Aggression is a multiplier for the Red actors' chase force.
	Aggression float64 `json:"aggression"`

	// Boids flocking parameters (matching pkg/behavior/boid.go)
	// VisualRange is the radius within which Blue actors can see friends for Cohesion/Alignment.
	VisualRange float64 `json:"visualRange"`
	// ProtectedRange is the radius within which Blue actors try to avoid each other (Separation).
	ProtectedRange float64 `json:"protectedRange"`

	// CenteringFactor controls the strength of Cohesion (moving towards the center of neighbors).
	CenteringFactor float64 `json:"centeringFactor"`
	// AvoidFactor controls the strength of Separation (avoiding crowding).
	AvoidFactor float64 `json:"avoidFactor"`
	// MatchingFactor controls the strength of Alignment (matching velocity with neighbors).
	MatchingFactor float64 `json:"matchingFactor"`
	// TurnFactor controls how strongly actors turn away from the screen edges.
	TurnFactor float64 `json:"turnFactor"`

	// MinSpeed is the minimum speed a Blue actor tries to maintain.
	MinSpeed float64 `json:"minSpeed"`
}

func DefaultConfig() *Config {
	return &Config{
		WorldWidth:      1000,
		WorldHeight:     800,
		NumRedAtStart:   5,
		NumBlueAtStart:  30,
		DetectionRadius: 50,
		DefenseRadius:   40,
		ContactRadius:   12,
		VisualRange:     70.0,
		ProtectedRange:  20.0,
		CenteringFactor: 0.0005,
		AvoidFactor:     0.05,
		MatchingFactor:  0.05,
		TurnFactor:      0.2,
		MaxSpeed:        4.0,
		MinSpeed:        2.0,
		Aggression:      0.8,
	}
}

// LoadConfig loads configuration from a JSON file and validates it against the schema.
func LoadConfig(configFile string, schemaFile string) (*Config, error) {
	// 1. Compile Schema
	sch, err := jsonschema.Compile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	// 2. Read Config File
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	// 3. Validate
	var v interface{}
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		return nil, fmt.Errorf("failed to decode config json: %w", err)
	}

	if err := sch.Validate(v); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// 4. Unmarshal into Struct
	// We need to re-read or marshal the map back to bytes, or just decode again.
	// Since we already decoded into interface{}, let's just re-open or seek.
	// Simpler: Just read bytes first.
	b, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
