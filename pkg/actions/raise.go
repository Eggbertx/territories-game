package actions

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
)

type RaiseAction struct {
	user      string
	territory string
	logger    zerolog.Logger
}

func (ra *RaiseAction) DoAction(db *sql.DB) error {
	infoEv := ra.logger.Info()
	defer infoEv.Discard()

	err := ValidateUser(ra.user, db, ra.logger)
	if err != nil {
		return err
	}

	if ra.territory == "" {
		ra.logger.Err(ErrNoTargetTerritory).Caller().Send()
		return ErrNoTargetTerritory
	}

	cfg, err := config.GetConfig()
	if err != nil {
		ra.logger.Err(err).Caller().Msg("Unable to get configuration")
		return err
	}

	territory, err := cfg.ResolveTerritory(ra.territory)
	if err != nil {
		ra.logger.Err(err).Caller().Send()
		return err
	}

	stmt, err := db.Prepare(`SELECT army_size FROM v_nation_holdings WHERE territory = ? and player = ?`)
	if err != nil {
		ra.logger.Err(err).Caller().Msg("Unable to prepare raise check statement")
		return err
	}
	defer stmt.Close()

	var armySize int
	if err = stmt.QueryRow(territory.Abbreviation, ra.user).Scan(&armySize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to raise", territory.Name, ra.user)
		}
		ra.logger.Err(err).Caller().Send()
		return err
	}

	if armySize == cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot raise army size in %s: already at maximum of %d", territory.Name, cfg.MaxArmiesPerTerritory)
		ra.logger.Err(err).Caller().Send()
		return err
	}

	if err = UpdateHoldingArmySize(db, nil, territory.Abbreviation, armySize+1, false, ra.logger); err != nil {
		return err
	}

	infoEv.Msg("Raised army size")
	return nil
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
