package actions

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/Eggbertx/territories-game/pkg/actions/turns"
	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
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
		const gameStartedQuery = `SELECT COUNT(*) FROM actions WHERE action_type NOT IN ('end_turn', 'join')`
		var numActionsTaken int
		var row *sql.Row
		if tx != nil {
			row = tx.QueryRow(gameStartedQuery)
		} else {
			db, err := db.GetDB()
			if err != nil {
				logger("Unable to get database connection", "error", err)
				return err
			}
			row = db.QueryRow(gameStartedQuery)
		}

		if err = row.Scan(&numActionsTaken); err != nil {
			logger("Unable to get number of actions taken", "error", err)
			return err
		}
		if numActionsTaken > 0 {
			// if this is reached, it can be assumed that a player was eliminated, putting the number of players below the minimum required,
			// return no error so the game can continue
			return nil
		}

		err = &ActionError{
			msg: fmt.Sprintf("not enough players to start the game, minimum required: %d, currently joined: %d", cfg.MinimumNationsToStart, numPlayers),
		}
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
				err = &ActionError{
					msg: fmt.Sprintf("no actions remaining for player %s", user),
				}
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
				err = &ActionError{
					msg: fmt.Sprintf("no actions remaining for player %s", user),
				}
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

// ActionError represents a non-critical error (e.g., not enough players to start, out of turns, invalid territory/action, etc)
type ActionError struct {
	msg string
	err error
}

func (e *ActionError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.msg
}

func (e *ActionError) Unwrap() error {
	return e.err
}
