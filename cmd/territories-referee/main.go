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
	logger   zerolog.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	usageStr                = "Usage: territories-referee join|color|raise|move|attack|help -user <user> -json [...]"
)

func usage(jsonOut bool, fatal bool) {
	err := config.InitLogger(jsonOut)
	if err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}
	logger, err = config.GetLogger()
	if err != nil {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		logger.Fatal().Err(err).Caller().Send()
	}
	var ev *zerolog.Event
	if fatal {
		ev = logger.Fatal()
	} else {
		ev = logger.Info()
	}
	ev.Msg(usageStr)
}

func main() {
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
		usage(false, true)
	}

	actionType := os.Args[1]

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
		flagSet.Parse(os.Args[2:])
		action = &actions.JoinAction{
			User:      user,
			Nation:    nation,
			Territory: territory,
			Logger:    logger,
		}
	case "color":
		var color string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is changing their color")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&color, "color", "", "the new color for the user")
		flagSet.Parse(os.Args[2:])
		action = &actions.ColorAction{
			User:   user,
			Color:  color,
			Logger: logger,
		}
	case "raise":
		var territory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is raising armies")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&territory, "territory", "", "the territory where the user is raising the army size")
		flagSet.Parse(os.Args[2:])
		action = &actions.RaiseAction{
			User:      user,
			Territory: territory,
			Logger:    logger,
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
		flagSet.Parse(os.Args[2:])
		action = &actions.MoveAction{
			User:        user,
			Armies:      armies,
			Source:      sourceTerritory,
			Destination: destinationTerritory,
			Logger:      logger,
		}
	case "attack":
		var attackingTerritory string
		var defendingTerritory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is attacking")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&attackingTerritory, "attacking", "", "the territory from which the user is attacking")
		flagSet.StringVar(&defendingTerritory, "defending", "", "the territory that is being attacked")
		flagSet.Parse(os.Args[2:])
		action = &actions.AttackAction{
			User:               user,
			AttackingTerritory: attackingTerritory,
			DefendingTerritory: defendingTerritory,
			Logger:             logger,
		}
	case "help", "-h":
		usage(len(os.Args) > 2 && os.Args[2] == "-json", false)
		os.Exit(0)
	default:
		usage(false, true)
	}

	if err = config.InitLogger(jsonOutput); err != nil {
		logger.Fatal().Err(err).Caller().Send()
	}

	logger, err = config.GetLogger()
	if err != nil {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
		logger.Fatal().Err(err).Caller().Send()
	}
	logger = logger.With().Str("action", actionType).Str("user", user).Logger()
	config.SetLogger(logger)

	fatalEv := logger.Fatal()
	defer fatalEv.Discard()
	fatalEv.Str("action", actionType)
	fatalEv.Str("user", user)

	if actionType == "" || user == "" {
		fatalEv.Msg("user must be specified")
	}

	db, err := db.GetDB()
	if err != nil {
		fatalEv.Err(err).Caller().Send()
	}

	defer func() {
		if err := db.Close(); err != nil {
			fatalEv.Err(err).Caller().Send()
		}
	}()

	actionResult, err := action.DoAction(db)
	if err != nil {
		os.Exit(1)
	}
	infoEv := logger.Info()
	defer infoEv.Discard()
	switch result := actionResult.(type) {
	case *actions.JoinActionResult:
		action := *result.Action
		infoEv.
			Str("nation", action.Nation).
			Str("territory", action.Territory)
	case *actions.ColorActionResult:
		action := *result.Action
		infoEv.Str("color", action.Color)
	case *actions.RaiseActionResult:
		action := *result.Action
		infoEv.Str("territory", action.Territory)
	case *actions.MoveActionResult:
		action := *result.Action
		infoEv.
			Str("source", action.Source).
			Str("destination", action.Destination).
			Int("armies", action.Armies)
	case *actions.AttackActionResult:
		action := *result.Action
		infoEv.
			Str("attacking", action.AttackingTerritory).
			Str("defending", action.DefendingTerritory)
	default:
		fatalEv.Str("actionType", actionResult.ActionType()).Msg("unknown action result")
	}
	infoEv.Msg(actionResult.String())

	if err = svgmap.ApplyDBEvents(); err != nil {
		fatalEv.Err(err).Caller().Send()
	}
	logger.Info().Msg("Map updated")
}
