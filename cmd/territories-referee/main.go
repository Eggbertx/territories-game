package main

import (
	"flag"
	"os"
	"slices"

	"github.com/Eggbertx/territories-game/pkg/actions"
	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/Eggbertx/territories-game/pkg/svgmap"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

var (
	logger       zerolog.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	validActions                = []string{"join", "color", "raise", "move", "attack"}
)

func runningInTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

func main() {
	var event actions.GameEvent
	var jsonOutput bool

	err := config.InitLogger(false)
	if err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}
	logger, err = config.GetLogger()
	if err != nil {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		logger.Fatal().Err(err).Caller().Send()
	}

	if len(os.Args) < 2 {
		logger.Fatal().Msg("Usage: territories-referee join|color|raise|move|attack -user <user> -subject <subject> -predicate <predicate>")
		os.Exit(1)
	}
	if !slices.Contains(validActions, os.Args[1]) {
		logger.Fatal().Msgf("Invalid action '%s', valid actions are: %v", os.Args[1], validActions)
		os.Exit(1)
	}

	event.Action = os.Args[1]
	flagSet := flag.NewFlagSet("territories-referee", flag.ExitOnError)
	flagSet.BoolVar(&jsonOutput, "json", false, "output in JSON format, default is colorized text (for console)")
	flagSet.StringVar(&event.Subject, "subject", "", "the subject of the action")
	flagSet.StringVar(&event.Predicate, "predicate", "", "the target that the subject is going to be joining, moving to, attacking, etc")
	flagSet.StringVar(&event.User, "user", "", "the user that is making the action")
	flagSet.Parse(os.Args[2:])

	if err = config.InitLogger(jsonOutput); err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}

	logger, err = config.GetLogger()
	if err != nil {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		logger.Fatal().Err(err).Caller().Send()
	}

	if event.Action == "" || event.User == "" {
		logger.Fatal().Msg("user must be specified")
	}

	db, err := db.GetDB()
	if err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Fatal().Err(err).Caller().Send()
		}
	}()

	if event.DoAction(db) != nil {
		os.Exit(1)
	}

	if err = svgmap.ApplyDBEvents(); err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}
}
