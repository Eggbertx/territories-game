package main

import (
	"flag"
	"os"

	"github.com/Eggbertx/territories-game/pkg/actions"
	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/Eggbertx/territories-game/pkg/svgmap"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

var (
	logger zerolog.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
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
	flag.BoolVar(&jsonOutput, "json", false, "output in JSON format, default is colorized text (for console)")
	flag.StringVar(&event.Action, "action", "", "the action to be taken, must be join, move, or attack")
	flag.StringVar(&event.Subject, "subject", "", "the subject of the action")
	flag.StringVar(&event.Predicate, "predicate", "", "the target that the subject is going to be joining, moving to, attacking, etc")
	flag.StringVar(&event.User, "user", "", "the user that is making the action")
	flag.Parse()

	err := config.InitLogger(jsonOutput)
	if err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}

	logger, err = config.GetLogger()
	if err != nil {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		logger.Fatal().Err(err).Caller().Send()
	}

	if event.Action == "" || event.User == "" {
		logger.Fatal().Msg("action and user must be specified")
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
