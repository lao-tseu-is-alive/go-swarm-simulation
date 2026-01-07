// Package simulation provides the core simulation logic for the swarm simulation game.
// It implements a predator-prey (Red vs Blue) simulation using an actor-based architecture
// with boids flocking behavior for Blue actors.
package simulation

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Config holds all configuration parameters for the simulation.
// It is organized into sections matching the UI panel layout.
// Config values are loaded from JSON and validated against a schema.
type Config struct {
	// =========================================================================
	// WORLD DIMENSIONS
	// =========================================================================

	// WorldWidth is the width of the simulation world in pixels.
	WorldWidth float64 `json:"worldWidth"`
	// WorldHeight is the height of the simulation world in pixels.
	WorldHeight float64 `json:"worldHeight"`

	// =========================================================================
	// POPULATION (UI: "Population" section - requires restart)
	// =========================================================================

	// NumRedAtStart is the initial number of Red (Hunter) actors.
	// UI Label: "Red Actors"
	NumRedAtStart int `json:"numRedAtStart"`
	// NumBlueAtStart is the initial number of Blue (Flock) actors.
	// UI Label: "Blue Actors"
	NumBlueAtStart int `json:"numBlueAtStart"`

	// =========================================================================
	// RED ACTOR SETTINGS (UI: "Red Hunter" section)
	// =========================================================================

	// RedDetectionRange is the radius within which Red actors can detect Blue actors.
	// Larger values make Red more aware of distant prey.
	// UI Label: "Detection Range"
	RedDetectionRange float64 `json:"redDetectionRange"`
	// RedAttackRange is the distance at which Red triggers combat with Blue.
	// Think of it as the "attack range" or "bite distance".
	// UI Label: "Attack Range"
	RedAttackRange float64 `json:"redAttackRange"`
	// RedAggression is a multiplier for how strongly Red chases detected Blue actors.
	// UI Label: "Aggression"
	RedAggression float64 `json:"redAggression"`

	// =========================================================================
	// BLUE ACTOR SETTINGS (UI: "Blue Flock" section)
	// =========================================================================

	// BlueFlockVision is the radius within which Blue actors see friends for flocking.
	// Used for Cohesion (moving toward flock center) and Alignment (matching velocity).
	// UI Label: "Flock Vision"
	BlueFlockVision float64 `json:"blueFlockVision"`
	// BluePersonalSpace is the minimum distance Blue tries to keep from other Blue.
	// Implements the Separation rule in boids flocking (personal space).
	// UI Label: "Personal Space"
	BluePersonalSpace float64 `json:"bluePersonalSpace"`
	// BlueDefenseRange is the range to count nearby Blue defenders during combat.
	// If >= 3 Blues are within this radius of a victim, the Red attacker converts to Blue.
	// UI Label: "Defense Range"
	BlueDefenseRange float64 `json:"blueDefenseRange"`

	// =========================================================================
	// FLOCKING STRENGTH (UI: "Flocking Tuning" section - Blue only)
	// =========================================================================

	// BlueCohesion controls Cohesion strength (moving towards neighbor center).
	// UI Label: "Cohesion"
	BlueCohesion float64 `json:"blueCohesion"`
	// BlueSeparation controls Separation strength (avoiding crowding neighbors).
	// UI Label: "Separation"
	BlueSeparation float64 `json:"blueSeparation"`
	// BlueAlignment controls Alignment strength (matching neighbor velocities).
	// UI Label: "Alignment"
	BlueAlignment float64 `json:"blueAlignment"`
	// BlueEdgeAvoidance controls how strongly Blue actors turn away from screen edges.
	// UI Label: "Edge Avoidance"
	BlueEdgeAvoidance float64 `json:"blueEdgeAvoidance"`

	// =========================================================================
	// PHYSICS (UI: "Physics (Both)" section - applies to Red and Blue)
	// =========================================================================

	// MaxSpeed is the maximum speed any actor can travel per tick.
	// UI Label: "Max Speed"
	MaxSpeed float64 `json:"maxSpeed"`
	// MinSpeed is the minimum speed Blue actors try to maintain.
	// UI Label: "Min Speed"
	MinSpeed float64 `json:"minSpeed"`

	// =========================================================================
	// LOGGING (not exposed in UI)
	// =========================================================================

	// LogLevel sets the logging level (debug, info, warn, error). Default: info
	LogLevel string `json:"logLevel"`
	// LogFormat sets the logging format (json, text). Default: json
	LogFormat string `json:"logFormat"`

	// =========================================================================
	// DEBUG VISUALIZATION (UI: "Visualization" section)
	// =========================================================================

	// DisplayDetectionCircle toggles drawing the detection radius around Red actors.
	// UI Label: "Show Detection Circle"
	DisplayDetectionCircle bool `json:"displayDetectionCircle"`
	// DisplayDefenseCircle toggles drawing the defense radius around Blue actors.
	// UI Label: "Show Defense Circle"
	DisplayDefenseCircle bool `json:"displayDefenseCircle"`

	// =========================================================================
	// SPRITES (not exposed in UI)
	// =========================================================================

	// RedSpritePath is the path to the red spaceship sprite design file.
	RedSpritePath string `json:"redSpritePath"`
	// RedPalettePath is the path to the red spaceship palette file.
	RedPalettePath string `json:"redPalettePath"`
	// BlueSpritePath is the path to the blue spaceship sprite design file.
	BlueSpritePath string `json:"blueSpritePath"`
	// BluePalettePath is the path to the blue spaceship palette file.
	BluePalettePath string `json:"bluePalettePath"`
}

// DefaultConfig returns a Config with sensible default values.
// These defaults are used when no config file is found or as a baseline.
func DefaultConfig() *Config {
	return &Config{
		WorldWidth:             1000,
		WorldHeight:            800,
		NumRedAtStart:          25,
		NumBlueAtStart:         250,
		RedDetectionRange:      50,
		RedAttackRange:         12,
		RedAggression:          0.8,
		BlueFlockVision:        70.0,
		BluePersonalSpace:      20.0,
		BlueDefenseRange:       40,
		BlueCohesion:           0.0005,
		BlueSeparation:         0.07,
		BlueAlignment:          0.06,
		BlueEdgeAvoidance:      0.2,
		MaxSpeed:               4.0,
		MinSpeed:               2.0,
		LogLevel:               "info",
		LogFormat:              "json",
		DisplayDetectionCircle: false,
		DisplayDefenseCircle:   false,
		RedSpritePath:          "assets/sprites/red_spaceship.sprite",
		RedPalettePath:         "assets/sprites/red_spaceship.palette",
		BlueSpritePath:         "assets/sprites/blue_spaceship.sprite",
		BluePalettePath:        "assets/sprites/blue_spaceship.palette",
	}
}

// Validate checks that config values are logically consistent.
// It returns an error if any validation rule is violated.
func (c *Config) Validate() error {
	if c.BlueDefenseRange > c.RedDetectionRange {
		return fmt.Errorf("blueDefenseRange (%f) cannot exceed redDetectionRange (%f)",
			c.BlueDefenseRange, c.RedDetectionRange)
	}
	if c.RedAttackRange > c.BlueDefenseRange {
		return fmt.Errorf("redAttackRange (%f) should be ≤ blueDefenseRange (%f)",
			c.RedAttackRange, c.BlueDefenseRange)
	}
	if c.MinSpeed >= c.MaxSpeed {
		return fmt.Errorf("minSpeed (%f) must be < maxSpeed (%f)",
			c.MinSpeed, c.MaxSpeed)
	}
	return nil
}

// LoadConfig loads configuration from a JSON file and validates it against the schema.
func LoadConfig(configFile string, schemaFile string) (*Config, error) {
	// 1. Compile Schema
	sch, err := jsonschema.Compile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	// 2. Read Config File Bytes
	b, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 3. Unmarshal to interface{} for JSON Schema Validation
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("failed to decode config json for validation: %w", err)
	}

	if err := sch.Validate(v); err != nil {
		return nil, fmt.Errorf("config schema validation failed: %w", err)
	}

	// 4. Unmarshal to Config Struct
	// Since we already have the bytes in memory, this is fast.
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config to struct: %w", err)
	}

	// 5. Logical Validation (struct-level checks)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
