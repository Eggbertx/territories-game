package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Eggbertx/durationutil"
)

var (
	cfg *Config
)

type Config struct {
	MapFile           string `json:"mapFile"`
	DBFile            string `json:"dbFile"`
	LogFile           string `json:"logFile"`
	PrintLogToConsole bool   `json:"printLogToConsole"`
	SVGOutFile        string `json:"svgOutFile"`
	PNGOutFile        string `json:"pngOutFile"`

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
	TurnDurationString string `json:"turnDuration,omitempty"`

	// DoTurnManagement indicates whether turn management should be handled internally. If it is false, it is assumed that the consuming
	// application will handle turn management, such as by using a timer or a game loop. Default is true.
	DoTurnManagement bool `json:"doTurnManagement"`

	Territories []Territory `json:"territories"`

	turnDuration time.Duration
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

func (tc *Config) TurnDuration() time.Duration {
	return tc.turnDuration
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
		tc.MaxArmiesPerTerritory = 5
	}
	if tc.InitialArmies <= 0 {
		tc.InitialArmies = 3
	}
	if tc.MinimumNationsToStart <= 0 {
		tc.MinimumNationsToStart = 3
	}
	if len(tc.Territories) == 0 {
		return fmt.Errorf("at least one territory is required")
	}
	if tc.ActionsPerTurnHoldingsDivisor <= 0 {
		tc.ActionsPerTurnHoldingsDivisor = 3
	}
	if tc.TurnDurationString != "" {
		var err error
		if tc.turnDuration, err = durationutil.ParseLongerDuration(tc.TurnDurationString); err != nil {
			return fmt.Errorf("failed to parse turnDuration: %w", err)
		}
	}
	if !tc.TurnEndsWhenAllPlayersDone && tc.turnDuration == 0 {
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

func openAndValidateConfig() (*Config, error) {
	c := Config{
		PrintLogToConsole:             true,
		MaxArmiesPerTerritory:         5,
		InitialArmies:                 3,
		MinimumNationsToStart:         3,
		ActionsPerTurnHoldingsDivisor: 3,
		TurnEndsWhenAllPlayersDone:    true,
		DoTurnManagement:              true,
	}
	fi, err := os.Open("config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer fi.Close()
	if err = json.NewDecoder(fi).Decode(&c); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	for t := range c.Territories {
		c.Territories[t].cfg = &c
	}
	if err = c.validateRequiredValues(); err != nil {
		return nil, fmt.Errorf("failed to validate required values: %w", err)
	}
	if err = c.validateUniqueness(); err != nil {
		return nil, fmt.Errorf("failed to validate uniqueness of territories: %w", err)
	}
	if err = c.validateNeighborMutuality(); err != nil {
		return nil, fmt.Errorf("failed to validate mutuality of neighbors: %w", err)
	}
	return &c, nil
}

func GetConfig() (*Config, error) {
	if cfg == nil {
		var err error
		cfg, err = openAndValidateConfig()
		if err != nil {
			cfg = nil
			return nil, err
		}
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
			MapFile:                       path.Join(dir, "test.svg"),
			DBFile:                        path.Join(dir, "test.db"),
			LogFile:                       path.Join(dir, "test.log"),
			PrintLogToConsole:             true,
			SVGOutFile:                    path.Join(dir, "test.svg"),
			PNGOutFile:                    path.Join(dir, "test.png"),
			DoCounterattack:               false,
			MaxArmiesPerTerritory:         5,
			InitialArmies:                 3,
			MinimumNationsToStart:         2,
			ActionsPerTurnHoldingsDivisor: 3,
			DoTurnManagement:              true,
			TurnEndsWhenAllPlayersDone:    true,
			Territories: []Territory{
				{Name: "California", Abbreviation: "CA", Neighbors: []string{"NV", "OR", "AZ"}},
				{Name: "Nevada", Abbreviation: "NV", Neighbors: []string{"CA", "OR", "UT"}},
				{Name: "Oregon", Abbreviation: "OR", Neighbors: []string{"CA", "NV"}},
				{Name: "Arizona", Abbreviation: "AZ", Neighbors: []string{"CA", "NV"}},
				{Name: "Utah", Abbreviation: "UT", Neighbors: []string{"NV", "AZ"}},
			},
		}
	}
	return cfg, nil
}

func CloseTestingConfig(t *testing.T) {
	cfg = nil
}

func SetConfig(c *Config) {
	if c != nil {
		cfg = c
	}
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
