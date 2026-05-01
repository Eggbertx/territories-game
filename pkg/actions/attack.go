package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
)

const (
	attackActionStalemateFmt = "%s attacked %s from %s, attack failed (rolled %d) but no armies were lost"
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
}

func (aa *AttackAction) DoAction(tdb *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	if err := db.ValidateUser(aa.User, tdb, cfg.LogError); err != nil {
		return nil, err
	}

	if err = checkIfEnoughPlayersToStart(nil, cfg, cfg.LogError); err != nil {
		return nil, err
	}

	if err = checkReturnsRemainingIfManaging(nil, aa.User, cfg, cfg.LogError); err != nil {
		return nil, err
	}

	attackingTerritory, err := cfg.ResolveTerritory(aa.AttackingTerritory)
	if err != nil {
		cfg.LogError("Unable to resolve attacking territory", "error", err)
		return nil, err
	}

	defendingTerritory, err := cfg.ResolveTerritory(aa.DefendingTerritory)
	if err != nil {
		cfg.LogError("Unable to resolve defending territory", "error", err)
		return nil, err
	}

	if attackingTerritory.Abbreviation == defendingTerritory.Abbreviation {
		err = fmt.Errorf("cannot attack %s from %s: friendly fire not allowed", defendingTerritory.Name, attackingTerritory.Name)
		cfg.LogError(err.Error())
		return nil, err
	}

	neighbors, err := attackingTerritory.IsNeighboring(aa.DefendingTerritory)
	if err != nil {
		cfg.LogError("Unable to check neighboring territories", "error", err)
		return nil, err
	}
	if !neighbors {
		err = fmt.Errorf("cannot attack %s from %s: not a neighboring territory", defendingTerritory.Name, attackingTerritory.Name)
		cfg.LogError("cannot attack territory (not neighboring)", "defending", defendingTerritory.Name, "attacking", attackingTerritory.Name)
		return nil, err
	}

	if cfg.DoCounterattack {
		return aa.doAttackWithCounter(tdb, attackingTerritory, defendingTerritory)
	}
	return aa.doNormalAttack(tdb, attackingTerritory, defendingTerritory)
}

func (aa *AttackAction) doNormalAttack(tdb *sql.DB, attackingTerritory, defendingTerritory *config.Territory) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	var attacking, defending int
	const attackSQL = `SELECT army_size FROM v_nation_holdings WHERE territory = ?`
	stmt, err := tdb.Prepare(attackSQL + "  AND player = ?")
	if err != nil {
		cfg.LogError("Unable to prepare attack query", "error", err)
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(attackingTerritory.Abbreviation, aa.User).Scan(&attacking)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		cfg.LogError("Unable to get attacking army size", "error", err)
		return nil, err
	}
	if attacking == 0 {
		err = fmt.Errorf("no armies in %s controlled by %s to attack with", attackingTerritory.Name, aa.User)
		cfg.LogError("No armies available to attack with", "user", aa.User, "defending", defendingTerritory.Name, "attacking", attackingTerritory.Name)
		return nil, err
	}

	if err = stmt.Close(); err != nil {
		cfg.LogError("Unable to close statement", "error", err)
		return nil, err
	}

	stmt, err = tdb.Prepare(attackSQL)
	if err != nil {
		cfg.LogError("Unable to prepare defending query", "error", err)
		return nil, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(defendingTerritory.Abbreviation).Scan(&defending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		cfg.LogError("Unable to get defending army size", "error", err)
		return nil, err
	}
	if defending == 0 {
		err = fmt.Errorf("no armies in %s", defendingTerritory.Name)
		cfg.LogError("No armies to attack in destination territory", "destination", defendingTerritory.Name)
		return nil, err
	}

	x, losses, err := attackCalculation(attacking, defending)
	if err != nil {
		cfg.LogError("Attack calculation failed", "error", err)
		return nil, err
	}

	var attackerLosses, defenderLosses int
	var nationRemoved bool
	if losses > 0 {
		// defending armies destroyed
		defenderLosses = int(math.Min(losses, float64(defending)))
		nationRemoved, err = db.UpdateHoldingArmySize(tdb, nil, defendingTerritory.Abbreviation, defending-defenderLosses, true)
	} else {
		// attacking armies destroyed
		attackerLosses = int(math.Min(math.Abs(losses), float64(attacking)))
		nationRemoved, err = db.UpdateHoldingArmySize(tdb, nil, attackingTerritory.Abbreviation, attacking-attackerLosses, true)
	}
	if err != nil {
		cfg.LogError("Unable to update holding army size", "error", err)
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

func (aa *AttackAction) doAttackWithCounter(_ *sql.DB, _, _ *config.Territory) (ActionResult, error) {
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
