package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

const (
	moveAllArmiesResultFmt  = "%s moved all armies from %s to %s"
	moveSomeArmiesResultFmt = "%s moved %d armies from %s to %s"
	moveFailed              = "%s's armies failed to clear %s (failed invasion check)"
	moveFailedNationRemoved = "%s's clearing party sent to %s was lost, and now %s has no territories left. How sad :("
)

var (
	ErrInvalidMove             = errors.New("invalid move action format, expected 'move' or 'moveX' where X is the number of armies")
	ErrMissingSourceTerritory  = errors.New("source territory not specified")
	ErrMissingDestTerritory    = errors.New("destination territory not specified")
	ErrSourceEqualsDestination = errors.New("source and destination territories cannot be the same")
)

type MoveActionResult struct {
	actionResultBase[*MoveAction]
	FailedMove    bool
	NationRemoved bool
}

func (mar *MoveActionResult) ActionType() string {
	return "move"
}

func (mar *MoveActionResult) String() string {
	str := mar.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *mar.Action
	if action == nil {
		return noActionString
	}
	if action.Armies <= 0 {
		return fmt.Sprintf(moveAllArmiesResultFmt, action.User, action.Source, action.Destination)
	}

	if mar.NationRemoved {
		return fmt.Sprintf(moveFailedNationRemoved, action.User, action.Destination, action.User)
	}

	if mar.FailedMove {
		return fmt.Sprintf(moveFailed, action.User, action.Destination)
	}

	return fmt.Sprintf(moveSomeArmiesResultFmt, action.User, action.Armies, action.Source, action.Destination)
}

type MoveAction struct {
	User        string
	Source      string
	Destination string
	Armies      int
}

func (ma *MoveAction) DoAction(tdb *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	if ma.Source == "" {
		cfg.LogError("No source territory specified")
		return nil, ErrMissingSourceTerritory
	}
	if ma.Destination == "" {
		cfg.LogError("No destination territory specified")
		return nil, ErrMissingDestTerritory
	}
	if ma.Source == ma.Destination {
		cfg.LogError("Source and destination territories cannot be the same")
		return nil, ErrSourceEqualsDestination
	}

	sourceTerritory, err := cfg.ResolveTerritory(ma.Source)
	if err != nil {
		cfg.LogError("Unable to resolve source territory", "error", err)
		return nil, err
	}
	destTerritory, err := cfg.ResolveTerritory(ma.Destination)
	if err != nil {
		cfg.LogError("Unable to resolve destination territory", "error", err)
		return nil, err
	}

	isNeighboring, err := sourceTerritory.IsNeighboring(ma.Destination)
	if err != nil {
		cfg.LogError("Unable to check if territories are neighboring", "error", err)
		return nil, err
	}

	if !isNeighboring {
		err = fmt.Errorf("cannot move from %s to %s: not a neighboring territory", sourceTerritory.Name, destTerritory.Name)
		cfg.LogError("Unable to move armies", "error", err)
		return nil, err
	}

	tx, err := tdb.Begin()
	if err != nil {
		cfg.LogError("Unable to begin transaction", "error", err)
		return nil, err
	}
	defer tx.Rollback()

	if err = checkIfEnoughPlayersToStart(tx, cfg, cfg.LogError); err != nil {
		return nil, err
	}

	if err = checkReturnsRemainingIfManaging(tx, ma.User, cfg, cfg.LogError); err != nil {
		return nil, err
	}

	var armiesInSourceTerritory, armiesInDestTerritory int
	var fromPlayer, destinationPlayer string
	const moveSQL = "SELECT army_size, player FROM v_nation_holdings WHERE territory = ?"
	stmt, err := tx.Prepare(moveSQL)
	if err != nil {
		cfg.LogError("Unable to prepare move query", "error", err)
		return nil, err
	}
	defer stmt.Close()
	err = stmt.QueryRow(sourceTerritory.Abbreviation).Scan(&armiesInSourceTerritory, &fromPlayer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to move", sourceTerritory.Name, ma.User)
		}
		cfg.LogError("Unable to query source territory", "error", err)
		return nil, err
	}

	if ma.Armies > 0 && ma.Armies > armiesInSourceTerritory {
		err = fmt.Errorf("cannot move %d armies from %s: only %d available", ma.Armies, sourceTerritory.Name, armiesInSourceTerritory)
		cfg.LogError("Unable to move armies", "error", err)
		return nil, err
	}
	if ma.Armies <= 0 {
		ma.Armies = armiesInSourceTerritory // none specified, move all armies in source territory
	}

	err = stmt.QueryRow(destTerritory.Abbreviation).Scan(&armiesInDestTerritory, &destinationPlayer)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		cfg.LogError("Unable to query destination territory", "error", err)
		return nil, err
	}

	if fromPlayer != ma.User {
		err = fmt.Errorf("cannot move from %s: no armies controlled by player", sourceTerritory.Name)
		cfg.LogError("Unable to move armies", "error", err)
		return nil, err
	}

	if armiesInDestTerritory > 0 && destinationPlayer != ma.User {
		err = ErrTerritoryAlreadyOccupied
		cfg.LogError("Territory already occupied", "error", err)
		return nil, err
	}

	if armiesInDestTerritory+ma.Armies > cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot move %d armies to %s: would exceed maximum of %d", ma.Armies, destTerritory.Name, cfg.MaxArmiesPerTerritory)
		cfg.LogError("Unable to move armies", "error", err)
		return nil, err
	}

	var newDestinationArmies int
	if armiesInDestTerritory == 0 && cfg.UnclaimedTerritoriesHave1Army {
		_, losses, err := attackCalculation(ma.Armies, 1)
		if err != nil {
			cfg.LogError("Unable to calculate attack", "error", err)
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
			cfg.LogError("Unable to prepare insert holding statement", "error", err)
			return nil, err
		}
		defer stmt.Close()
		if _, err = stmt.Exec(ma.User, destTerritory.Abbreviation, newDestinationArmies); err != nil {
			cfg.LogError("Unable to insert new holding", "error", err)
			return nil, err
		}
	} else if newDestinationArmies > 0 {
		// player is joining armies into an existing holding, update the army size
		if _, err = db.UpdateHoldingArmySize(tdb, tx, destTerritory.Abbreviation, newDestinationArmies, false); err != nil {
			cfg.LogError("Unable to update holding army size", "error", err)
			return nil, err
		}
	}

	// remove armies from source territory, if they lost armies in the attack and have no armies left, delete the holding
	var nationRemoved bool
	if nationRemoved, err = db.UpdateHoldingArmySize(tdb, tx, sourceTerritory.Abbreviation, armiesInSourceTerritory-ma.Armies, true); err != nil {
		return nil, err
	}

	if err = addTurnEntryIfManaging(tx, ma.User, "move"); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		cfg.LogError("Unable to commit transaction", "error", err)
		return nil, err
	}
	return &MoveActionResult{
		actionResultBase: actionResultBase[*MoveAction]{
			Action: &ma,
			user:   ma.User,
		},
		FailedMove:    newDestinationArmies == 0,
		NationRemoved: nationRemoved,
	}, nil
}
