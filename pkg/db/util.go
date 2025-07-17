package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net"

	"github.com/Eggbertx/territories-game/pkg/config"
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
func EnoughPlayersToStart() (bool, error) {
	db, err := GetDB()
	if err != nil {
		return false, err
	}
	cfg, err := config.GetConfig()
	if err != nil {
		return false, err
	}
	if db == nil {
		return false, net.ErrClosed
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM nations").Scan(&count)
	if err != nil {
		return false, err
	}
	return count >= cfg.MinimumNationsToStart, nil
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
