package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/mattn/go-sqlite3"
	"github.com/mazznoer/csscolorparser"
	"github.com/rs/zerolog"
)

type ColorActionResult struct {
	actionResultBase[*ColorAction]
}

func (car *ColorActionResult) ActionType() string {
	return "color"
}

func (car *ColorActionResult) String() string {
	str := car.actionResultBase.String()
	if str != "" {
		return str
	}
	action := *car.action

	return fmt.Sprintf("%s changed their nation's color to %s", action.user, action.color)
}

type ColorAction struct {
	user   string
	color  string
	logger zerolog.Logger
}

func (ca *ColorAction) DoAction(db *sql.DB) (ActionResult, error) {
	err := ValidateUser(ca.user, db, ca.logger)
	if err != nil {
		return nil, err
	}

	parsedColor, err := csscolorparser.Parse(ca.color)
	if err != nil {
		ca.logger.Err(err).Caller().Send()
		return nil, err
	}
	parsedColor.A = 1.0 // Ensure the color is fully opaque
	ca.color = strings.TrimPrefix(parsedColor.Clamp().HexString(), "#")

	stmt, err := db.Prepare("UPDATE nations SET color = ? WHERE player = ?")
	if err != nil {
		ca.logger.Err(err).Caller().Msg("Unable to prepare color update statement")
		return nil, err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(ca.color, ca.user); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrColorInUse
		}
		ca.logger.Err(err).Caller().Msg("Unable to update nation color")
		return nil, err
	}
	if err = stmt.Close(); err != nil {
		ca.logger.Err(err).Caller().Msg("Unable to close color update statement")
		return nil, err
	}

	var result ColorActionResult
	result.action = &ca
	result.user = ca.user
	ca.logger.Info().Msg(result.String())
	return &result, nil
}

func randomColor() string {
	return fmt.Sprintf("%0.2x%0.2x%0.2x", randInt(256), randInt(256), randInt(256))
}

func colorActionParser(args ...string) (Action, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("color action requires at least 2 arguments: user and color")
	}
	action := &ColorAction{
		user:  args[0],
		color: args[1],
	}

	if action.user == "" {
		return nil, ErrMissingUser
	}

	if action.color == "" {
		action.color = randomColor()
	}

	var err error
	action.logger, err = config.GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	return action, nil
}
