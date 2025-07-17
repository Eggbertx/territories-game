package actions

import (
	"database/sql"
	"errors"

	"github.com/Eggbertx/territories-game/pkg/db"
)

var (
	ErrInvalidAction            = errors.New(`action must be join, move, or attack`)
	ErrNoTargetTerritory        = errors.New("missing target territory name or abbreviation")
	ErrTerritoryAlreadyOccupied = errors.New("the territory is already occupied")
	testInt                     int // for testing purposes, to avoid random number generation in tests
	useTestInt                  bool
)

const (
	noActionString = "no action performed"
)

type Action interface {
	DoAction(db *sql.DB) (ActionResult, error)
}

type ActionResult interface {
	ActionType() string
	User() string
	String() string
}

type actionResultBase[a Action] struct {
	Action *a
	user   string
}

func (arb *actionResultBase[a]) User() string {
	if arb.Action == nil {
		return ""
	}
	return arb.user
}

func (arb *actionResultBase[a]) String() string {
	if arb.Action == nil {
		return noActionString
	}
	if arb.user == "" {
		return db.ErrMissingUser.Error()
	}

	return ""
}
