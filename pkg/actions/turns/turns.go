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
	// var turnTimestampStr sql.NullString
	var turnTimestamp db.SQLite3Timestamp
	tdb, err := db.GetDB()
	if err != nil {
		return turnTimestamp.Time, false, err
	}
	stmt, err := tdb.Prepare("SELECT MAX(timestamp) FROM v_new_turn_actions")
	if err != nil {
		return turnTimestamp.Time, false, err
	}
	defer stmt.Close()
	if err = stmt.QueryRow().Scan(&turnTimestamp); err != nil {
		return turnTimestamp.Time, false, err
	}
	if err = stmt.Close(); err != nil {
		return turnTimestamp.Time, false, err
	}

	firstTurn := turnTimestamp.Time.IsZero()
	if firstTurn {
		// still on the first turn, get the first action and use its timestamp
		stmt, err = tdb.Prepare("SELECT MIN(timestamp) FROM actions")
		if err != nil {
			return turnTimestamp.Time, firstTurn, err
		}
		defer stmt.Close()
		if err = stmt.QueryRow().Scan(&turnTimestamp); err != nil {
			return turnTimestamp.Time, firstTurn, err
		}
		if err = stmt.Close(); err != nil {
			return turnTimestamp.Time, firstTurn, err
		}
	}

	return turnTimestamp.Time, firstTurn, nil
}

// MaxPlayerActionsPerTurn calculates the number of actions a player can take per turn based on their holdings and the configured divisor.
// If the player does not have any holdings, it returns 0.
func MaxPlayerActionsPerTurn(player string, tx *sql.Tx) (int, error) {
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
	playersWithActions, err := PlayersWithActionsLeft(tx)
	if err != nil {
		return 0, err
	}
	if actionInfo, ok := playersWithActions[player]; ok {
		return actionInfo.MaxActions - actionInfo.ActionsCompleted, nil
	}
	return 0, nil
}

// EndTurn ends the current turn, inserting a new action with is_new_turn set to true, and calling all registered turn end handlers.
// This is mainly used by the game when all players have used their available actions or the time limit has been reached
func EndTurn(reason TurnEndReason, tx *sql.Tx) error {
	now := time.Now()
	err := AddTurnEndActionEntry(now, tx)
	if err != nil {
		return err
	}

	for _, handler := range turnEndHandlers {
		if err = handler(now, reason); err != nil {
			return err
		}
	}

	return nil
}

func addActionEntry(tx *sql.Tx, actionType string, player string, timestamp time.Time) error {
	shouldCommit := tx == nil
	if shouldCommit {
		db, err := db.GetDB()
		if err != nil {
			return err
		}
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	var stmt *sql.Stmt
	var err error
	var args []any
	if player == "" {
		stmt, err = tx.Prepare("INSERT INTO actions (action_type, timestamp, is_new_turn) VALUES ('end_turn', ?, 1)")
		args = []any{timestamp}
	} else {
		stmt, err = tx.Prepare(`INSERT INTO actions (action_type, nation_id, timestamp)
			VALUES (?, (SELECT id FROM nations WHERE player = ?), ?)`)
		args = []any{actionType, player, timestamp}
	}
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(args...); err != nil {
		return err
	}

	if _, err = HasTurnDurationExpired(tx); err != nil {
		return err
	}

	if shouldCommit {
		return tx.Commit()
	}
	return nil
}

// AddPlayerActionEntry adds a new row in the actions table representing a turn action taken by a player.
// It is assumed that this will be run at the end of an action handler function, after all necessary checks
// have been made
func AddPlayerActionEntry(tx *sql.Tx, actionType string, player string, timestamp time.Time) error {
	return addActionEntry(tx, actionType, player, timestamp)
}

// AddTurnEndActionEntry adds a new row in the actions table representing the end of a turn.
func AddTurnEndActionEntry(timestamp time.Time, tx *sql.Tx) error {
	return addActionEntry(tx, "end_turn", "", timestamp)
}
