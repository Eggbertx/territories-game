package turns

import (
	"database/sql"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

const (
	TurnEndReasonUnknown TurnEndReason = iota
	TurnEndReasonTimeLimit
	TurnEndReasonPlayersAllDone
)

type TurnEndReason int

// PlayerActions represents the actions taken and maximum actions (based on holdings) a player can take in a turn.
type PlayerActions struct {
	ActionsCompleted int
	MaxActions       int
}

func queryPlayersWithActionsLeft(tx *sql.Tx, actionsPerTurnHoldingsDivisor float64) (map[string]PlayerActions, error) {
	const query = `SELECT q1.player, coalesce(actions_completed, 0) as actions_completed, max_actions
	FROM (
		SELECT player, nation_id,
			CEIL(COUNT(*) / ?) AS max_actions
		FROM v_nation_holdings
		GROUP BY player, nation_id
	) q1 LEFT JOIN v_current_turn_player_actions q2 ON q1.player = q2.player
	WHERE COALESCE(q2.actions_completed, 0) < q1.max_actions`

	db, err := db.GetDB()
	if err != nil {
		return nil, err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
	}

	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(actionsPerTurnHoldingsDivisor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playerActions map[string]PlayerActions
	for rows.Next() {
		if playerActions == nil {
			playerActions = make(map[string]PlayerActions)
		}
		var player string
		var actionInfo PlayerActions
		if err := rows.Scan(&player, &actionInfo.ActionsCompleted, &actionInfo.MaxActions); err != nil {
			return nil, err
		}
		playerActions[player] = actionInfo
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}

	if shouldCommit {
		if err = tx.Commit(); err != nil {
			return playerActions, err
		}
	}
	return playerActions, nil
}

// PlayersWithActionsLeft returns a map of player names to PlayerActions for all players that still have actions available in the current turns.
// If all players are done and the configuration allows it, it will end the turn.
func PlayersWithActionsLeft(tx *sql.Tx) (map[string]PlayerActions, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	db, err := db.GetDB()
	if err != nil {
		return nil, err
	}

	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
	}

	playerActions, err := queryPlayersWithActionsLeft(tx, cfg.ActionsPerTurnHoldingsDivisor)
	if err != nil {
		return nil, err
	}

	if len(playerActions) == 0 && cfg.TurnEndsWhenAllPlayersDone {
		// all players are done, configuration set to end turn when all players are done
		if err = EndTurn(TurnEndReasonPlayersAllDone, tx); err != nil {
			return playerActions, err
		}

		// re-query to get updated player actions after turn end
		playerActions, err = queryPlayersWithActionsLeft(tx, cfg.ActionsPerTurnHoldingsDivisor)
		if err != nil {
			return nil, err
		}
	}
	if shouldCommit {
		if err = tx.Commit(); err != nil {
			return playerActions, err
		}
	}

	return playerActions, nil
}

// HasTurnDurationExpired returns true if the turn duration has expired based on the last action timestamp.
// if turnDuration is empty or unset, it always returns false (no time limit)
func HasTurnDurationExpired(tx *sql.Tx) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return false, err
	}

	turnDuration := cfg.TurnDuration()
	if turnDuration <= 0 {
		return false, nil // turns have no time limit if turnDuration is unset or empty
	}

	tdb, err := db.GetDB()
	if err != nil {
		return false, err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = tdb.Begin()
		if err != nil {
			return false, err
		}
		defer tx.Rollback()
	}

	var lastTurnEndTimestamp db.SQLite3Timestamp
	var actionCount int
	err = tx.QueryRow("SELECT MAX(timestamp), COUNT(*) FROM v_new_turn_actions").Scan(&lastTurnEndTimestamp, &actionCount)
	if err != nil {
		return false, err
	}

	if !lastTurnEndTimestamp.Valid || actionCount == 0 {
		// return false, nil // No previous turn end time found
		if err = tx.QueryRow("SELECT MIN(timestamp) FROM actions").Scan(&lastTurnEndTimestamp); err != nil {
			return false, err
		}
		if !lastTurnEndTimestamp.Valid {
			return false, nil
		}
	}

	expired := lastTurnEndTimestamp.Time.Add(turnDuration).Before(time.Now())
	if !expired {
		return false, nil
	}
	if err = EndTurn(TurnEndReasonTimeLimit, tx); err != nil {
		return false, err
	}
	return true, nil
}

// IsTurnDone checks if the turn is done based on the configuration and player actions. If all players are
// done or the turn duration has expired, it will insert a turn end entry and return true.
func IsTurnDone(tx *sql.Tx) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return false, err
	}
	var shouldEndTurn bool
	if cfg.TurnEndsWhenAllPlayersDone {
		playerActions, err := PlayersWithActionsLeft(tx)
		if err != nil {
			return false, err
		}
		shouldEndTurn = len(playerActions) == 0
	}
	if !shouldEndTurn && cfg.TurnDuration() > 0 {
		if shouldEndTurn, err = HasTurnDurationExpired(tx); err != nil {
			return false, err
		}
	}

	return shouldEndTurn, nil
}
