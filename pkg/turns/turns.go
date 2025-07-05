package turns

import (
	"database/sql"
	"math"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

var (
	turnEndHandlers []func(time.Time, TurnEndReason) error
)

// RegisterTurnEndHandler registers a function to be called when a turn ends, passing to it the timestamp
// and the reason for the turn ending.
func RegisterTurnEndHandler(handler func(time.Time, TurnEndReason) error) {
	turnEndHandlers = append(turnEndHandlers, handler)
}

// CurrentTurnStarted returns the timestamp of the current turn's start time and whether the current turn is the first turn
func CurrentTurnStarted() (time.Time, bool, error) {
	var turnTimestamp time.Time
	db, err := db.GetDB()
	if err != nil {
		return turnTimestamp, false, err
	}
	stmt, err := db.Prepare("SELECT MAX(timestamp) FROM v_new_turn_actions")
	if err != nil {
		return turnTimestamp, false, err
	}
	defer stmt.Close()
	if err = stmt.QueryRow().Scan(&turnTimestamp); err != nil {
		return turnTimestamp, false, err
	}
	if err = stmt.Close(); err != nil {
		return turnTimestamp, false, err
	}

	firstTurn := turnTimestamp.IsZero()
	if firstTurn {
		// still on the first turn, get the first action and use its timestamp
		stmt, err = db.Prepare("SELECT MIN(timestamp) FROM actions")
		if err != nil {
			return turnTimestamp, firstTurn, err
		}
		defer stmt.Close()
		if err = stmt.QueryRow().Scan(&turnTimestamp); err != nil {
			return turnTimestamp, firstTurn, err
		}
	}

	return turnTimestamp, firstTurn, nil
}

// PlayerActionsPerTurn calculates the number of actions a player can take per turn based on their holdings and the configured divisor.
// If the player does not have any holdings, it returns 0.
func PlayerActionsPerTurn(player string, tx *sql.Tx) (int, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return 0, err
	}
	divisor := cfg.ActionsPerTurnHoldingsDivisor
	if divisor <= 0 {
		cfg.ActionsPerTurnHoldingsDivisor = 3
		divisor = 3
	}
	var holdings int
	db, err := db.GetDB()
	if err != nil {
		return 0, err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
	}

	stmt, err := tx.Prepare("SELECT COUNT(*) FROM v_nation_holdings WHERE player = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	if err = stmt.QueryRow(player).Scan(&holdings); err != nil {
		return 0, err
	}
	if err = stmt.Close(); err != nil {
		return 0, err
	}
	if shouldCommit {
		if err = tx.Commit(); err != nil {
			return 0, err
		}
	}
	return int(math.Ceil(float64(holdings) / divisor)), nil
}

// PlayerActionsRemaining returns the number of actions a player can still take in the current turn.
func PlayerActionsRemaining(player string, tx *sql.Tx) (int, error) {
	db, err := db.GetDB()
	if err != nil {
		return 0, err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
	}

	totalTurns, err := PlayerActionsPerTurn(player, tx)
	if err != nil {
		return 0, err
	}
	if totalTurns == 0 {
		return 0, nil // No actions available if total turns is 0
	}

	var actionsTaken int
	stmt, err := tx.Prepare("SELECT COUNT(*) FROM v_actions WHERE timestamp > (SELECT MAX(timestamp) FROM v_new_turn_actions) AND player = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	if err = stmt.QueryRow(player).Scan(&actionsTaken); err != nil {
		return 0, err
	}

	return int(math.Min(float64(totalTurns-actionsTaken), 0)), nil
}

// EndTurn ends the current turn, inserting a new action with is_new_turn set to true, and calling all registered turn end handlers.
// This is mainly used by the game when all players have used their available actions or the time limit has been reached
func EndTurn(reason TurnEndReason, tx *sql.Tx) error {
	db, err := db.GetDB()
	if err != nil {
		return err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	stmt, err := tx.Prepare("INSERT INTO actions (action_type, timestamp, is_new_turn) VALUES ('end_turn', ?, 1)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	if _, err = stmt.Exec(now); err != nil {
		return err
	}

	for _, handler := range turnEndHandlers {
		if err = handler(now, reason); err != nil {
			return err
		}
	}

	if shouldCommit {
		return tx.Commit()
	}
	return nil
}
