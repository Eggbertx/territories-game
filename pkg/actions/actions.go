package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/mattn/go-sqlite3"
	"github.com/mazznoer/csscolorparser"
	"github.com/rs/zerolog"
)

var (
	ErrInvalidAction            = errors.New(`action must be join, move, or attack`)
	ErrMissingUser              = errors.New("unset user string")
	ErrUserNotRegistered        = errors.New("user is not registered in the game")
	ErrNoCountryName            = errors.New("missing country name from subject")
	ErrNoTargetState            = errors.New("missing target state name or abbreviation")
	ErrPlayerAlreadyJoined      = errors.New("the player already joined")
	ErrNationAlreadyJoined      = errors.New("a nation with the given name already exists")
	ErrTerritoryAlreadyOccupied = errors.New("the territory is already occupied")
	ErrColorInUse               = errors.New("color already in use by another player")
	testInt                     *int // for testing purposes, to avoid random number generation in tests
)

type GameEvent struct {
	User      string
	Action    string
	Subject   string
	Predicate string

	logger zerolog.Logger
}

func (ge *GameEvent) DoAction(db *sql.DB) error {
	var err error
	ge.logger, err = config.GetLogger()
	if err != nil {
		ge.logger = zerolog.New(os.Stdout)
		ge.logger.Err(err).Msg("Unable to get logger")
		return err
	}

	if ge.User == "" {
		ge.logger.Err(ErrMissingUser).Caller().Send()
		return ErrMissingUser
	}
	switch ge.Action {
	case "join":
		err = ge.doJoin(db, ge.Subject, ge.Predicate)
	case "color":
		if err = ge.validateUser(db); err != nil {
			ge.logger.Err(err).Caller().Send()
			return err
		}
		err = ge.doColor(db, ge.Subject)
	case "move":
		if err = ge.validateUser(db); err != nil {
			ge.logger.Err(err).Caller().Send()
			return err
		}
		sourceTerritoryStr := ge.Subject
		colonIndex := strings.LastIndex(ge.Subject, ":")
		var moveArmies int
		if colonIndex > 0 {
			sourceTerritoryStr = ge.Subject[:colonIndex]
			moveArmies, err = strconv.Atoi(ge.Subject[colonIndex+1:])
			if err != nil {
				ge.logger.Err(err).Caller().Msg("Invalid move armies count")
				return err
			}
		}
		err = ge.doMove(db, sourceTerritoryStr, moveArmies, ge.Predicate)
	case "attack":
		if err = ge.validateUser(db); err != nil {
			ge.logger.Err(err).Caller().Send()
			return err
		}
		err = ge.doAttack(db, ge.Subject, ge.Predicate)
	case "raise":
		if err = ge.validateUser(db); err != nil {
			ge.logger.Err(err).Caller().Send()
			return err
		}
		err = ge.doRaise(db, ge.Subject)
	default:
		err = ErrInvalidAction
		ge.logger.Err(err).Caller().Send()
	}
	return err
}

func (ge *GameEvent) validateUser(db *sql.DB) error {
	var countryName string
	stmt, err := db.Prepare("SELECT country_name FROM nations WHERE player = ?")
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to prepare user check statement")
		return err
	}
	defer stmt.Close()

	if err = stmt.QueryRow(ge.User).Scan(&countryName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ge.logger.Err(ErrUserNotRegistered).Caller().Send()
			return ErrUserNotRegistered
		}
		ge.logger.Err(err).Caller().Msg("Unable to check if user is registered")
		return err
	}

	if err = stmt.Close(); err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to close user check statement")
		return err
	}
	return nil
}

func (ge *GameEvent) doColor(db *sql.DB, color string) error {
	errEv := ge.logger.Err(nil)
	infoEv := ge.logger.Info()
	config.LogString("color", color, infoEv, errEv)
	defer config.DiscardLogEvents(infoEv, errEv)

	parsedColor, err := csscolorparser.Parse(color)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}
	parsedColor.A = 1.0 // Ensure the color is fully opaque
	color = strings.TrimPrefix(parsedColor.Clamp().HexString(), "#")

	stmt, err := db.Prepare("UPDATE nations SET color = ? WHERE player = ?")
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to prepare color update statement")
		return err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(color, ge.User); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrColorInUse
		}
		errEv.Err(err).Caller().Msg("Unable to update nation color")
		return err
	}
	if err = stmt.Close(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to close color update statement")
		return err
	}
	infoEv.Msg("Updated player color")
	return nil
}

func (ge *GameEvent) doJoin(db *sql.DB, nation string, territory string) error {
	cfg, _ := config.GetConfig() // if we got this far, it's safe to assume we can get the config

	infoEv := ge.logger.Info()
	errEv := ge.logger.Err(nil)
	config.LogString("nation", nation, infoEv, errEv)
	config.LogString("territory", territory, infoEv, errEv)
	defer config.DiscardLogEvents(infoEv, errEv)

	if nation == "" {
		nation = fmt.Sprintf("%s's Nation", ge.User)
	}
	if territory == "" {
		ge.logger.Err(ErrNoTargetState).Caller().Send()
		return ErrNoTargetState
	}
	joinTerritory, err := cfg.ResolveTerritory(territory)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to begin transaction")
		return err
	}
	defer tx.Rollback()

	const userAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE player = ?`
	const nationAlreadyJoinedSQL = `SELECT COUNT(*) FROM nations WHERE country_name = ?`
	const nationAddSQL = `INSERT INTO nations (country_name,player, color) VALUES(?,?,?)`
	const nationInitialHolding = `INSERT INTO holdings (nation_id, territory, army_size) VALUES(
		(SELECT id FROM nations WHERE country_name = ?),
		?, 1)`
	var numPlayerMatches int
	var numNationMatches int
	if err = tx.QueryRow(userAlreadyJoinedSQL, ge.User).Scan(&numPlayerMatches); err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}
	if numPlayerMatches > 0 {
		ge.logger.Err(ErrPlayerAlreadyJoined).Caller().Send()
		return ErrPlayerAlreadyJoined
	}

	if err = tx.QueryRow(nationAlreadyJoinedSQL, nation).Scan(&numNationMatches); err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}
	if numNationMatches > 0 {
		ge.logger.Err(ErrNationAlreadyJoined).Caller().Send()
		return ErrNationAlreadyJoined
	}

	if _, err = tx.Exec(nationAddSQL, nation, ge.User, randomColor()); err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to add nation")
		return err
	}
	if _, err = tx.Exec(nationInitialHolding, nation, territory); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrTerritoryAlreadyOccupied
		}
		ge.logger.Err(err).Caller().Msg("Unable to add initial holding")
		return err
	}
	if err = tx.Commit(); err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to commit transaction")
		return err
	}
	ge.logger.Info().Str("territory", joinTerritory.Name).Msg("Added new user")
	return nil
}

func (ge *GameEvent) doAttack(db *sql.DB, attacker, defender string) error {
	cfg, _ := config.GetConfig()

	attackingTerritory, err := cfg.ResolveTerritory(attacker)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}

	defendingTerritory, err := cfg.ResolveTerritory(defender)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}

	if attackingTerritory.Abbreviation == defendingTerritory.Abbreviation {
		err = fmt.Errorf("cannot attack %s from %s: friendly fire not allowed", defendingTerritory.Name, attackingTerritory.Name)
		ge.logger.Err(err).Caller().Send()
		return err
	}

	neighbors, err := attackingTerritory.IsNeighboring(defender)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
		return err
	}
	if !neighbors {
		err = fmt.Errorf("cannot attack %s from %s: not a neighboring territory", defendingTerritory.Name, attackingTerritory.Name)
		ge.logger.Err(err).Caller().Send()
		return err
	}

	if cfg.DoCounterattack {
		return ge.doAttackWithCounter(db, attackingTerritory, defendingTerritory)
	}
	return ge.doNormalAttack(db, attackingTerritory, defendingTerritory)
}

// doNormalAttack calculates the result and losses of a normal attack, based on a dice roll and the difference in army sizes
func (ge *GameEvent) doNormalAttack(db *sql.DB, attackingFrom, defendingFrom *config.Territory) error {
	infoEv := ge.logger.Info()
	errEv := ge.logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)
	config.LogString("attackingTerritory", attackingFrom.Name, infoEv, errEv)
	config.LogString("defendingTerritory", defendingFrom.Name, infoEv, errEv)

	var attacking, defending int
	const attackSQL = `SELECT army_size FROM v_nation_holdings WHERE territory = ?`
	stmt, err := db.Prepare(attackSQL + "  AND player = ?")
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to prepare attack query")
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(attackingFrom.Abbreviation, ge.User).Scan(&attacking)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ge.logger.Err(err).Caller().Msg("Unable to get attacking army size")
		return err
	}
	if attacking == 0 {
		err = fmt.Errorf("no armies in %s controlled by %s to attack with", attackingFrom.Name, ge.User)
		ge.logger.Err(err).Caller().Send()
		return err
	}

	if err = stmt.Close(); err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to close statement")
		return err
	}

	stmt, err = db.Prepare(attackSQL)
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to prepare defending query")
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(defendingFrom.Abbreviation).Scan(&defending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ge.logger.Err(err).Caller().Msg("Unable to get defending army size")
		return err
	}
	if defending == 0 {
		err = fmt.Errorf("no armies in %s", defendingFrom.Name)
		ge.logger.Err(err).Caller().Send()
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
		err = ge.updateHoldingArmySize(db, nil, defendingFrom.Abbreviation, defending-defenderLosses, true, infoEv, errEv)
	} else {
		// attacking armies destroyed
		attackerLosses = int(math.Min(math.Abs(losses), float64(attacking)))
		config.LogInt("attackerLosses", attackerLosses, infoEv, errEv)
		err = ge.updateHoldingArmySize(db, nil, attackingFrom.Abbreviation, attacking-attackerLosses, true, infoEv, errEv)
	}
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to update holding army size")
	}

	return err
}

// doAttackWithCounter handles an attack where the defender automatically counterattacks, based on formulas reverse engineered
// from Advance Wars.
func (ge *GameEvent) doAttackWithCounter(db *sql.DB, attackingFrom, defendingFrom *config.Territory) error {
	// do attack and defender automatically counterattacks using damage calculation from Advance Wars
	err := errors.New("attack with counterattack not implemented yet")
	ge.logger.Err(err).Caller().Send()
	return err
}

func (ge *GameEvent) doMove(db *sql.DB, sourceTerritoryStr string, moveArmies int, destTerritoryStr string) error {
	infoEv := ge.logger.Info()
	errEv := ge.logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)
	config.LogString("sourceTerritory", sourceTerritoryStr, infoEv, errEv)
	config.LogString("destTerritory", destTerritoryStr, infoEv, errEv)
	if moveArmies > 0 {
		config.LogInt("moveArmies", moveArmies, infoEv, errEv)
	}

	if destTerritoryStr == "" || sourceTerritoryStr == destTerritoryStr {
		errEv.Err(ErrNoTargetState).Caller().Send()
		return ErrNoTargetState
	}

	cfg, err := config.GetConfig()
	if err != nil {
		errEv.Caller().Msg("Unable to get configuration")
		return err
	}

	sourceTerritory, err := cfg.ResolveTerritory(sourceTerritoryStr)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}
	destTerritory, err := cfg.ResolveTerritory(destTerritoryStr)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}

	isNeighboring, err := sourceTerritory.IsNeighboring(destTerritoryStr)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}
	if !isNeighboring {
		err = fmt.Errorf("cannot move from %s to %s: not a neighboring territory", sourceTerritory.Name, destTerritory.Name)
		errEv.Err(err).Caller().Send()
	}

	var armiesInSourceTerritory, armiesInDestTerritory int
	var fromPlayer, destinationPlayer string
	const moveSQL = "SELECT army_size, player FROM v_nation_holdings WHERE territory = ?"
	stmt, err := db.Prepare(moveSQL)
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to prepare move query")
		return err
	}
	defer stmt.Close()
	err = stmt.QueryRow(sourceTerritory.Abbreviation).Scan(&armiesInSourceTerritory, &fromPlayer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to move", sourceTerritory.Name, ge.User)
		}
		errEv.Err(err).Caller().Send()
		return err
	}

	if moveArmies > 0 && moveArmies > armiesInSourceTerritory {
		err = fmt.Errorf("cannot move %d armies from %s: only %d available", moveArmies, sourceTerritory.Name, armiesInSourceTerritory)
		errEv.Err(err).Caller().Send()
		return err
	}
	if moveArmies <= 0 {
		moveArmies = armiesInSourceTerritory // none specified, move all armies in source territory
		config.LogInt("moveArmies", moveArmies, infoEv, errEv)
	}

	err = stmt.QueryRow(destTerritory.Abbreviation).Scan(&armiesInDestTerritory, &destinationPlayer)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		errEv.Err(err).Caller().Send()
		return err
	}

	if fromPlayer != ge.User {
		err = fmt.Errorf("cannot move from %s: no armies controlled by player", sourceTerritory.Name)
		errEv.Err(err).Caller().Send()
		return err
	}

	if armiesInDestTerritory > 0 && destinationPlayer != ge.User {
		err = ErrTerritoryAlreadyOccupied
		errEv.Err(err).Caller().Send()
		return err
	}

	if armiesInDestTerritory+moveArmies > cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot move %d armies to %s: would exceed maximum of %d", moveArmies, destTerritory.Name, cfg.MaxArmiesPerTerritory)
		errEv.Err(err).Caller().Send()
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to begin transaction")
		return err
	}
	defer tx.Rollback()

	if err = ge.updateHoldingArmySize(db, tx, sourceTerritory.Abbreviation, armiesInSourceTerritory-moveArmies, false, infoEv, errEv); err != nil {
		return err
	}
	if armiesInDestTerritory == 0 {
		stmt, err := tx.Prepare(`INSERT INTO holdings (nation_id, territory, army_size) VALUES(
			(SELECT id FROM nations WHERE player = ?),
			?, ?)`)
		if err != nil {
			errEv.Err(err).Caller().Msg("Unable to prepare insert holding statement")
			return err
		}
		defer stmt.Close()
		if _, err = stmt.Exec(ge.User, destTerritory.Abbreviation, moveArmies); err != nil {
			errEv.Err(err).Caller().Msg("Unable to insert new holding")
			return err
		}
	} else {
		if err = ge.updateHoldingArmySize(db, tx, destTerritory.Abbreviation, armiesInDestTerritory+moveArmies, false, infoEv, errEv); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to commit transaction")
		return err
	}
	infoEv.Msg("Moved armies")

	return nil
}

func (ge *GameEvent) doRaise(db *sql.DB, territoryStr string) error {
	infoEv := ge.logger.Info()
	errEv := ge.logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)

	config.LogString("territory", territoryStr, infoEv, errEv)
	if territoryStr == "" {
		ge.logger.Err(ErrNoTargetState).Caller().Send()
		return ErrNoTargetState
	}

	cfg, err := config.GetConfig()
	if err != nil {
		errEv.Caller().Msg("Unable to get configuration")
		return err
	}

	territory, err := cfg.ResolveTerritory(territoryStr)
	if err != nil {
		errEv.Err(err).Caller().Send()
		return err
	}

	stmt, err := db.Prepare(`SELECT army_size FROM v_nation_holdings WHERE territory = ? and player = ?`)
	if err != nil {
		ge.logger.Err(err).Caller().Msg("Unable to prepare raise check statement")
		return err
	}
	defer stmt.Close()

	var armySize int
	if err = stmt.QueryRow(territory.Abbreviation, ge.User).Scan(&armySize); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no armies in %s controlled by %s to raise", territory.Name, ge.User)
		}
		ge.logger.Err(err).Caller().Send()
		return err
	}

	if armySize == cfg.MaxArmiesPerTerritory {
		err = fmt.Errorf("cannot raise army size in %s: already at maximum of %d", territory.Name, cfg.MaxArmiesPerTerritory)
		ge.logger.Err(err).Caller().Send()
		return err
	}

	if err = ge.updateHoldingArmySize(db, nil, territory.Abbreviation, armySize+1, false, infoEv, errEv); err != nil {
		return err
	}

	infoEv.Msg("Raised army size")
	return nil
}

func (ge *GameEvent) numTerritories(db *sql.DB, tx *sql.Tx, territory string, errEv *zerolog.Event) (int, error) {
	const territoriesLeftSQL = `SELECT COUNT(*) FROM v_nation_holdings WHERE player = (select player from v_nation_holdings where territory = ?)`
	var stmt *sql.Stmt
	var err error
	if tx == nil {
		stmt, err = db.Prepare(territoriesLeftSQL)
	} else {
		stmt, err = tx.Prepare(territoriesLeftSQL)
	}
	if err != nil {
		errEv.Err(err).Caller().Send()
		return 0, err
	}
	defer stmt.Close()

	var count int
	if err = stmt.QueryRow(ge.User).Scan(&count); err != nil {
		errEv.Err(err).Caller().Msg("Unable to check if user has territories left")
		return 0, err
	}
	return count, nil
}

func (ge *GameEvent) updateHoldingArmySize(db *sql.DB, tx *sql.Tx, territory string, size int, deleteNationIfNoTerritories bool, infoEv, errEv *zerolog.Event) error {
	const updateSizeSQL = `UPDATE holdings SET army_size = ? WHERE territory = ?`
	const destroyedSQL = `DELETE FROM holdings WHERE territory = ?`
	var stmt *sql.Stmt
	var err error
	shouldCommit := tx == nil
	if tx == nil {
		tx, err = db.Begin()
		if err != nil {
			errEv.Err(err).Caller().Msg("Unable to begin transaction")
			return err
		}
		defer tx.Rollback()
	}
	if size > 0 {
		if stmt, err = tx.Prepare(updateSizeSQL); err != nil {
			errEv.Err(err).Caller().Msg("Unable to prepare update holding army size statement")
			return err
		}
		defer stmt.Close()
		stmt.Exec(size, territory)
	} else {
		if stmt, err = tx.Prepare(destroyedSQL); err != nil {
			errEv.Err(err).Caller().Msg("Unable to prepare delete holding statement")
			return err
		}
		defer stmt.Close()
		stmt.Exec(territory)
	}
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to update holding army size")
	}
	if err = stmt.Close(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to close update holding statement")
		return err
	}

	if size == 0 && deleteNationIfNoTerritories {
		territoryCount, err := ge.numTerritories(db, tx, ge.Predicate, errEv)
		if err != nil {
			return err
		}
		if territoryCount == 0 {
			stmt, err := db.Prepare("SELECT id FROM nations WHERE player = (SELECT player FROM v_nation_holdings WHERE territory = ?)")
			if err != nil {
				errEv.Err(err).Caller().Msg("Unable to prepare get nation ID statement")
				return err
			}
			defer stmt.Close()
			var nationID int
			if err = stmt.QueryRow(ge.Predicate).Scan(&nationID); err != nil {
				errEv.Err(err).Caller().Msg("Unable to get nation ID for deletion")
				return err
			}

			if stmt, err = tx.Prepare(`DELETE FROM nations WHERE id = ?`); err != nil {
				errEv.Err(err).Caller().Msg("Unable to prepare delete nation statement")
				return err
			}
			defer stmt.Close()
			if _, err = stmt.Exec(nationID); err != nil {
				errEv.Err(err).Caller().Msg("Unable to delete nation")
				return err
			}
			if err = stmt.Close(); err != nil {
				errEv.Err(err).Caller().Msg("Unable to close delete nation statement")
				return err
			}
			infoEv.Msg("User has no territories left, nation deleted")
		}
	}

	if shouldCommit {
		if err = tx.Commit(); err != nil {
			errEv.Err(err).Caller().Msg("Unable to commit transaction")
			return err
		}
	}

	return err
}

func randInt(max int) int {
	if testInt != nil {
		return *testInt % max
	}
	return rand.Intn(max)
}

func randomColor() string {
	return fmt.Sprintf("%0.2x%0.2x%0.2x", randInt(256), randInt(256), randInt(256))
}
