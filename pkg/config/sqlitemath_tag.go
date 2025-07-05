//go:build sqlite_math_functions
// +build sqlite_math_functions

package config

const (
	HasSQLiteMathFunctions = true
)

var (
	errNoSQLiteMathFunctionsError error = nil
)
