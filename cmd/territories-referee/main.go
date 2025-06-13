package main

import (
	"flag"
	"os"
	"slices"
	"strconv"

	"github.com/Eggbertx/territories-game/pkg/actions"
	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/Eggbertx/territories-game/pkg/svgmap"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

var (
	validActions []string

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
	validActions := actions.RegisteredActionParsers()
	if !slices.Contains(validActions, os.Args[1]) {
		logger.Fatal().Msgf("Invalid action '%s', valid actions are: %v", os.Args[1], validActions)
	}

	actionType := os.Args[1]

	var user string
	var actionArgs []string
	var armies int

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
		actionArgs = []string{user, nation, territory}
	case "color":
		var color string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is changing their color")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&color, "color", "", "the new color for the user")
		flagSet.Parse(os.Args[2:])
		actionArgs = []string{user, color}
	case "raise":
		var territory string
		flagSet := flag.NewFlagSet("", flag.ExitOnError)
		flagSet.StringVar(&user, "user", "", "the user that is raising armies")
		flagSet.BoolVar(&jsonOutput, "json", false, "log output in JSON format")
		flagSet.StringVar(&territory, "territory", "", "the territory where the user is raising the army size")
		flagSet.Parse(os.Args[2:])
		actionArgs = []string{user, territory}
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
		if armies > 0 {
			actionArgs = []string{user, strconv.Itoa(armies), sourceTerritory, destinationTerritory}
		} else {
			actionArgs = []string{user, sourceTerritory, destinationTerritory}
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
		actionArgs = []string{user, attackingTerritory, defendingTerritory}
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

	fatalEv := logger.Fatal()
	defer fatalEv.Discard()
	fatalEv.Str("action", actionType)
	fatalEv.Str("user", user)

	if actionType == "" || user == "" {
		fatalEv.Msg("user must be specified")
	}

	// if actionType == "move" && armies > 0 {
	// 	actionType = fmt.Sprintf("%s%d", actionType, armies)
	// }

	db, err := db.GetDB()
	if err != nil {
		fatalEv.Err(err).Caller().Send()
	}

	defer func() {
		if err := db.Close(); err != nil {
			fatalEv.Err(err).Caller().Send()
		}
	}()

	actionParser, err := actions.GetActionParser(actionType)
	if err != nil {
		fatalEv.Err(err).Caller().Msg("Unable to get action parser")
	}

	action, err := actionParser(actionArgs...)
	if err != nil {
		fatalEv.Err(err).Caller().Msgf("Unable to parse action parameters")
	}

	if action.DoAction(db) != nil {
		os.Exit(1)
	}

	if err = svgmap.ApplyDBEvents(); err != nil {
		fatalEv.Err(err).Caller().Send()
	}
}
