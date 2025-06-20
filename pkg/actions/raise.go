package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
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
	Logger    zerolog.Logger
}

func (ra *RaiseAction) DoAction(db *sql.DB) (ActionResult, error) {
	infoEv := ra.Logger.Info()
	defer infoEv.Discard()

	err := ValidateUser(ra.User, db, ra.Logger)
	if err != nil {
		return nil, err
	}

	if ra.Territory == "" {
		ra.Logger.Err(ErrNoTargetTerritory).Caller().Send()
		return nil, ErrNoTargetTerritory
	}

	cfg, err := config.GetConfig()
	if err != nil {
		ra.Logger.Err(err).Caller().Msg("Unable to get configuration")
		return nil, err
	}

	territory, err := cfg.ResolveTerritory(ra.Territory)
	if err != nil {
		ra.Logger.Err(err).Caller().Send()
		return nil, err
	}

	stmt, err := db.Prepare(`SELECT army_size FROM v_nation_holdings WHERE territory = ? and player = ?`)
	if err != nil {
		ra.Logger.Err(err).Caller().Msg("Unable to prepare raise check statement")
		return nil, err
	}
	defer stmt.Close()

	var armySize int
	if err = stmt.QueryRow(territory.Abbreviation, ra.User).Scan(&armySize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to raise", territory.Name, ra.User)
		}
		ra.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if armySize == cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot raise army size in %s: already at maximum of %d", territory.Name, cfg.MaxArmiesPerTerritory)
		ra.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if _, err = UpdateHoldingArmySize(db, nil, territory.Abbreviation, armySize+1, false, ra.Logger); err != nil {
		return nil, err
	}

	return &RaiseActionResult{
		actionResultBase: actionResultBase[*RaiseAction]{
			Action: &ra,
			user:   ra.User,
		},
	}, nil
}
