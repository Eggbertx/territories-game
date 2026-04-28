package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/mattn/go-sqlite3"
)

const (
	joinActionResultFmt = "%s founded by %s in %s"
)

type JoinActionResult struct {
	actionResultBase[*JoinAction]
}

func (jar *JoinActionResult) ActionType() string {
	return "join"
}

func (jar *JoinActionResult) String() string {
	str := jar.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *jar.Action
	if action == nil {
		return noActionString
	}
	return fmt.Sprintf(joinActionResultFmt, action.Nation, action.User, action.Territory)
}

type JoinAction struct {
	User      string
	Nation    string
	Territory string
}

func (ja *JoinAction) DoAction(tdb *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	if ja.Nation == "" {
		ja.Nation = fmt.Sprintf("%s's Nation", ja.User)
	}
	if ja.Territory == "" {
		cfg.LogError("No target territory specified")
		return nil, ErrNoTargetTerritory
	}
	joinTerritory, err := cfg.ResolveTerritory(ja.Territory)
	if err != nil {
		cfg.LogError("Unable to resolve territory", "error", err)
		return nil, err
	}

	tx, err := tdb.Begin()
	if err != nil {
		cfg.LogError("Unable to begin transaction", "error", err)
		return nil, err
	}
	defer tx.Rollback()

	const userAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE player = ?`
	const nationAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE country_name = ?`
	const nationAddSQL = `INSERT INTO nations (country_name,player, color) VALUES(?,?,?)`
	const nationInitialHolding = `INSERT INTO holdings (nation_id, territory, army_size) VALUES(
		(SELECT id FROM nations WHERE country_name = ?),
		?, ?)`
	var numPlayerMatches int
	var numNationMatches int
	if err = tx.QueryRow(userAlreadyJoinedSQL, ja.User).Scan(&numPlayerMatches); err != nil {
		cfg.LogError("Error querying user", "error", err)
		return nil, err
	}
	if numPlayerMatches > 0 {
		cfg.LogError("Player has already joined a nation")
		return nil, db.ErrPlayerAlreadyJoined
	}

	if err = tx.QueryRow(nationAlreadyJoinedSQL, ja.Nation).Scan(&numNationMatches); err != nil {
		cfg.LogError("Error querying nation", "error", err)
		return nil, err
	}
	if numNationMatches > 0 {
		cfg.LogError("Nation with the given name already exists")
		return nil, db.ErrNationAlreadyJoined
	}

	if _, err = tx.Exec(nationAddSQL, ja.Nation, ja.User, randomColor()); err != nil {
		cfg.LogError("Unable to add nation", "error", err)
		return nil, err
	}
	if _, err = tx.Exec(nationInitialHolding, ja.Nation, joinTerritory.Abbreviation, cfg.InitialArmies); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrTerritoryAlreadyOccupied
		}
		cfg.LogError("Unable to add initial holding", "error", err)
		return nil, err
	}

	if err = addTurnEntryIfManaging(tx, ja.User, "join"); err != nil {
		cfg.LogError("Unable to add turn entry", "error", err)
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		cfg.LogError("Unable to commit transaction", "error", err)
		return nil, err
	}

	return &JoinActionResult{
		actionResultBase: actionResultBase[*JoinAction]{
			Action: &ja,
			user:   ja.User,
		},
	}, nil
}
