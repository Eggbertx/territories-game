package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

var (
	cfg *Config
)

type Config struct {
	MapFile                       string      `json:"mapFile"`
	DBFile                        string      `json:"dbFile"`
	LogFile                       string      `json:"logFile"`
	PrintLogToConsole             bool        `json:"printLogToConsole"`
	SVGOutFile                    string      `json:"svgOutFile"`
	PNGOutFile                    string      `json:"pngOutFile"`
	DoCounterattack               bool        `json:"doCounterattack"`
	InitialArmies                 int         `json:"initialArmies"`
	MinimumNationsToStart         int         `json:"minimumNationsToStart"`
	MaxArmiesPerTerritory         int         `json:"maxArmiesPerTerritory"`
	UnclaimedTerritoriesHave1Army bool        `json:"unclaimedTerritoriesHave1Army"`
	Territories                   []Territory `json:"territories"`
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
		return fmt.Errorf("mapFile is required")
	}
	if tc.DBFile == "" {
		return fmt.Errorf("dbFile is required")
	}
	if tc.SVGOutFile == "" {
		return fmt.Errorf("svgOutFile is required")
	}
	if tc.PNGOutFile == "" {
		return fmt.Errorf("pngOutFile is required")
	}
	if tc.MaxArmiesPerTerritory <= 0 {
		tc.MaxArmiesPerTerritory = 5
	}
	if tc.InitialArmies <= 0 {
		tc.InitialArmies = 1
	}
	if tc.MinimumNationsToStart <= 0 {
		return fmt.Errorf("minimumNationsToStart must be greater than 0")
	}
	if len(tc.Territories) == 0 {
		return fmt.Errorf("at least one territory is required")
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

func openAndValidateConfig() (*Config, error) {
	c := Config{
		PrintLogToConsole:     true,
		MaxArmiesPerTerritory: 5,
		InitialArmies:         1,
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

func GetTestingConfig() (*Config, error) {
	if !testing.Testing() {
		panic("GetTestingConfig should only be called in testing mode")
	}
	if cfg == nil {
		dir, err := os.MkdirTemp("", "territories-test-config")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory for testing config: %w", err)
		}
		cfg = &Config{
			MapFile:               path.Join(dir, "test.svg"),
			DBFile:                path.Join(dir, "test.db"),
			LogFile:               path.Join(dir, "test.log"),
			PrintLogToConsole:     true,
			SVGOutFile:            path.Join(dir, "test.svg"),
			PNGOutFile:            path.Join(dir, "test.png"),
			DoCounterattack:       false,
			MaxArmiesPerTerritory: 5,
			InitialArmies:         3,
			MinimumNationsToStart: 2,
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
	if cfg != nil {
		dir := path.Dir(cfg.MapFile)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("failed to remove temporary directory %q: %v\n", dir, err)
		}
		cfg = nil
	}
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
