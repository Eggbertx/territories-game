package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
	"github.com/mazznoer/csscolorparser"
	"github.com/rs/zerolog"
)

const (
	colorActionResultFmt = "%s changed their nation's color to %s"
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

	return fmt.Sprintf(colorActionResultFmt, action.User, action.Color)
}

type ColorAction struct {
	User   string
	Color  string
	Logger zerolog.Logger
}

func (ca *ColorAction) DoAction(db *sql.DB) (ActionResult, error) {
	err := ValidateUser(ca.User, db, ca.Logger)
	if err != nil {
		return nil, err
	}

	parsedColor, err := csscolorparser.Parse(ca.Color)
	if err != nil {
		ca.Logger.Err(err).Caller().Send()
		return nil, err
	}
	parsedColor.A = 1.0 // Ensure the color is fully opaque
	ca.Color = strings.TrimPrefix(parsedColor.Clamp().HexString(), "#")

	stmt, err := db.Prepare("UPDATE nations SET color = ? WHERE player = ?")
	if err != nil {
		ca.Logger.Err(err).Caller().Msg("Unable to prepare color update statement")
		return nil, err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(ca.Color, ca.User); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrColorInUse
		}
		ca.Logger.Err(err).Caller().Msg("Unable to update nation color")
		return nil, err
	}
	if err = stmt.Close(); err != nil {
		ca.Logger.Err(err).Caller().Msg("Unable to close color update statement")
		return nil, err
	}

	var result ColorActionResult
	result.action = &ca
	result.user = ca.User
	ca.Logger.Info().Msg(result.String())
	return &result, nil
}

func randomColor() string {
	return fmt.Sprintf("%0.2x%0.2x%0.2x", randInt(256), randInt(256), randInt(256))
}
