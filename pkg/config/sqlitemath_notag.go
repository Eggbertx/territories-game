//go:build !sqlite_math_functions
// +build !sqlite_math_functions

package config

import "errors"

const (
	HasSQLiteMathFunctions = false
)

var (
	errNoSQLiteMathFunctionsError error = errors.New("this build does not support SQLite math functions (built without sqlite_math_functions tag)")
)
