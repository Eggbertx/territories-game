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
)

func randInt(max int) int {
	if testing.Testing() && useTestInt {
		return int(math.Min(float64(testInt), float64(max))) - 1
	}
	return rand.Intn(max)
}

func checkIfEnoughPlayersToStart(tx *sql.Tx, cfg *config.Config, logger config.LoggerFunc) error {
	if cfg == nil {
		var err error
		cfg, err = config.GetConfig()
		if err != nil {
			logger("Unable to get configuration", "error", err)
			return err
		}
	}

	if cfg.MinimumNationsToStart < 2 {
		return nil
	}

	enough, numPlayers, err := db.EnoughPlayersToStart(tx)
	if err != nil {
		logger("Unable to check if enough players are joined", "error", err)
		return err
	}

	if !enough {
		err = fmt.Errorf("not enough players to start the game, minimum required: %d, currently joined: %d", cfg.MinimumNationsToStart, numPlayers)
		logger("Not enough players to start the game", "minimumRequired", cfg.MinimumNationsToStart, "currentlyJoined", numPlayers, "error", err)
		return err
	}

	return nil
}

func checkReturnsRemainingIfManaging(tx *sql.Tx, user string, cfg *config.Config, logger config.LoggerFunc) error {
	var err error
	if cfg == nil {
		cfg, err = config.GetConfig()
		if err != nil {
			logger("Unable to get configuration", "error", err)
			return err
		}
	}
	if cfg.DoTurnManagement {
		actionsRemaining, err := turns.PlayerActionsRemaining(user, tx)
		if err != nil {
			logger("Unable to get player actions remaining", "error", err)
			return err
		}
		if actionsRemaining < 1 {
			if cfg.TurnDuration <= 0 {
				err = fmt.Errorf("no actions remaining for player %s", user)
				logger("Out of actions", "player", user, "error", err)
				return err
			}

			// check if turn duration has expired
			shouldEndTurn, err := turns.HasTurnDurationExpired(tx)
			if err != nil {
				logger("Unable to check if turn duration has expired", "error", err)
				return err
			}
			if !shouldEndTurn {
				err = fmt.Errorf("no actions remaining for player %s", user)
				logger("Out of actions", "player", user, "error", err)
				return err
			}
		}
	}

	return nil
}

func addTurnEntryIfManaging(tx *sql.Tx, user string, actionType string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}
	if cfg.DoTurnManagement {
		if err := turns.AddPlayerActionEntry(tx, actionType, user, time.Now()); err != nil {
			cfg.LogError("Unable to add player action entry", "error", err)
			return err
		}
	}
	return nil
}
