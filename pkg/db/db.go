package db

import (
	"database/sql"
	"net"

	"github.com/Eggbertx/territories-game/pkg/config"
	_ "github.com/mattn/go-sqlite3"

	_ "embed"
)

var (
	db *sql.DB

	//go:embed provision.sql
	provisionStr string
)

type HoldingRecord struct {
	HoldingID   int
	NationID    int
	Territory   string
	ArmySize    int
	Color       string
	CountryName string
	Player      string
}

func openDB() (*sql.DB, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	db, err = sql.Open("sqlite3", cfg.DBFile)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func ProvisionDB(tdb *sql.DB) error {
	if tdb == nil {
		return net.ErrClosed
	}
	_, err := tdb.Exec(provisionStr)
	return err
}

func GetDB() (*sql.DB, error) {
	var err error
	if db == nil {
		db, err = openDB()
		if err != nil {
			return nil, err
		}
		if err = ProvisionDB(db); err != nil {
			db.Close()
			return nil, err
		}
	}

	return db, nil
}

func CloseDB() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}
