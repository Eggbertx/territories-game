package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

const (
	raiseActionResultFmt = "%s raised an army in %s"
)

type RaiseActionResult struct {
	actionResultBase[*RaiseAction]
}

func (rar *RaiseActionResult) ActionType() string {
	return "raise"
}

func (rar *RaiseActionResult) String() string {
	str := rar.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *rar.Action
	if action == nil {
		return noActionString
	}
	return fmt.Sprintf(raiseActionResultFmt, action.User, action.Territory)
}

type RaiseAction struct {
	User      string
	Territory string
}

func (ra *RaiseAction) DoAction(tdb *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	if err = db.ValidateUser(ra.User, tdb, cfg.LogError); err != nil {
		return nil, err
	}

	if ra.Territory == "" {
		cfg.LogError("No target territory specified")
		return nil, ErrNoTargetTerritory
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

	if err = checkReturnsRemainingIfManaging(tx, ra.User, cfg, cfg.LogError); err != nil {
		return nil, err
	}

	territory, err := cfg.ResolveTerritory(ra.Territory)
	if err != nil {
		cfg.LogError("Unable to resolve territory", "error", err)
		return nil, err
	}

	stmt, err := tx.Prepare(`SELECT army_size FROM v_nation_holdings WHERE territory = ? and player = ?`)
	if err != nil {
		cfg.LogError("Unable to prepare raise check statement", "error", err)
		return nil, err
	}
	defer stmt.Close()

	var armySize int
	if err = stmt.QueryRow(territory.Abbreviation, ra.User).Scan(&armySize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to raise", territory.Name, ra.User)
		}
		cfg.LogError("Unable to check raise conditions", "error", err)
		return nil, err
	}

	if armySize == cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot raise army size in %s: already at maximum of %d", territory.Name, cfg.MaxArmiesPerTerritory)
		cfg.LogError("Not enough actions remaining", "player", ra.User, "error", err)
		return nil, err
	}

	if _, err = db.UpdateHoldingArmySize(tdb, tx, territory.Abbreviation, armySize+1, false); err != nil {
		return nil, err
	}

	if err = addTurnEntryIfManaging(tx, ra.User, "raise"); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		cfg.LogError("Unable to commit transaction", "error", err)
		return nil, err
	}

	return &RaiseActionResult{
		actionResultBase: actionResultBase[*RaiseAction]{
			Action: &ra,
			user:   ra.User,
		},
	}, nil
}
