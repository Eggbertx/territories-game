package actions

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrInvalidAction            = errors.New(`action must be join, move, or attack`)
	ErrMissingUser              = errors.New("unset user string")
	ErrUserNotRegistered        = errors.New("user is not registered in the game")
	ErrNoCountryName            = errors.New("missing country name from subject")
	ErrNoTargetTerritory        = errors.New("missing target territory name or abbreviation")
	ErrPlayerAlreadyJoined      = errors.New("the player already joined")
	ErrNationAlreadyJoined      = errors.New("a nation with the given name already exists")
	ErrTerritoryAlreadyOccupied = errors.New("the territory is already occupied")
	ErrColorInUse               = errors.New("color already in use by another player")
	testInt                     *int // for testing purposes, to avoid random number generation in tests

	registeredActionParsers map[string]ActionParser = make(map[string]ActionParser)
)

type Action interface {
	DoAction(db *sql.DB) error
}

type ActionParser func(...string) (Action, error)

func RegisterActionParser(actionType string, parser ActionParser) error {
	if _, exists := registeredActionParsers[actionType]; exists {
		return fmt.Errorf("action %s already registered", actionType)
	}
	registeredActionParsers[actionType] = parser

	return nil
}

func GetActionParser(actionType string) (ActionParser, error) {
	if parser, exists := registeredActionParsers[actionType]; exists {
		return parser, nil
	}
	return nil, fmt.Errorf("no action parser registered for %s", actionType)
}

func GetAction(actionType string, args ...string) (Action, error) {
	if parser, exists := registeredActionParsers[actionType]; exists {
		return parser(args...)
	}
	return nil, fmt.Errorf("%s is not a recognized action type", actionType)
}

func RegisteredActionParsers() []string {
	var actionTypes []string
	for actionType := range registeredActionParsers {
		actionTypes = append(actionTypes, actionType)
	}
	return actionTypes
}

func init() {
	RegisterActionParser("join", joinActionParser)
	RegisterActionParser("move", moveActionParser)
	RegisterActionParser("raise", raiseActionParser)
	RegisterActionParser("color", colorActionParser)
	RegisterActionParser("attack", attackActionParser)
}
