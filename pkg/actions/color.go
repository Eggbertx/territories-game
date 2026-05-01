package actions

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/mattn/go-sqlite3"
	"github.com/mazznoer/csscolorparser"
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
	action := *car.Action

	return fmt.Sprintf(colorActionResultFmt, action.User, action.Color)
}

type ColorAction struct {
	User  string
	Color string
}

func (ca *ColorAction) DoAction(tdb *sql.DB) (ActionResult, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	if err = db.ValidateUser(ca.User, tdb, cfg.LogError); err != nil {
		return nil, err
	}

	parsedColor, err := csscolorparser.Parse(ca.Color)
	if err != nil {
		cfg.LogError("Unable to parse color", "error", err)
		return nil, err
	}
	parsedColor.A = 1.0 // Ensure the color is fully opaque
	ca.Color = strings.TrimPrefix(parsedColor.Clamp().HexString(), "#")

	stmt, err := tdb.Prepare("UPDATE nations SET color = ? WHERE player = ?")
	if err != nil {
		cfg.LogError("Unable to prepare color update statement", "error", err)
		return nil, err
	}
	defer stmt.Close()
	if _, err = stmt.Exec(ca.Color, ca.User); err != nil {
		if sqlErr, ok := err.(sqlite3.Error); ok && errors.Is(sqlErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = db.ErrColorInUse
		}
		cfg.LogError("Unable to update nation color", "error", err)
		return nil, err
	}
	if err = stmt.Close(); err != nil {
		cfg.LogError("Unable to close color update statement", "error", err)
		return nil, err
	}

	return &ColorActionResult{
		actionResultBase: actionResultBase[*ColorAction]{
			Action: &ca,
			user:   ca.User,
		},
	}, nil
}

func randomColor() string {
	return fmt.Sprintf("%0.2x%0.2x%0.2x", randInt(256), randInt(256), randInt(256))
}
