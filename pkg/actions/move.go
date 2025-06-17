package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
)

const (
	moveAllArmiesResultFmt  = "%s moved all armies from %s to %s"
	moveSomeArmiesResultFmt = "%s moved %d armies from %s to %s"
)

var (
	ErrInvalidMove = errors.New("invalid move action format, expected 'move' or 'moveX' where X is the number of armies")
)

type MoveActionResult struct {
	actionResultBase[*MoveAction]
}

func (mar *MoveActionResult) ActionType() string {
	return "move"
}

func (mar *MoveActionResult) String() string {
	str := mar.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *mar.action
	if action == nil {
		return noActionString
	}
	if action.armies <= 0 {
		return fmt.Sprintf(moveAllArmiesResultFmt, action.user, action.sourceTerritory, action.destinationTerritory)
	}
	return fmt.Sprintf(moveSomeArmiesResultFmt, action.user, action.armies, action.sourceTerritory, action.destinationTerritory)
}

type MoveAction struct {
	user                 string
	sourceTerritory      string
	destinationTerritory string
	armies               int

	logger zerolog.Logger
}

func (ma *MoveAction) DoAction(db *sql.DB) (ActionResult, error) {
	if ma.destinationTerritory == "" || ma.sourceTerritory == ma.destinationTerritory {
		ma.logger.Err(ErrNoTargetTerritory).Caller().Send()
		return nil, ErrNoTargetTerritory
	}

	cfg, err := config.GetConfig()
	if err != nil {
		ma.logger.Err(err).Caller().Msg("Unable to get configuration")
		return nil, err
	}

	sourceTerritory, err := cfg.ResolveTerritory(ma.sourceTerritory)
	if err != nil {
		ma.logger.Err(err).Caller().Str("sourceTerritory", ma.sourceTerritory).Send()
		return nil, err
	}
	destTerritory, err := cfg.ResolveTerritory(ma.destinationTerritory)
	if err != nil {
		ma.logger.Err(err).Caller().Str("destinationTerritory", ma.destinationTerritory).Send()
		return nil, err
	}

	isNeighboring, err := sourceTerritory.IsNeighboring(ma.destinationTerritory)
	if err != nil {
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	if !isNeighboring {
		err = fmt.Errorf("cannot move from %s to %s: not a neighboring territory", sourceTerritory.Name, destTerritory.Name)
		ma.logger.Err(err).Caller().Send()
	}

	var armiesInSourceTerritory, armiesInDestTerritory int
	var fromPlayer, destinationPlayer string
	const moveSQL = "SELECT army_size, player FROM v_nation_holdings WHERE territory = ?"
	stmt, err := db.Prepare(moveSQL)
	if err != nil {
		ma.logger.Err(err).Caller().Msg("Unable to prepare move query")
		return nil, err
	}
	defer stmt.Close()
	err = stmt.QueryRow(sourceTerritory.Abbreviation).Scan(&armiesInSourceTerritory, &fromPlayer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to move", sourceTerritory.Name, ma.user)
		}
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	if ma.armies > 0 && ma.armies > armiesInSourceTerritory {
		err = fmt.Errorf("cannot move %d armies from %s: only %d available", ma.armies, sourceTerritory.Name, armiesInSourceTerritory)
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}
	if ma.armies <= 0 {
		ma.armies = armiesInSourceTerritory // none specified, move all armies in source territory
	}

	err = stmt.QueryRow(destTerritory.Abbreviation).Scan(&armiesInDestTerritory, &destinationPlayer)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	if fromPlayer != ma.user {
		err = fmt.Errorf("cannot move from %s: no armies controlled by player", sourceTerritory.Name)
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	if armiesInDestTerritory > 0 && destinationPlayer != ma.user {
		err = ErrTerritoryAlreadyOccupied
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	if armiesInDestTerritory+ma.armies > cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot move %d armies to %s: would exceed maximum of %d", ma.armies, destTerritory.Name, cfg.MaxArmiesPerTerritory)
		ma.logger.Err(err).Caller().Send()
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		ma.logger.Err(err).Caller().Msg("Unable to begin transaction")
		return nil, err
	}
	defer tx.Rollback()

	if err = UpdateHoldingArmySize(db, tx, sourceTerritory.Abbreviation, armiesInSourceTerritory-ma.armies, false, ma.logger); err != nil {
		return nil, err
	}
	if armiesInDestTerritory == 0 {
		stmt, err := tx.Prepare(`INSERT INTO holdings (nation_id, territory, army_size) VALUES(
			(SELECT id FROM nations WHERE player = ?),
			?, ?)`)
		if err != nil {
			ma.logger.Err(err).Caller().Msg("Unable to prepare insert holding statement")
			return nil, err
		}
		defer stmt.Close()
		if _, err = stmt.Exec(ma.user, destTerritory.Abbreviation, ma.armies); err != nil {
			ma.logger.Err(err).Caller().Msg("Unable to insert new holding")
			return nil, err
		}
	} else {
		if err = UpdateHoldingArmySize(db, tx, destTerritory.Abbreviation, armiesInDestTerritory+ma.armies, false, ma.logger); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		ma.logger.Err(err).Caller().Msg("Unable to commit transaction")
		return nil, err
	}
	var result MoveActionResult
	result.action = &ma
	result.user = ma.user

	ma.logger.Info().Msg(result.String())
	return &result, nil
}

func moveActionParser(s ...string) (Action, error) {
	if len(s) < 1 {
		return nil, ErrMissingUser
	}
	action := &MoveAction{
		user: s[0],
	}
	action.user = s[0]

	if len(s) == 3 {
		// user, source, destination
		action.armies = 0 // move all armies in the source territory
		action.sourceTerritory = s[1]
		action.destinationTerritory = s[2]
	} else if len(s) == 4 {
		// user, armies, source, destination
		var err error
		action.armies, err = strconv.Atoi(s[1])
		if err != nil {
			return nil, fmt.Errorf("%w (invalid number of armies: %s)", ErrInvalidMove, s[1])
		}
		action.sourceTerritory = s[2]
		action.destinationTerritory = s[3]
	} else {
		return nil, fmt.Errorf("%w (expected 3 or 4 arguments)", ErrInvalidMove)
	}

	var err error
	action.logger, err = config.GetLogger()
	if err != nil {
		action.logger.Err(err).Caller().Msg("Unable to get logger")
		return nil, err
	}
	return action, nil
}
