package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/rs/zerolog"
)

const (
	attackActionStalemateFmt = "%s attacked %s from %s, attack failed (rolled %d) and no armies were lost"
	attackActionSuccessFmt   = "%s attacked %s from %s, attack succeeded (rolled %d) and %d defending armies were lost"
	attackActionFailureFmt   = "%s attacked %s from %s, attack failed (rolled %d) and %d attacking armies were lost"
)

type AttackActionResult struct {
	actionResultBase[*AttackAction]
	DieRoll       int
	Attacking     int
	Defending     int
	Losses        int
	NationRemoved bool
}

func (aar *AttackActionResult) ActionType() string {
	return "attack"
}

func (aar *AttackActionResult) String() string {
	str := aar.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *aar.Action
	if action == nil {
		return noActionString
	}
	if aar.Losses == 0 {
		return fmt.Sprintf(attackActionStalemateFmt, action.User, action.DefendingTerritory, action.AttackingTerritory, aar.DieRoll)
	}
	if aar.Losses > 0 {
		return fmt.Sprintf(attackActionSuccessFmt, action.User, action.DefendingTerritory, action.AttackingTerritory, aar.DieRoll, aar.Losses)
	}
	return fmt.Sprintf(attackActionFailureFmt, action.User, action.DefendingTerritory, action.AttackingTerritory, aar.DieRoll, -aar.Losses)
}

type AttackAction struct {
	User               string
	AttackingTerritory string
	DefendingTerritory string
	Logger             zerolog.Logger
}

func (aa *AttackAction) DoAction(db *sql.DB) (ActionResult, error) {
	cfg, _ := config.GetConfig()

	err := ValidateUser(aa.User, db, aa.Logger)
	if err != nil {
		return nil, err
	}

	attackingTerritory, err := cfg.ResolveTerritory(aa.AttackingTerritory)
	if err != nil {
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	defendingTerritory, err := cfg.ResolveTerritory(aa.DefendingTerritory)
	if err != nil {
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if attackingTerritory.Abbreviation == defendingTerritory.Abbreviation {
		err = fmt.Errorf("cannot attack %s from %s: friendly fire not allowed", defendingTerritory.Name, attackingTerritory.Name)
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	neighbors, err := attackingTerritory.IsNeighboring(aa.DefendingTerritory)
	if err != nil {
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}
	if !neighbors {
		err = fmt.Errorf("cannot attack %s from %s: not a neighboring territory", defendingTerritory.Name, attackingTerritory.Name)
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if cfg.DoCounterattack {
		return aa.doAttackWithCounter(db, attackingTerritory, defendingTerritory)
	}
	return aa.doNormalAttack(db, attackingTerritory, defendingTerritory)
}

func (aa *AttackAction) doNormalAttack(db *sql.DB, attackingTerritory, defendingTerritory *config.Territory) (ActionResult, error) {
	infoEv := aa.Logger.Info()
	errEv := aa.Logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)

	var attacking, defending int
	const attackSQL = `SELECT army_size FROM v_nation_holdings WHERE territory = ?`
	stmt, err := db.Prepare(attackSQL + "  AND player = ?")
	if err != nil {
		aa.Logger.Err(err).Caller().Msg("Unable to prepare attack query")
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(attackingTerritory.Abbreviation, aa.User).Scan(&attacking)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		aa.Logger.Err(err).Caller().Msg("Unable to get attacking army size")
		return nil, err
	}
	if attacking == 0 {
		err = fmt.Errorf("no armies in %s controlled by %s to attack with", attackingTerritory.Name, aa.User)
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	if err = stmt.Close(); err != nil {
		aa.Logger.Err(err).Caller().Msg("Unable to close statement")
		return nil, err
	}

	stmt, err = db.Prepare(attackSQL)
	if err != nil {
		aa.Logger.Err(err).Caller().Msg("Unable to prepare defending query")
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(defendingTerritory.Abbreviation).Scan(&defending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		aa.Logger.Err(err).Caller().Msg("Unable to get defending army size")
		return nil, err
	}
	if defending == 0 {
		err = fmt.Errorf("no armies in %s", defendingTerritory.Name)
		aa.Logger.Err(err).Caller().Send()
		return nil, err
	}

	x, losses, err := attackCalculation(attacking, defending)
	if err != nil {
		aa.Logger.Err(err).Caller().Msg("Attack calculation failed")
		return nil, err
	}
	config.LogInt("dieRoll", x, infoEv, errEv)
	config.LogInt("attacking", attacking, infoEv, errEv)
	config.LogInt("defending", defending, infoEv, errEv)
	config.LogInt("losses", int(losses), infoEv, errEv)

	success := x > (defending-attacking)*2+10
	infoEv.Bool("success", success)

	var attackerLosses, defenderLosses int
	var nationRemoved bool
	if losses > 0 {
		// defending armies destroyed
		defenderLosses = int(math.Min(losses, float64(defending)))
		config.LogInt("defenderLosses", defenderLosses, infoEv, errEv)
		nationRemoved, err = UpdateHoldingArmySize(db, nil, defendingTerritory.Abbreviation, defending-defenderLosses, true, aa.Logger)
	} else {
		// attacking armies destroyed
		attackerLosses = int(math.Min(math.Abs(losses), float64(attacking)))
		config.LogInt("attackerLosses", attackerLosses, infoEv, errEv)
		nationRemoved, err = UpdateHoldingArmySize(db, nil, attackingTerritory.Abbreviation, attacking-attackerLosses, true, aa.Logger)
	}
	if err != nil {
		aa.Logger.Err(err).Caller().Msg("Unable to update holding army size")
		return nil, err
	}
	return &AttackActionResult{
		actionResultBase: actionResultBase[*AttackAction]{Action: &aa, user: aa.User},
		DieRoll:          x,
		Attacking:        attacking,
		Defending:        defending,
		Losses:           defenderLosses,
		NationRemoved:    nationRemoved,
	}, nil
}

func (aa *AttackAction) doAttackWithCounter(db *sql.DB, attackingTerritory, defendingTerritory *config.Territory) (ActionResult, error) {
	// Placeholder for Advance Wars-style attack logic
	return nil, errors.New("counterattack logic not implemented yet")
}

func attackCalculation(attacking, defending int) (int, float64, error) {
	if attacking <= 0 || defending <= 0 {
		return 0, 0, fmt.Errorf("invalid army sizes: attacking=%d, defending=%d", attacking, defending)
	}

	x := randInt(20) + 1
	success := x > (defending-attacking)*2+10

	var losses float64
	if success {
		// attack successful, losses are on the defending side
		losses = math.Floor(0.5*float64(x) + float64(attacking-defending-5))
		if losses == 0 {
			losses = 1
		}
		losses = math.Min(losses, float64(defending)) // cannot lose more armies than defending has
	} else {
		// attack failed, losses are on the attacking side (negative value)
		losses = -math.Floor(0.5*float64(x) + float64(defending-attacking-5))
		if x == 1 && losses >= 0 {
			losses = -1 // critical failure, at least one army lost
		}
		losses = math.Max(losses, -float64(attacking)) // cannot lose more armies than attacking has
	}

	return x, losses, nil
}
