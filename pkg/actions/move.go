package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
)

const (
	moveAllArmiesResultFmt  = "%s moved all armies from %s to %s"
	moveSomeArmiesResultFmt = "%s moved %d armies from %s to %s"
	moveFailed              = "%s's armies failed to clear %s (failed invasion check)"
	moveFailedNationRemoved = "%s's clearing party sent to %s was lost, and now %s has no territories left. How sad :("
)

var (
	ErrInvalidMove = errors.New("invalid move action format, expected 'move' or 'moveX' where X is the number of armies")
)

type MoveActionResult struct {
	actionResultBase[*MoveAction]
	failedMove    bool
	nationRemoved bool
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
	if action.Armies <= 0 {
		return fmt.Sprintf(moveAllArmiesResultFmt, action.User, action.Source, action.Destination)
	}

	if mar.nationRemoved {
		return fmt.Sprintf(moveFailedNationRemoved, action.User, action.Destination, action.User)
	}

	if mar.failedMove {
		return fmt.Sprintf(moveFailed, action.User, action.Destination)
	}

	return fmt.Sprintf(moveSomeArmiesResultFmt, action.User, action.Armies, action.Source, action.Destination)
}

type MoveAction struct {
	User        string
	Source      string
	Destination string
	Armies      int

	Logger zerolog.Logger
}

func (ma *MoveAction) DoAction(db *sql.DB) (ActionResult, error) {
	if ma.Destination == "" || ma.Source == ma.Destination {
		ma.Logger.Err(ErrNoTargetTerritory).Caller().Send()
		return nil, ErrNoTargetTerritory
	}

	cfg, err := config.GetConfig()
	if err != nil {
		ma.Logger.Err(err).Caller().Msg("Unable to get configuration")
		return nil, err
	}

	sourceTerritory, err := cfg.ResolveTerritory(ma.Source)
	if err != nil {
		ma.Logger.Err(err).Caller().Str("sourceTerritory", ma.Source).Send()
		return nil, err
	}
	destTerritory, err := cfg.ResolveTerritory(ma.Destination)
	if err != nil {
		ma.Logger.Err(err).Caller().Str("destinationTerritory", ma.Destination).Send()
		return nil, err
	}

	isNeighboring, err := sourceTerritory.IsNeighboring(ma.Destination)
	if err != nil {
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if !isNeighboring {
		err = fmt.Errorf("cannot move from %s to %s: not a neighboring territory", sourceTerritory.Name, destTerritory.Name)
		ma.Logger.Err(err).Caller().Send()
	}

	var armiesInSourceTerritory, armiesInDestTerritory int
	var fromPlayer, destinationPlayer string
	const moveSQL = "SELECT army_size, player FROM v_nation_holdings WHERE territory = ?"
	stmt, err := db.Prepare(moveSQL)
	if err != nil {
		ma.Logger.Err(err).Caller().Msg("Unable to prepare move query")
		return nil, err
	}
	defer stmt.Close()
	err = stmt.QueryRow(sourceTerritory.Abbreviation).Scan(&armiesInSourceTerritory, &fromPlayer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to move", sourceTerritory.Name, ma.User)
		}
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if ma.Armies > 0 && ma.Armies > armiesInSourceTerritory {
		err = fmt.Errorf("cannot move %d armies from %s: only %d available", ma.Armies, sourceTerritory.Name, armiesInSourceTerritory)
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}
	if ma.Armies <= 0 {
		ma.Armies = armiesInSourceTerritory // none specified, move all armies in source territory
	}

	err = stmt.QueryRow(destTerritory.Abbreviation).Scan(&armiesInDestTerritory, &destinationPlayer)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if fromPlayer != ma.User {
		err = fmt.Errorf("cannot move from %s: no armies controlled by player", sourceTerritory.Name)
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if armiesInDestTerritory > 0 && destinationPlayer != ma.User {
		err = ErrTerritoryAlreadyOccupied
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if armiesInDestTerritory+ma.Armies > cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot move %d armies to %s: would exceed maximum of %d", ma.Armies, destTerritory.Name, cfg.MaxArmiesPerTerritory)
		ma.Logger.Err(err).Caller().Send()
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		ma.Logger.Err(err).Caller().Msg("Unable to begin transaction")
		return nil, err
	}
	defer tx.Rollback()

	var newDestinationArmies int
	if armiesInDestTerritory == 0 && cfg.UnclaimedTerritoriesHave1Army {
		_, losses, err := attackCalculation(ma.Armies, 1)
		if err != nil {
			ma.Logger.Err(err).Caller().Send()
			return nil, err
		}
		if losses < 0 {
			// territory not cleared, attack failed
			newDestinationArmies = ma.Armies + int(losses)
		} else {
			newDestinationArmies = ma.Armies
		}
	} else {
		newDestinationArmies = armiesInDestTerritory + ma.Armies
	}

	if armiesInDestTerritory == 0 && newDestinationArmies > 0 {
		// player is claiming an unoccupied territory, insert a new holding
		stmt, err := tx.Prepare(`INSERT INTO holdings (nation_id, territory, army_size) VALUES(
			(SELECT id FROM nations WHERE player = ?),
			?, ?)`)
		if err != nil {
			ma.Logger.Err(err).Caller().Msg("Unable to prepare insert holding statement")
			return nil, err
		}
		defer stmt.Close()
		if _, err = stmt.Exec(ma.User, destTerritory.Abbreviation, newDestinationArmies); err != nil {
			ma.Logger.Err(err).Caller().Msg("Unable to insert new holding")
			return nil, err
		}
	} else if newDestinationArmies > 0 {
		// player is joining armies into an existing holding, update the army size
		if _, err = UpdateHoldingArmySize(db, tx, destTerritory.Abbreviation, newDestinationArmies, false, ma.Logger); err != nil {
			return nil, err
		}
	}

	// remove armies from source territory, if they lost armies in the attack and have no armies left, delete the holding
	var nationRemoved bool
	if nationRemoved, err = UpdateHoldingArmySize(db, tx, sourceTerritory.Abbreviation, armiesInSourceTerritory-ma.Armies, true, ma.Logger); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		ma.Logger.Err(err).Caller().Msg("Unable to commit transaction")
		return nil, err
	}
	var result MoveActionResult
	result.action = &ma
	result.user = ma.User
	result.failedMove = newDestinationArmies == 0
	result.nationRemoved = nationRemoved

	ma.Logger.Info().Msg(result.String())
	return &result, nil
}
