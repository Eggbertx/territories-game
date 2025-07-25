package actions

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/Eggbertx/territories-game/pkg/turns"
	"github.com/rs/zerolog"
)

func randInt(max int) int {
	if testing.Testing() && useTestInt {
		return int(math.Min(float64(testInt), float64(max))) - 1
	}
	return rand.Intn(max)
}

func checkIfEnoughPlayersToStart(tx *sql.Tx, cfg *config.Config, logger zerolog.Logger) error {
	if cfg == nil {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			logger.Err(err).Caller(1).Msg("Unable to get configuration")
			return err
		}
	}

	if cfg.MinimumNationsToStart < 2 {
		return nil
	}

	enough, numPlayers, err := db.EnoughPlayersToStart(tx)
	if err != nil {
		logger.Err(err).Caller(1).Msg("Unable to check if enough players are joined")
		return err
	}

	if !enough {
		err = fmt.Errorf("not enough players to start the game, minimum required: %d, currently joined: %d", cfg.MinimumNationsToStart, numPlayers)
		logger.Err(err).Caller(1).Send()
		return err
	}

	return nil
}

func checkReturnsRemainingIfManaging(tx *sql.Tx, user string, cfg *config.Config, logger zerolog.Logger) error {
	var err error
	if cfg == nil {
		cfg, err = config.GetConfig()
		if err != nil {
			logger.Err(err).Caller(1).Msg("Unable to get configuration")
			return err
		}
	}
	if cfg.DoTurnManagement {
		actionsRemaining, err := turns.PlayerActionsRemaining(user, tx)
		if err != nil {
			logger.Err(err).Caller(1).Msg("Unable to get player actions remaining")
			return err
		}
		if actionsRemaining < 1 {
			err = fmt.Errorf("no actions remaining for player %s", user)
			logger.Err(err).Caller(1).Send()
			return err
		}
	}

	return nil
}

func addTurnEntryIfManaging(tx *sql.Tx, user string, actionType string, cfg *config.Config, logger zerolog.Logger) error {
	if cfg == nil {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			logger.Err(err).Caller(1).Msg("Unable to get configuration")
			return err
		}
	}
	if !cfg.DoTurnManagement {
		if err := turns.AddPlayerActionEntry(tx, actionType, user, time.Now()); err != nil {
			logger.Err(err).Caller(1).Msg("Unable to add player action entry")
			return err
		}
	}
	return nil
}
