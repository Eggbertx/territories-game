package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

var (
	ErrPlayerAlreadyJoined = errors.New("the player already joined")
	ErrNationAlreadyJoined = errors.New("a nation with the given name already exists")
	ErrMissingUser         = errors.New("unset user string")
	ErrUserNotRegistered   = errors.New("user is not registered in the game")
	ErrColorInUse          = errors.New("color already in use by another player")
)

// EnoughPlayersToStart checks if there are enough players to start the game based on the configured minimum number of nations.
func EnoughPlayersToStart(tx *sql.Tx) (bool, int, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return false, 0, err
	}

	shouldCommit := tx == nil
	if shouldCommit {
		db, err := GetDB()
		if err != nil {
			return false, 0, err
		}
		tx, err = db.Begin()
		if err != nil {
			return false, 0, err
		}
		defer tx.Rollback()
	}

	if cfg.MinimumNationsToStart <= 1 {
		return true, 0, nil // No minimum nations required to start
	}

	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM nations").Scan(&count); err != nil {
		return false, 0, err
	}
	if shouldCommit {
		if err = tx.Commit(); err != nil {
			return false, count, err
		}
	}
	return count >= cfg.MinimumNationsToStart, count, nil
}

// ValidateUser checks if the user is registered in the game by querying the nations table
func ValidateUser(user string, tdb *sql.DB, logger zerolog.Logger) error {
	if user == "" {
		logger.Err(ErrMissingUser).Caller().Send()
		return ErrMissingUser
	}

	var countryName string
	stmt, err := tdb.Prepare("SELECT country_name FROM nations WHERE player = ?")
	if err != nil {
		logger.Err(err).Caller().Msg("Unable to prepare user check statement")
		return err
	}
	defer stmt.Close()

	if err = stmt.QueryRow(user).Scan(&countryName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Err(ErrUserNotRegistered).Caller().Send()
			return ErrUserNotRegistered
		}
		logger.Err(err).Caller().Msg("Unable to check if user is registered")
		return err
	}

	if err = stmt.Close(); err != nil {
		logger.Err(err).Caller().Msg("Unable to close user check statement")
		return err
	}
	return nil
}

// PlayerHoldings returns the number of territories a player currently holds
func PlayerHoldings(db *sql.DB, tx *sql.Tx, player string, logger zerolog.Logger) (int, error) {
	const territoriesLeftSQL = `SELECT COUNT(*) FROM v_nation_holdings WHERE player = ?`
	var stmt *sql.Stmt
	var err error
	if tx == nil {
		stmt, err = db.Prepare(territoriesLeftSQL)
	} else {
		stmt, err = tx.Prepare(territoriesLeftSQL)
	}
	if err != nil {
		logger.Err(err).Caller().Send()
		return 0, err
	}
	defer stmt.Close()

	var count int
	if err = stmt.QueryRow(player).Scan(&count); err != nil {
		logger.Err(err).Caller().Msg("Unable to check if user has territories left")
		return 0, err
	}
	return count, nil
}

// UpdateHoldingArmySize updates the army size of a holding in the database. If deleteNationIfNoTerritories is true and the size is 0,
// it will remove the nation from play if it has no remaining territories.
func UpdateHoldingArmySize(db *sql.DB, tx *sql.Tx, territory string, size int, deleteNationIfNoTerritories bool, logger zerolog.Logger) (bool, error) {
	var stmt *sql.Stmt
	var err error
	shouldCommit := tx == nil
	if tx == nil {
		tx, err = db.Begin()
		if err != nil {
			logger.Err(err).Caller().Msg("Unable to begin transaction")
			return false, err
		}
		defer tx.Rollback()
	}

	stmt, err = tx.Prepare("SELECT player FROM v_nation_holdings WHERE territory = ?")
	if err != nil {
		logger.Err(err).Caller().Msg("Unable to prepare get defending nation statement")
		return false, err
	}
	defer stmt.Close()
	var defendingNationPlayer string
	if err = stmt.QueryRow(territory).Scan(&defendingNationPlayer); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("no defending nation found for territory %s", territory)
			logger.Err(err).Caller().Send()
		}
		logger.Err(err).Caller().Msg("Unable to get territory nation")
		return false, err
	}
	if err = stmt.Close(); err != nil {
		logger.Err(err).Caller().Msg("Unable to close get nation statement")
		return false, err
	}

	if size > 0 {
		if stmt, err = tx.Prepare("UPDATE holdings SET army_size = ? WHERE territory = ?"); err != nil {
			logger.Err(err).Caller().Msg("Unable to prepare update holding army size statement")
			return false, err
		}
		defer stmt.Close()
		stmt.Exec(size, territory)
	} else {
		if stmt, err = tx.Prepare("DELETE FROM holdings WHERE territory = ?"); err != nil {
			logger.Err(err).Caller().Msg("Unable to prepare delete holding statement")
			return false, err
		}
		defer stmt.Close()
		stmt.Exec(territory)
	}
	if err != nil {
		logger.Err(err).Caller().Msg("Unable to update holding army size")
	}
	if err = stmt.Close(); err != nil {
		logger.Err(err).Caller().Msg("Unable to close update holding statement")
		return false, err
	}

	var nationRemoved bool
	if size == 0 && deleteNationIfNoTerritories {
		territoryCount, err := PlayerHoldings(db, tx, defendingNationPlayer, logger)
		if err != nil {
			return false, err
		}
		if territoryCount == 0 {
			if stmt, err = tx.Prepare(`DELETE FROM nations WHERE player = ?`); err != nil {
				logger.Err(err).Caller().Msg("Unable to prepare delete nation statement")
				return false, err
			}
			defer stmt.Close()
			if _, err = stmt.Exec(defendingNationPlayer); err != nil {
				logger.Err(err).Caller().Msg("Unable to delete nation")
				return false, err
			}
			if err = stmt.Close(); err != nil {
				logger.Err(err).Caller().Msg("Unable to close delete nation statement")
				return false, err
			}
			logger.Info().Msgf("Player %s has no territories left, nation removed from play", defendingNationPlayer)
			nationRemoved = true
		}
	}

	if shouldCommit {
		if err = tx.Commit(); err != nil {
			logger.Err(err).Caller().Msg("Unable to commit transaction")
			return false, err
		}
	}

	return nationRemoved, err
}

// SQLite3Timestamp is used to represent timestamps in SQLite3 format that may scan into a time.Time, or a
// string representation of a timestamp.
// It implements the sql.Scanner interface to allow scanning from database rows.
type SQLite3Timestamp sql.NullTime

func (t *SQLite3Timestamp) Scan(value any) error {
	if value == nil {
		t.Time = time.Time{}
		t.Valid = false
		return nil
	}
	var nt sql.NullTime
	if err := nt.Scan(&value); err == nil {
		t.Time = nt.Time
		t.Valid = nt.Valid
		return nil
	}
	var timestampStr string
	switch v := value.(type) {
	case []byte:
		timestampStr = string(v)
	case string:
		timestampStr = v
	default:
		return fmt.Errorf("cannot scan type %T into SQLite3Timestamp: %v", value, value)
	}

	err := t.parseSQLite3Timestamp(timestampStr)
	if err != nil {
		return err
	}
	t.Valid = true
	return nil
}

// parseSQLite3Timestamp attempts to parse a given string as a timestamp
func (t *SQLite3Timestamp) parseSQLite3Timestamp(ts string) (err error) {
	for _, format := range sqlite3.SQLiteTimestampFormats {
		t.Time, err = time.Parse(format, ts)
		if err == nil {
			return nil
		}
	}

	return &time.ParseError{Value: ts, Message: ": invalid sqlite3 timestamp"}
}
