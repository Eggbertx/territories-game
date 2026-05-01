package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/Eggbertx/territories-game/pkg/actions"
	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/Eggbertx/territories-game/pkg/svgmap"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/term"
)

var (
	validActionTypes  = slog.AnyValue([]string{"join", "color", "raise", "move", "attack", "help", "-h"})
	logger            *slog.Logger
	runningInTerminal = term.IsTerminal(int(os.Stdin.Fd()))
)

type Config struct {
	config.Config
	LogFile string `json:"logFile"`
}

// resolveArgs is used for debugging purposes to allow passing arguments through environment variables
// instead of having to update launch.json for every possible action/argument combination.
func resolveArgs() []string {
	_, ok := os.LookupEnv("ARGS_FROM_ENV")
	if !ok {
		return os.Args[1:]
	}
	var args []string

	cmd, _ := os.LookupEnv("CMD")
	args = append(args, cmd)

	for i := 1; ; i++ {
		value, ok := os.LookupEnv(fmt.Sprintf("ARG%d", i))
		if !ok || value == "" {
			break
		}
		args = append(args, value)
	}

	return args
}

func main() {
	jsonOutput := !runningInTerminal
	if !jsonOutput {
		if slices.Contains(os.Args, "-json") {
			// assume that we probably want JSON since no action has this as an argument
			jsonOutput = true
		}
	}
	if jsonOutput {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}))
	}

	args := resolveArgs()

	cfg := Config{
		Config: config.DefaultConfig,
	}
	cfgFile, err := os.Open("config.json")
	if err != nil {
		logger.Error("Unable to open file", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := cfgFile.Close(); err != nil {
			logger.Error("Unable to close file", "error", err)
		}
	}()
	if err = json.NewDecoder(cfgFile).Decode(&cfg); err != nil {
		logger.Error("Unable to decode config", "error", err)
		os.Exit(1)
	}
	cfg.LogInfo = func(msg string, args ...interface{}) {
		logger.Info(msg, args...)
	}
	cfg.LogError = func(msg string, args ...interface{}) {
		logger.Error(msg, args...)
	}
	config.SetConfig(&cfg.Config)

	if len(args) == 0 {
		logger.Error("No action specified", "validActions", validActionTypes)
		os.Exit(1)
	}
	if cfg.LogFile != "" {
		// write to both stdout and the log file
		logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Unable to open log file", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := logFile.Close(); err != nil {
				logger.Error("Unable to close log file", "error", err)
			}
		}()
		fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
			AddSource: true,
		})
		logger = slog.New(slog.NewMultiHandler(logger.Handler(), fileHandler))
	}

	actionType := args[0]

	var user string
	var armies int
	var action actions.Action
	switch actionType {
	case "join":
		var nation string
		var territory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is joining the game")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&nation, "nation", "", "the name of the nation the user is joining")
		flagSet.StringVar(&territory, "territory", "", "the territory the user is joining")
		flagSet.Parse(args[1:])
		action = &actions.JoinAction{
			User:      user,
			Nation:    nation,
			Territory: territory,
		}
	case "color":
		var color string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is changing their color")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&color, "color", "", "the new color for the user")
		flagSet.Parse(args[1:])
		action = &actions.ColorAction{
			User:  user,
			Color: color,
		}
	case "raise":
		var territory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is raising armies")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&territory, "territory", "", "the territory where the user is raising the army size")
		flagSet.Parse(args[1:])
		action = &actions.RaiseAction{
			User:      user,
			Territory: territory,
		}
	case "move":
		var sourceTerritory string
		var destinationTerritory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is moving armies")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.IntVar(&armies, "armies", 0, "the number of armies to move")
		flagSet.StringVar(&sourceTerritory, "source", "", "the territory from which the user is moving armies")
		flagSet.StringVar(&destinationTerritory, "destination", "", "the territory to which the user is moving armies")
		flagSet.Parse(args[1:])
		action = &actions.MoveAction{
			User:        user,
			Armies:      armies,
			Source:      sourceTerritory,
			Destination: destinationTerritory,
		}
	case "attack":
		var attackingTerritory string
		var defendingTerritory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is attacking")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&attackingTerritory, "attacking", "", "the territory from which the user is attacking")
		flagSet.StringVar(&defendingTerritory, "defending", "", "the territory that is being attacked")
		flagSet.Parse(args[1:])
		action = &actions.AttackAction{
			User:               user,
			AttackingTerritory: attackingTerritory,
			DefendingTerritory: defendingTerritory,
		}
	case "help", "-h":
		logger.Info(fmt.Sprintf("usage: %s <action> [args...]", os.Args[0]), "validActions", validActionTypes)
		os.Exit(0)
	default:
		logger.Error("Invalid command", "validActions", validActionTypes)
	}

	if actionType == "" || user == "" {
		logger.Error("user must be specified")
		os.Exit(1)
	}

	db, err := db.GetDB()
	if err != nil {
		logger.Error("Unable to initialize database", "error", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Unable to close database", "error", err)
		}
	}()

	actionResult, err := action.DoAction(db)
	if err != nil {
		// assume that any error returned from DoAction is already logged
		os.Exit(1)
	}

	resultMsg := actionResult.String()
	switch result := actionResult.(type) {
	case *actions.JoinActionResult:
		action := *result.Action
		logger.Info(resultMsg, "nation", action.Nation, "territory", action.Territory)
	case *actions.ColorActionResult:
		action := *result.Action
		logger.Info(resultMsg, "color", action.Color)
	case *actions.RaiseActionResult:
		action := *result.Action
		logger.Info(resultMsg, "territory", action.Territory)
	case *actions.MoveActionResult:
		action := *result.Action
		logger.Info(resultMsg, "source", action.Source, "destination", action.Destination)
	case *actions.AttackActionResult:
		action := *result.Action
		logger.Info(resultMsg, "attacking", action.AttackingTerritory, "defending", action.DefendingTerritory)
	default:
		logger.Error("Unknown action result", "actionType", actionResult.ActionType())
	}

	if err = svgmap.ApplyDBEvents(); err != nil {
		logger.Error("Unable to apply database events to map", "error", err)
		os.Exit(1)
	}
}
