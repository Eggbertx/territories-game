package turns

import (
	"database/sql"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

const (
	TurnEndReasonTimeLimit TurnEndReason = iota
	TurnEndReasonPlayersAllDone
)

type TurnEndReason int

// PlayerActions represents the actions taken and maximum actions (based on holdings) a player can take in a turn.
type PlayerActions struct {
	ActionsCompleted int
	MaxActions       int
}

// PlayersWithActionsLeft returns a map of player names to PlayerActions for all players that still have actions available in the current turns.
// If it is nil, all players have completed their actions. If all players are done and the configuration allows it, it will end the turn.
func PlayersWithActionsLeft(tx *sql.Tx) (map[string]PlayerActions, error) {
	const query = `SELECT q1.player, COALESCE(actions, 0) AS actions_completed, max_actions
	FROM (
		SELECT player, nation_id,
			CEIL(COUNT(*) / ?) AS max_actions
		FROM v_nation_holdings
		GROUP BY player, nation_id
	) q1 LEFT JOIN (
		SELECT player, COUNT(*) AS actions
		FROM v_actions
		WHERE player IS NOT NULL
		AND timestamp >= (SELECT COALESCE(MAX(timestamp),'0001-01-01') FROM v_new_turn_actions)
		GROUP BY player
	) q2 ON q1.player = q2.player
	WHERE COALESCE(q2.actions, 0) < q1.max_actions`

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

	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(cfg.ActionsPerTurnHoldingsDivisor)
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

	if len(playerActions) == 0 && cfg.TurnEndsWhenAllPlayersDone {
		// all players are done, configuration set to end turn when all players are done
		if err = EndTurn(TurnEndReasonPlayersAllDone, tx); err != nil {
			return playerActions, err
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

	db, err := db.GetDB()
	if err != nil {
		return false, err
	}
	shouldCommit := tx == nil
	if shouldCommit {
		tx, err = db.Begin()
		if err != nil {
			return false, err
		}
		defer tx.Rollback()
	}

	var lastTurnEndTime sql.NullTime
	err = tx.QueryRow("SELECT MAX(timestamp) FROM v_new_turn_actions").Scan(&lastTurnEndTime)
	if err != nil {
		return false, err
	}

	if !lastTurnEndTime.Valid {
		return false, nil // No previous turn end time found
	}

	expired := lastTurnEndTime.Time.Add(turnDuration).Before(time.Now())
	if expired && cfg.TurnEndsWhenAllPlayersDone {
		err = EndTurn(TurnEndReasonTimeLimit, tx)
		if err != nil {
			return false, err
		}
	}

	return expired, nil
}
