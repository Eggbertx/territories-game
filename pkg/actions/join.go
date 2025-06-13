package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

type JoinAction struct {
	user      string
	nation    string
	territory string

	logger zerolog.Logger
}

func (ja *JoinAction) DoAction(db *sql.DB) error {
	cfg, err := config.GetConfig()
	if err != nil {
		log, _ := config.GetLogger()
		log.Err(err).Caller().Msg("Unable to get configuration")
		return err
	}

	infoEv := ja.logger.Info()
	errEv := ja.logger.Err(nil)
	config.LogString("nation", ja.nation, infoEv, errEv)
	config.LogString("territory", ja.territory, infoEv, errEv)
	defer config.DiscardLogEvents(infoEv, errEv)

	if ja.nation == "" {
		ja.nation = fmt.Sprintf("%s's Nation", ja.user)
	}
	if ja.territory == "" {
		ja.logger.Err(ErrNoTargetTerritory).Caller().Send()
		return ErrNoTargetTerritory
	}
	joinTerritory, err := cfg.ResolveTerritory(ja.territory)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to begin transaction")
		return err
	}
	defer tx.Rollback()

	const userAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE player = ?`
	const nationAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE country_name = ?`
	const nationAddSQL = `INSERT INTO nations (country_name,player, color) VALUES(?,?,?)`
	const nationInitialHolding = `INSERT INTO holdings (nation_id, territory, army_size) VALUES(
		(SELECT id FROM nations WHERE country_name = ?),
		?, 1)`
	var numPlayerMatches int
	var numNationMatches int
	if err = tx.QueryRow(userAlreadyJoinedSQL, ja.user).Scan(&numPlayerMatches); err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}
	if numPlayerMatches > 0 {
		errEv.Err(ErrPlayerAlreadyJoined).Caller().Send()
		return ErrPlayerAlreadyJoined
	}

	if err = tx.QueryRow(nationAlreadyJoinedSQL, ja.nation).Scan(&numNationMatches); err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}
	if numNationMatches > 0 {
		errEv.Err(ErrNationAlreadyJoined).Caller().Send()
		return ErrNationAlreadyJoined
	}

	if _, err = tx.Exec(nationAddSQL, ja.nation, ja.user, randomColor()); err != nil {
		errEv.Err(err).Caller().Msg("Unable to add nation")
		return err
	}
	if _, err = tx.Exec(nationInitialHolding, ja.nation, joinTerritory.Abbreviation); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrTerritoryAlreadyOccupied
		}
		errEv.Err(err).Caller().Msg("Unable to add initial holding")
		return err
	}
	if err = tx.Commit(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to commit transaction")
		return err
	}
	infoEv.Str("territory", joinTerritory.Name).Msg("Added new user")
	return nil
}

func joinActionParser(s ...string) (Action, error) {
	// arg[0] = user
	// arg[1] = nation name
	// arg[2] = territory
	var err error

	if len(s) < 1 {
		return nil, ErrMissingUser
	}
	var action JoinAction
	action.user = s[0]
	if len(s) < 2 {
		action.nation = fmt.Sprintf("%s's Nation", s[0])
	} else {
		action.nation = s[1]
	}

	if len(s) < 3 {
		return nil, ErrNoTargetTerritory
	}
	action.territory = s[2]
	action.logger, err = config.GetLogger()
	if err != nil {
		action.logger.Err(err).Caller().Msg("Unable to get logger")
		return nil, err
	}
	return &action, nil
}
