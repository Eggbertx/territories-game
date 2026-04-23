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
	('CA', 1, 3),
	('NV', 2, 3),
	('UT', 3, 3)`)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	if !assert.NoError(t, AddPlayerActionEntry(nil, "join", "player0", time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}
	if !assert.NoError(t, AddPlayerActionEntry(nil, "join", "player1", time.Date(2025, 1, 1, 2, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}
	if !assert.NoError(t, AddPlayerActionEntry(nil, "join", "player2", time.Date(2025, 1, 1, 3, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}

	return tdb
}

func doTestAreAllPlayersFinished(t *testing.T, withTx bool) {
	turnEndHandlers = nil
	var turnEnds int
	var turnEndReason TurnEndReason
	RegisterTurnEndHandler(func(_ time.Time, reason TurnEndReason) error {
		turnEndReason = reason
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
	if !assert.Equal(t, map[string]PlayerActions{
		"player0": {ActionsCompleted: 0, MaxActions: 1},
		"player1": {ActionsCompleted: 0, MaxActions: 1},
		"player2": {ActionsCompleted: 0, MaxActions: 1},
	}, playersWithActions, "Players should not have actions available immediately after joining") {
		t.FailNow()
	}

	// add holdings to player2 so that they should have 2 actions per-turn
	query := `INSERT INTO holdings(territory, nation_id, army_size) VALUES
		('WA', 3, 3),
		('OR', 3, 3),
		('ID', 3, 3)`
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
	if !assert.Equal(t, map[string]PlayerActions{
		"player0": {ActionsCompleted: 0, MaxActions: 1},
		"player1": {ActionsCompleted: 0, MaxActions: 1},
		"player2": {ActionsCompleted: 0, MaxActions: 2},
	}, playersWithActions, "Players should have updated actions available after holdings change") {
		t.FailNow()
	}

	if !assert.NoError(t, AddPlayerActionEntry(tx, "move", "player0", time.Date(2025, 1, 1, 5, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}
	playersWithActions, err = PlayersWithActionsLeft(tx)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.Len(t, playersWithActions, 2) {
		t.FailNow()
	}

	if !assert.NoError(t, AddPlayerActionEntry(tx, "move", "player1", time.Date(2025, 1, 1, 6, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}
	if !assert.NoError(t, AddPlayerActionEntry(tx, "move", "player2", time.Date(2025, 1, 1, 7, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}

	playersWithActions, err = PlayersWithActionsLeft(tx)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.NotNil(t, playersWithActions, "Players should have actions available now")
	// assert.Equal(t, 1, playersWithActions["player0"].MaxActions, "player0 should have 0 actions per-turn")
	// assert.Equal(t, 1, playersWithActions["player1"].MaxActions, "player1 should have 1 actions per-turn")
	// assert.Equal(t, 2, playersWithActions["player2"].MaxActions, "player2 should have 2 actions per-turn")
	assert.Equal(t, 1, playersWithActions["player2"].ActionsCompleted, "player2 should have completed 1 action")

	if !assert.NoError(t, AddTurnEndActionEntry(time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC), tx)) {
		t.FailNow()
	}
	if !assert.NoError(t, AddPlayerActionEntry(tx, "move", "player0", time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}
	if !assert.NoError(t, AddPlayerActionEntry(tx, "move", "player1", time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC))) {
		t.FailNow()
	}

	if withTx && !assert.NoError(t, tx.Commit()) {
		t.FailNow()
	}

	assert.Equal(t, 1, turnEnds)
	assert.Equal(t, TurnEndReasonPlayersAllDone, turnEndReason, "Turn should end because all players have completed their actions")
}

func TestAreAllPlayersFinished(t *testing.T) {
	if !config.HasSQLiteMathFunctions {
		t.Skip("Skipping test because the sqlite_math_functions build tag is not enabled")
	}

	cfg, err := config.GetTestingConfig(t)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	cfg.TurnDurationString = "" // disable turn duration to prevent time spent stepping through code from causing turns to end
	config.SetConfig(cfg)

	t.Run("with transaction", func(t *testing.T) {
		doTestAreAllPlayersFinished(t, true)
	})
	config.CloseTestingConfig(t)
	t.Run("without transaction", func(t *testing.T) {
		doTestAreAllPlayersFinished(t, false)
	})
}
