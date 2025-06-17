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
	action := *rar.action
	if action == nil {
		return noActionString
	}
	return fmt.Sprintf(raiseActionResultFmt, action.user, action.territory)
}

type RaiseAction struct {
	user      string
	territory string
	logger    zerolog.Logger
}

func (ra *RaiseAction) DoAction(db *sql.DB) (ActionResult, error) {
	infoEv := ra.logger.Info()
	defer infoEv.Discard()

	err := ValidateUser(ra.user, db, ra.logger)
	if err != nil {
		return nil, err
	}

	if ra.territory == "" {
		ra.logger.Err(ErrNoTargetTerritory).Caller().Send()
		return nil, ErrNoTargetTerritory
	}

	cfg, err := config.GetConfig()
	if err != nil {
		ra.logger.Err(err).Caller().Msg("Unable to get configuration")
		return nil, err
	}

	territory, err := cfg.ResolveTerritory(ra.territory)
	if err != nil {
		ra.logger.Err(err).Caller().Send()
		return nil, err
	}

	stmt, err := db.Prepare(`SELECT army_size FROM v_nation_holdings WHERE territory = ? and player = ?`)
	if err != nil {
		ra.logger.Err(err).Caller().Msg("Unable to prepare raise check statement")
		return nil, err
	}
	defer stmt.Close()

	var armySize int
	if err = stmt.QueryRow(territory.Abbreviation, ra.user).Scan(&armySize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to raise", territory.Name, ra.user)
		}
		ra.logger.Err(err).Caller().Send()
		return nil, err
	}

	if armySize == cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot raise army size in %s: already at maximum of %d", territory.Name, cfg.MaxArmiesPerTerritory)
		ra.logger.Err(err).Caller().Send()
		return nil, err
	}

	if err = UpdateHoldingArmySize(db, nil, territory.Abbreviation, armySize+1, false, ra.logger); err != nil {
		return nil, err
	}

	var result RaiseActionResult
	result.action = &ra
	result.user = ra.user
	ra.logger.Info().Msg(result.String())
	return &result, nil
}

func raiseActionParser(args ...string) (Action, error) {
	var action RaiseAction
	if len(args) < 1 {
		return nil, ErrMissingUser
	}
	action.user = args[0]

	if len(args) < 2 {
		return nil, fmt.Errorf("missing territory argument for raise action")
	}
	action.territory = args[1]

	var err error
	action.logger, err = config.GetLogger()
	if err != nil {
		return nil, fmt.Errorf("unable to get logger: %w", err)
	}
	action.logger = action.logger.With().Str("action", "raise").Str("user", action.user).Str("territory", action.territory).Logger()

	return &action, nil
}
