package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
)

type AttackAction struct {
	user               string
	attackingTerritory string
	defendingTerritory string
	logger             zerolog.Logger
}

func (aa *AttackAction) DoAction(db *sql.DB) error {
	cfg, _ := config.GetConfig()

	err := ValidateUser(aa.user, db, aa.logger)
	if err != nil {
		return err
	}

	attackingTerritory, err := cfg.ResolveTerritory(aa.attackingTerritory)
	if err != nil {
		aa.logger.Err(err).Caller().Send()
		return err
	}

	defendingTerritory, err := cfg.ResolveTerritory(aa.defendingTerritory)
	if err != nil {
		aa.logger.Err(err).Caller().Send()
		return err
	}

	if attackingTerritory.Abbreviation == defendingTerritory.Abbreviation {
		err = fmt.Errorf("cannot attack %s from %s: friendly fire not allowed", defendingTerritory.Name, attackingTerritory.Name)
		aa.logger.Err(err).Caller().Send()
		return err
	}

	neighbors, err := attackingTerritory.IsNeighboring(aa.defendingTerritory)
	if err != nil {
		aa.logger.Err(err).Caller().Send()
		return err
	}
	if !neighbors {
		err = fmt.Errorf("cannot attack %s from %s: not a neighboring territory", defendingTerritory.Name, attackingTerritory.Name)
		aa.logger.Err(err).Caller().Send()
		return err
	}

	if cfg.DoCounterattack {
		return aa.doAttackWithCounter(db, attackingTerritory, defendingTerritory)
	}
	return aa.doNormalAttack(db, attackingTerritory, defendingTerritory)
}

func (aa *AttackAction) doNormalAttack(db *sql.DB, attackingTerritory, defendingTerritory *config.Territory) error {
	infoEv := aa.logger.Info()
	errEv := aa.logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)

	var attacking, defending int
	const attackSQL = `SELECT army_size FROM v_nation_holdings WHERE territory = ?`
	stmt, err := db.Prepare(attackSQL + "  AND player = ?")
	if err != nil {
		aa.logger.Err(err).Caller().Msg("Unable to prepare attack query")
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(attackingTerritory.Abbreviation, aa.user).Scan(&attacking)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		aa.logger.Err(err).Caller().Msg("Unable to get attacking army size")
		return err
	}
	if attacking == 0 {
		err = fmt.Errorf("no armies in %s controlled by %s to attack with", attackingTerritory.Name, aa.user)
		aa.logger.Err(err).Caller().Send()
		return err
	}

	if err = stmt.Close(); err != nil {
		aa.logger.Err(err).Caller().Msg("Unable to close statement")
		return err
	}

	stmt, err = db.Prepare(attackSQL)
	if err != nil {
		aa.logger.Err(err).Caller().Msg("Unable to prepare defending query")
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(defendingTerritory.Abbreviation).Scan(&defending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		aa.logger.Err(err).Caller().Msg("Unable to get defending army size")
		return err
	}
	if defending == 0 {
		err = fmt.Errorf("no armies in %s", defendingTerritory.Name)
		aa.logger.Err(err).Caller().Send()
		return err
	}

	x := randInt(20) + 1

	success := x > (defending-attacking)*2+10
	infoEv.Bool("success", success)

	losses := math.Floor(0.5*float64(x) + float64(attacking-defending-5))
	if success && losses == 0 {
		losses = 1 // at least one army must be lost
	}
	var attackerLosses, defenderLosses int
	if losses > 0 {
		// defending armies destroyed
		defenderLosses = int(math.Min(losses, float64(defending)))
		config.LogInt("defenderLosses", defenderLosses, infoEv, errEv)
		err = UpdateHoldingArmySize(db, nil, defendingTerritory.Abbreviation, defending-defenderLosses, true, aa.logger)
	} else {
		// attacking armies destroyed
		attackerLosses = int(math.Min(math.Abs(losses), float64(attacking)))
		config.LogInt("attackerLosses", attackerLosses, infoEv, errEv)
		err = UpdateHoldingArmySize(db, nil, attackingTerritory.Abbreviation, attacking-attackerLosses, true, aa.logger)
	}
	return err
}

func (aa *AttackAction) doAttackWithCounter(db *sql.DB, attackingTerritory, defendingTerritory *config.Territory) error {
	// Placeholder for Advance Wars-style attack logic
	return errors.New("counterattack logic not implemented yet")
}

func attackActionParser(args ...string) (Action, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("attack action requires 3 arguments: user, attacking territory, defending territory")
	}
	action := &AttackAction{
		user:               args[0],
		attackingTerritory: args[1],
		defendingTerritory: args[2],
	}

	if action.user == "" {
		return nil, ErrMissingUser
	}

	var err error
	action.logger, err = config.GetLogger()
	if err != nil {
		action.logger.Err(err).Caller().Msg("Failed to get logger for attack action")
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	return action, nil
}
