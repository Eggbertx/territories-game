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

// Action is the interface that all in-game actions must implement
type Action interface {
	DoAction(db *sql.DB) (ActionResult, error)
}

// ActionResult is the interface returned by a successful DoAction call. A successful DoAction call
// does not guarantee that the action done was successful (e.g., an attack may fail).
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
