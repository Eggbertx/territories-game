package config

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/Eggbertx/durationutil"
)

const (
	defaultMaxArmiesPerTerritory         = 5
	defaultInitialArmies                 = 3
	defaultMinimumNationsToStart         = 2
	defaultActionsPerTurnHoldingsDivisor = 3.0
)

var (
	cfg                  *Config
	ErrGameNotConfigured = fmt.Errorf("no active configuration has been set")
)

func noopLoggerFunc(string, ...any) {}

type LoggerFunc func(string, ...any)

type Config struct {
	// MapFile is the path to the SVG input file
	MapFile string `json:"mapFile"`

	// DBFile is the path to the SQLite database file
	DBFile string `json:"dbFile"`

	// LogInfo is a function that can be used to send information level events to the log. It is assumed that it will treat arguments the
	// same as they are treated by slog.Logger.Log
	LogInfo LoggerFunc `json:"-"`

	// LogError is a function that can be used to send error level events to the log. It is assumed that it will treat arguments the
	// same as they are treated by slog.Logger.Log
	LogError LoggerFunc `json:"-"`

	// SVGOutFile is the path to the SVG output file with the current nations/players and territory holdings
	SVGOutFile string `json:"svgOutFile"`

	// PNGOutFile is the path to the exported PNG output file generated from the SVG output file
	PNGOutFile string `json:"pngOutFile"`

	// DoCounterattack will eventually be used to determine if a defending territory automatically counterattacks
	DoCounterattack bool `json:"doCounterattack"`

	// InitialArmies is the number of armies each player starts with in their initial territory.
	InitialArmies int `json:"initialArmies"`

	// MinimumNationsToStart is the minimum number of nations required before players can start taking turns, aside from color
	MinimumNationsToStart int `json:"minimumNationsToStart"`

	// MaxArmiesPerTerritory is the maximum number of armies that can be moved into or raised in a territory.
	MaxArmiesPerTerritory int `json:"maxArmiesPerTerritory"`

	// UnclaimedTerritoriesHave1Army indicates whether unclaimed territories are treated as having 1 army to destroy
	// before a player can claim them.
	UnclaimedTerritoriesHave1Army bool `json:"unclaimedTerritoriesHave1Army"`

	// ActionsPerTurnHoldingsDivisor is used to determine how many actions a player can take per turn.
	// A player can take ceil(holdings / ActionsPerTurnHoldingsDivisor) actions per turn.
	ActionsPerTurnHoldingsDivisor float64 `json:"actionsPerTurnHoldingsDivisor"`

	// TurnEndsWhenAllPlayersDone indicates whether a turn ends when all players have done all their actions. If TurnDurationString is
	// unset, this must be true (otherwise, the turn will never end).
	TurnEndsWhenAllPlayersDone bool `json:"turnEndsWhenAllPlayersDone"`

	// TurnDurationString determines how long a turn lasts before it ends, if it is a zero value, the turn only ends when all players are done.
	// It is expected to be a valid duration string parseable by `durationutil.ParseLongerDuration`
	TurnDuration durationutil.ExtendedDuration `json:"turnDuration,omitempty"`

	// DoTurnManagement indicates whether turn management should be handled internally. If it is false, it is assumed that the consuming
	// application will handle turn management, such as by using a timer or a game loop. Default is true.
	DoTurnManagement bool `json:"doTurnManagement"`

	// Territories is the list of valid territories that can be owned by players
	Territories []Territory `json:"territories"`
}

func (tc *Config) ResolveTerritory(query string) (*Territory, error) {
	for t, territory := range tc.Territories {
		queryLower := strings.ToLower(query)
		abbrLower := strings.ToLower(territory.Abbreviation)
		nameLower := strings.ToLower(territory.Name)
		if abbrLower == queryLower || queryLower == nameLower {
			tc.Territories[t].cfg = tc
			return &tc.Territories[t], nil
		}
		for _, alias := range territory.Aliases {
			aliasLower := strings.ToLower(alias)
			if queryLower == aliasLower {
				tc.Territories[t].cfg = tc
				return &tc.Territories[t], nil
			}
		}
	}
	return nil, fmt.Errorf("unrecognized abbreviation, name, or alias %q", query)
}

func (tc *Config) validateRequiredValues() error {
	if tc.MapFile == "" {
		return &missingFieldError{"mapFile"}
	}
	if tc.DBFile == "" {
		return &missingFieldError{"dbFile"}
	}
	if tc.SVGOutFile == "" {
		return &missingFieldError{"svgOutFile"}
	}
	if tc.PNGOutFile == "" {
		return &missingFieldError{"pngOutFile"}
	}
	if tc.MaxArmiesPerTerritory <= 0 {
		tc.MaxArmiesPerTerritory = defaultMaxArmiesPerTerritory
	}
	if tc.InitialArmies <= 0 {
		tc.InitialArmies = defaultInitialArmies
	}
	if tc.MinimumNationsToStart <= 0 {
		tc.MinimumNationsToStart = defaultMinimumNationsToStart
	}
	if len(tc.Territories) == 0 {
		return fmt.Errorf("at least one territory is required")
	}
	if tc.ActionsPerTurnHoldingsDivisor <= 0 {
		tc.ActionsPerTurnHoldingsDivisor = defaultActionsPerTurnHoldingsDivisor
	}

	if !tc.TurnEndsWhenAllPlayersDone && tc.TurnDuration == 0 {
		return fmt.Errorf("turnDuration must be set if turnEndsWhenAllPlayersDone is false")
	}

	if tc.DoTurnManagement {
		return errNoSQLiteMathFunctionsError // if this build has sqlite_math_functions tag, this should be nil
	}

	return nil
}

func (tc *Config) validateUniqueness() error {
	const errFmt = "found non-unique territory with query %q"
	uniqueTerritories := make(map[string]string)
	for _, territory := range tc.Territories {
		_, found := uniqueTerritories[territory.Abbreviation]
		if found {
			return fmt.Errorf(errFmt, territory.Abbreviation)
		}
		uniqueTerritories[territory.Abbreviation] = ""

		_, found = uniqueTerritories[territory.Name]
		if found {
			return fmt.Errorf(errFmt, territory.Name)
		}
		uniqueTerritories[territory.Name] = ""

		for _, alias := range territory.Aliases {
			_, found = uniqueTerritories[alias]
			if found {
				return fmt.Errorf(errFmt, alias)
			}
			uniqueTerritories[alias] = ""
		}
	}
	return nil
}

func (tc *Config) validateNeighborMutuality() error {
	for _, territory := range tc.Territories {
		abbr := territory.Abbreviation
		if len(territory.Neighbors) == 0 {
			return fmt.Errorf("found territory %q with no neighbors", territory.Name)
		}
		for _, neighborAbbr := range territory.Neighbors {
			if neighborAbbr == abbr {
				return fmt.Errorf("found territory %q with itself as a neighbor", abbr)
			}
			neighbor, err := tc.ResolveTerritory(neighborAbbr)
			if err != nil {
				return err
			}
			mutual, err := neighbor.IsNeighboring(abbr)
			if err != nil {
				return err
			}
			if !mutual {
				return fmt.Errorf("found non-mutual neighbors %q and %q", abbr, neighborAbbr)
			}
		}
	}
	return nil
}

type missingFieldError struct {
	field string
}

func (e *missingFieldError) Error() string {
	return fmt.Sprintf("%s is required", e.field)
}

func validateConfig() (err error) {
	if cfg == nil {
		return ErrGameNotConfigured
	}
	for t := range cfg.Territories {
		cfg.Territories[t].cfg = cfg
	}
	if err = cfg.validateRequiredValues(); err != nil {
		return fmt.Errorf("failed to validate required values: %w", err)
	}
	if err = cfg.validateUniqueness(); err != nil {
		return fmt.Errorf("failed to validate uniqueness of territories: %w", err)
	}
	if err = cfg.validateNeighborMutuality(); err != nil {
		return fmt.Errorf("failed to validate mutuality of neighbors: %w", err)
	}
	return nil
}

// SetConfig validates the incoming configuration struct, and if it passes validation, sets it as the active configuration.
func SetConfig(c *Config) error {
	if c == nil {
		return ErrGameNotConfigured
	}
	cfg = c
	err := validateConfig()
	if err == nil {
		if cfg.LogInfo == nil {
			cfg.LogInfo = noopLoggerFunc
		}
		if cfg.LogError == nil {
			cfg.LogError = noopLoggerFunc
		}
		*c = *cfg
	}
	return err
}

func GetConfig() (*Config, error) {
	if cfg == nil {
		return nil, ErrGameNotConfigured
	}

	return cfg, nil
}

func GetTestingConfig(t *testing.T) (*Config, error) {
	if !testing.Testing() {
		panic("GetTestingConfig should only be called in testing mode")
	}
	if cfg == nil {
		dir := t.TempDir()
		cfg = &Config{
			MapFile: path.Join(dir, "test.svg"),
			DBFile:  path.Join(dir, "test.db"),

			LogInfo: func(s string, a ...any) {
				t.Helper()
				t.Log(append([]any{s}, a...)...)
			},
			LogError: func(s string, a ...any) {
				t.Helper()
				t.Log(append([]any{s}, a...)...)
			},
			SVGOutFile:                    path.Join(dir, "test.svg"),
			PNGOutFile:                    path.Join(dir, "test.png"),
			DoCounterattack:               false,
			MaxArmiesPerTerritory:         defaultMaxArmiesPerTerritory,
			InitialArmies:                 defaultInitialArmies,
			MinimumNationsToStart:         defaultMinimumNationsToStart,
			ActionsPerTurnHoldingsDivisor: defaultActionsPerTurnHoldingsDivisor,
			DoTurnManagement:              true,
			TurnEndsWhenAllPlayersDone:    true,
			Territories: []Territory{
				{Name: "California", Abbreviation: "CA", Neighbors: []string{"NV", "OR", "AZ"}},
				{Name: "Nevada", Abbreviation: "NV", Neighbors: []string{"CA", "OR", "UT", "AZ"}},
				{Name: "Oregon", Abbreviation: "OR", Neighbors: []string{"CA", "NV"}},
				{Name: "Arizona", Abbreviation: "AZ", Neighbors: []string{"CA", "NV"}},
				{Name: "Utah", Abbreviation: "UT", Neighbors: []string{"NV"}},
			},
		}
	}
	return cfg, nil
}

func CloseTestingConfig(t *testing.T) {
	cfg = nil
}

type Territory struct {
	Abbreviation string   `json:"abbr"`
	Name         string   `json:"name"`
	Aliases      []string `json:"aliases"`
	Neighbors    []string `json:"neighbors"`
	cfg          *Config
}

func (t *Territory) IsNeighboring(query string) (bool, error) {
	found, err := t.cfg.ResolveTerritory(query)
	if err != nil {
		return false, err
	}
	return slices.Contains(t.Neighbors, found.Abbreviation), nil
}
