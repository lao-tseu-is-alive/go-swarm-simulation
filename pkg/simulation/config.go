package simulation

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Config struct {
	// World Dimensions
	WorldWidth  float64 `json:"worldWidth"`
	WorldHeight float64 `json:"worldHeight"`

	// Population
	NumRedAtStart  int `json:"numRedAtStart"`
	NumBlueAtStart int `json:"numBlueAtStart"`

	// Interaction Radii
	DetectionRadius float64 `json:"detectionRadius"`
	DefenseRadius   float64 `json:"defenseRadius"`
	ContactRadius   float64 `json:"contactRadius"` // Previously hardcoded 12.0

	// Physics / Behavior
	MaxSpeed   float64 `json:"maxSpeed"`   // Previously 5.0
	Aggression float64 `json:"aggression"` // Previously 0.8

	// Boids flocking parameters (matching pkg/behavior/boid.go)
	VisualRange    float64 `json:"visualRange"`    // How far can they see?
	ProtectedRange float64 `json:"protectedRange"` // Personal space radius

	CenteringFactor float64 `json:"centeringFactor"` // Cohesion strength
	AvoidFactor     float64 `json:"avoidFactor"`     // Separation strength
	MatchingFactor  float64 `json:"matchingFactor"`  // Alignment strength
	TurnFactor      float64 `json:"turnFactor"`      // Edge turning strength

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
