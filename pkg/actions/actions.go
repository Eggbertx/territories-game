package actions

import (
	"database/sql"
	"errors"
)

var (
	ErrInvalidAction            = errors.New(`action must be join, move, or attack`)
	ErrMissingUser              = errors.New("unset user string")
	ErrUserNotRegistered        = errors.New("user is not registered in the game")
	ErrNoTargetTerritory        = errors.New("missing target territory name or abbreviation")
	ErrPlayerAlreadyJoined      = errors.New("the player already joined")
	ErrNationAlreadyJoined      = errors.New("a nation with the given name already exists")
	ErrTerritoryAlreadyOccupied = errors.New("the territory is already occupied")
	ErrColorInUse               = errors.New("color already in use by another player")
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
		return ErrMissingUser.Error()
	}

	return ""
}
