package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
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

	Logger zerolog.Logger
}

func (ja *JoinAction) DoAction(db *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log, _ := config.GetLogger()
		log.Err(err).Caller().Msg("Unable to get configuration")
		return nil, err
	}

	infoEv := ja.Logger.Info()
	errEv := ja.Logger.Err(nil)
	config.LogString("nation", ja.Nation, infoEv, errEv)
	config.LogString("territory", ja.Territory, infoEv, errEv)
	defer config.DiscardLogEvents(infoEv, errEv)

	if ja.Nation == "" {
		ja.Nation = fmt.Sprintf("%s's Nation", ja.User)
	}
	if ja.Territory == "" {
		ja.Logger.Err(ErrNoTargetTerritory).Caller().Send()
		return nil, ErrNoTargetTerritory
	}
	joinTerritory, err := cfg.ResolveTerritory(ja.Territory)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to begin transaction")
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
		errEv.Err(err).Caller().Send()
		return nil, err
	}
	if numPlayerMatches > 0 {
		errEv.Err(ErrPlayerAlreadyJoined).Caller().Send()
		return nil, ErrPlayerAlreadyJoined
	}

	if err = tx.QueryRow(nationAlreadyJoinedSQL, ja.Nation).Scan(&numNationMatches); err != nil {
		errEv.Err(err).Caller().Send()
		return nil, err
	}
	if numNationMatches > 0 {
		errEv.Err(ErrNationAlreadyJoined).Caller().Send()
		return nil, ErrNationAlreadyJoined
	}

	if _, err = tx.Exec(nationAddSQL, ja.Nation, ja.User, randomColor()); err != nil {
		errEv.Err(err).Caller().Msg("Unable to add nation")
		return nil, err
	}
	if _, err = tx.Exec(nationInitialHolding, ja.Nation, joinTerritory.Abbreviation, cfg.InitialArmies); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrTerritoryAlreadyOccupied
		}
		errEv.Err(err).Caller().Msg("Unable to add initial holding")
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to commit transaction")
		return nil, err
	}

	return &JoinActionResult{
		actionResultBase: actionResultBase[*JoinAction]{
			Action: &ja,
			user:   ja.User,
		},
	}, nil
}
