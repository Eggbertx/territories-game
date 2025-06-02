package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
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
)

type GameEvent struct {
	User      string
	Action    string
	Subject   string
	Predicate string

	logger zerolog.Logger
}

func NewEvent(user, action, subject, predicate string) *GameEvent {
	return &GameEvent{
		User:      user,
		Action:    action,
		Subject:   subject,
		Predicate: predicate,
	}
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
	if ge.Action != "join" {
		var user string
		if err = db.QueryRow(`SELECT player FROM nations WHERE player = ?`, ge.User).Scan(&user); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ge.logger.Err(ErrUserNotRegistered).Caller().Send()
				return ErrUserNotRegistered
			}
			ge.logger.Err(err).Caller().Msg("Unable to check if user is registered")
			return err
		}
	}
	switch ge.Action {
	case "join":
		err = ge.doJoin(db, ge.Subject, ge.Predicate)
	case "color":
		err = ge.doColor(db, ge.Subject)
	case "move":
		err = errors.New("move action not implemented yet")
		ge.logger.Err(err).Caller().Send()
	case "attack":
		err = ge.doAttack(db, ge.Subject, ge.Predicate)
	case "raise":
		err = errors.New("raise action not implemented yet")
		ge.logger.Err(err).Caller().Send()
	default:
		err = ErrInvalidAction
		ge.logger.Err(err).Caller().Send()
	}
	return err
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

	const colorInUseSQL = `SELECT COUNT(*) FROM nations WHERE color = ? AND player <> ?`
	const colorUpdateSQL = `UPDATE nations SET color = ? WHERE player = ?`
	stmt, err := db.Prepare(colorInUseSQL)
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to prepare color update statement")
		return err
	}
	defer stmt.Close()

	var count int
	if err = stmt.QueryRow(color, ge.User).Scan(&count); err != nil {
		errEv.Err(err).Caller().Msg("Unable to check if color is in use")
		return err
	}
	if count > 0 {
		errEv.Err(fmt.Errorf("color %q is already in use by another player", color)).Caller().Send()
		return errors.New("color already in use by another player")
	}
	if err = stmt.Close(); err != nil {
		errEv.Err(err).Caller().Msg("Unable to close color update statement")
		return err
	}

	if stmt, err = db.Prepare(colorUpdateSQL); err != nil {
		errEv.Err(err).Caller().Msg("Unable to prepare color update statement")
		return err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(color, ge.User); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = errors.New("color already in use by another player")
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
	if cfg.DoCounterattack {
		return ge.doAttackWithCounter(db, attacker, defender)
	}
	return ge.doNormalAttack(db, attacker, defender)

}

func (ge *GameEvent) doNormalAttack(db *sql.DB, attackingFrom, defendingFrom string) error {
	infoEv := ge.logger.Info()
	errEv := ge.logger.Err(nil)
	defer config.DiscardLogEvents(infoEv, errEv)
	cfg, _ := config.GetConfig()
	config.LogString("attackingTerritory", attackingFrom, infoEv, errEv)
	config.LogString("defendingTerritory", defendingFrom, infoEv, errEv)

	attackingTerritory, err := cfg.ResolveTerritory(attackingFrom)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
	}
	defendingTerritory, err := cfg.ResolveTerritory(defendingFrom)
	if err != nil {
		ge.logger.Err(err).Caller().Send()
	}

	var attacking, defending int
	const attackSQL = `SELECT army_size FROM holdings WHERE territory = ?`
	err = db.QueryRow(attackSQL+"  AND player = ?", attackingTerritory.Abbreviation, ge.User).Scan(&attacking)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ge.logger.Err(err).Caller().Msg("Unable to get attacking army size")
		return err
	}
	if attacking == 0 {
		err = fmt.Errorf("no armies in %q controlled by %q to attack with", attackingTerritory.Name, ge.User)
		ge.logger.Err(err).Caller().Send()
	}

	err = db.QueryRow(attackSQL, defendingTerritory.Abbreviation).Scan(&defending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ge.logger.Err(err).Caller().Msg("Unable to get defending army size")
		return err
	}
	if defending == 0 {
		err = fmt.Errorf("no armies in %s", defendingTerritory.Name)
		ge.logger.Err(err).Caller().Send()
	}

	x := rand.Intn(20) + 1

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
		infoEv.Int("defenderLosses", defenderLosses)

	} else {
		// attacking armies destroyed
		attackerLosses = int(math.Min(math.Abs(losses), float64(attacking)))
		infoEv.Int("attackerLosses", attackerLosses)
	}

	return err
}

func (ge *GameEvent) doAttackWithCounter(db *sql.DB, attackingFrom, defendingFrom string) error {
	// do attack and defender automatically counterattacks using damage calculation from Advance Wars
	err := errors.New("attack with counterattack not implemented yet")
	ge.logger.Err(err).Caller().Send()
	return err
}

func (ge *GameEvent) updateHoldingArmySize(db *sql.DB, territory string, size int, infoEv, errEv *zerolog.Event) error {
	const updateSizeSQL = `UPDATE holdings SET army_size = ? WHERE territory = ?`
	const destroyedSQL = `DELETE FROM holdings WHERE territory = ?`
	var err error
	if size > 0 {
		_, err = db.Exec(updateSizeSQL, size, territory)
	} else {
		_, err = db.Exec(destroyedSQL, territory)
	}
	if err != nil {
		errEv.Err(err).Caller().Msg("Unable to update holding army size")
	}
	return err
}

func randomColor() string {
	return fmt.Sprintf("%0.2x%0.2x%0.2x", rand.Intn(256), rand.Intn(256), rand.Intn(256))
}
