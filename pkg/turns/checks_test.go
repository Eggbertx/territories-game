package turns

import (
	"database/sql"
	"testing"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/stretchr/testify/assert"
)

func setupTurnCheckDB(t *testing.T) *sql.DB {
	_, err := config.GetTestingConfig(t)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	tdb, err := db.GetDB()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	_, err = tdb.Exec(`INSERT INTO nations (country_name, player, color) VALUES
	('nation0', 'player0', '111'),
	('nation1', 'player1', '222'),
	('nation2', 'player2', '333')`)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	_, err = tdb.Exec(`INSERT INTO holdings (territory, nation_id, army_size) VALUES
	('ca', 1, 3),
	('nv', 2, 3),
	('ut', 3, 3)`)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	_, err = tdb.Exec(`INSERT INTO actions(nation_id, action_type, is_new_turn, timestamp) VALUES
	(1, 'join', 0, '2025-01-01 01:00:00'),
	(2, 'join', 0, '2025-01-01 02:00:00'),
	(3, 'join', 0, '2025-01-01 03:00:00')`)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return tdb
}

func doTestAreAllPlayersFinished(t *testing.T, withTx bool) {
	turnEndHandlers = nil
	var turnEnds int
	RegisterTurnEndHandler(func(_ time.Time, _ TurnEndReason) error {
		turnEnds++
		return nil
	})
	tdb := setupTurnCheckDB(t)
	defer db.CloseDB()
	var tx *sql.Tx
	if withTx {
		var err error
		tx, err = tdb.Begin()
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		defer tx.Rollback()
	}
	playersWithActions, err := PlayersWithActionsLeft(tx)
	if !assert.NoError(t, err, "Failed to get players with actions left") {
		t.FailNow()
	}
	// Initial validation.
	assert.Equal(t, 1, turnEnds)
	assert.Nil(t, playersWithActions, "Players should not have actions available immediately after joining")

	query := `INSERT INTO holdings(territory, nation_id, army_size) VALUES
		('wa', 3, 3),
		('or', 3, 3),
		('id', 3, 3)`
	if withTx {
		_, err = tx.Exec(query)
	} else {
		_, err = tdb.Exec(query)
	}
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	query = `INSERT INTO actions(nation_id, action_type, is_new_turn, timestamp) VALUES
	(NULL, 'turn', 1, '2025-01-01 04:00:00'),
	(1, 'move', 0, '2025-01-01 05:00:00'),
	(3, 'move', 0, '2025-01-01 06:00:00')`
	if withTx {
		_, err = tx.Exec(query)
	} else {
		_, err = tdb.Exec(query)
	}
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	playersWithActions, err = PlayersWithActionsLeft(tx)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.NotNil(t, playersWithActions, "Players should have actions available now")
	assert.Equal(t, 1, playersWithActions["player0"].MaxActions, "player0 should have 0 actions available")
	assert.Equal(t, 1, playersWithActions["player1"].MaxActions, "player1 should have 1 action available")
	assert.Equal(t, 2, playersWithActions["player2"].MaxActions, "player2 should have 2 actions available")
	assert.Equal(t, 1, playersWithActions["player2"].ActionsCompleted, "player2 should have completed 1 action")

	query = `INSERT INTO actions(nation_id, action_type, is_new_turn, timestamp) VALUES
	(2, 'move', 0, '2025-01-01 07:00:00'),
	(3, 'move', 0, '2025-01-01 08:00:00')`
	if withTx {
		_, err = tx.Exec(query)
	} else {
		_, err = tdb.Exec(query)
	}
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, 1, turnEnds)
}

func TestAreAllPlayersFinished(t *testing.T) {
	if !config.HasSQLiteMathFunctions {
		t.Skip("Skipping test because the sqlite_math_functions build tag is not enabled")
	}
	t.Run("with transaction", func(t *testing.T) {
		doTestAreAllPlayersFinished(t, true)
	})
	config.CloseTestingConfig(t)
	t.Run("without transaction", func(t *testing.T) {
		doTestAreAllPlayersFinished(t, false)
	})
}
