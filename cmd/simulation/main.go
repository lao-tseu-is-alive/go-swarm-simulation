package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	stdLog "log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/simulation"
	"github.com/lao-tseu-is-alive/go-swarm-simulation/pkg/version"
	"github.com/tochemey/goakt/v3/actor"
	"github.com/tochemey/goakt/v3/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to file")
)

// ZapAdapter adapts zap.SugaredLogger to goakt.Logger interface
type ZapAdapter struct {
	*zap.SugaredLogger
}

// LogLevel returns the current log level of the underlying logger
// Note: zap doesn't easily expose the current level dynamically in a way that maps 1:1 to goakt's int level,
// but goakt uses this mainly for filtering. We can return a static value or map it if we had access to the atom.
// For now, we'll return a safe default since zap handles filtering internally.
func (z *ZapAdapter) LogLevel() log.Level {
	return log.InfoLevel // Placeholder, as zap handles its own filtering
}

func (z *ZapAdapter) LogOutput() []io.Writer {
	return []io.Writer{os.Stdout}
}

func (z *ZapAdapter) StdLogger() *stdLog.Logger {
	return stdLog.New(os.Stdout, "", stdLog.LstdFlags)
}

func main() {
	flag.Parse()

	// CPU Profiling
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			stdLog.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			stdLog.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	fmt.Printf("🚀 Starting App:'%s', ver:%s, BuildStamp: %s, Repo: %s\n", version.APP, version.VERSION, version.BuildStamp, version.REPOSITORY)

	ctx := context.Background()
	// Load Config
	cfg, err := simulation.LoadConfig("config.json", "config_schema.json")
	if err != nil {
		// Fallback to basic logging if config fails
		stdLog.Fatalf("Failed to load config: %v", err)
	}

	// 1. Configure Logger
	var logger *zap.Logger
	var zapCfg zap.Config

	if strings.ToLower(cfg.LogFormat) == "json" {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err = zapCfg.Build()
	if err != nil {
		stdLog.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Wrap in Adapter
	adapter := &ZapAdapter{SugaredLogger: logger.Sugar()}

	ebiten.SetWindowSize(int(cfg.WorldWidth), int(cfg.WorldHeight))
	ebiten.SetWindowTitle("Red Virus vs Blue Flock...Convert or Be Converted 🦠🚀") // suggested by Grok 4.1 🤣🔥

	// 2. Start Actor System with Custom Logger
	system, _ := actor.NewActorSystem("SwarmWorld",
		actor.WithLogger(adapter),
		actor.WithActorInitMaxRetries(3))
	_ = system.Start(ctx)

	// 5. Initialize Sliders (UI only) - Moved back to GetNewGame or handled there?
	// Wait, GetNewGame does everything. I just need to pass the system.

	// Actually, GetNewGame spawns the world too.
	// So I should just call GetNewGame(ctx, cfg, system)

	game := simulation.GetNewGame(ctx, cfg, system)
	defer game.System.Stop(ctx)
	err = ebiten.RunGame(game)
	if err != nil {
		stdLog.Fatal(err)
	}

	// Memory Profiling (written on exit)
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			stdLog.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // Run GC before taking heap profile
		if err := pprof.WriteHeapProfile(f); err != nil {
			stdLog.Fatal("could not write memory profile: ", err)
		}
	}
}
