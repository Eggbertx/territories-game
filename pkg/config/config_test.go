package config

import (
	"strings"
	"testing"

	"github.com/Eggbertx/durationutil"
	"github.com/stretchr/testify/assert"
)

var (
	territoryAbbreviations = []string{
		"al", "ak", "az", "ar", "ca", "co", "ct",
		"de", "fl", "ga", "hi", "id", "il", "in",
		"ia", "ks", "ky", "la", "me", "md", "ma",
		"mi", "mn", "ms", "mo", "mt", "ne", "nv",
		"nh", "nj", "nm", "ny", "nc", "nd", "oh",
		"ok", "or", "pa", "ri", "sc", "sd", "tn",
		"tx", "ut", "vt", "va", "wa", "wv", "wi",
		"wy", "dc", "pr", "vi", "gu", "as", "mp",
	}
	territoryNames = []string{
		"Alabama", "Alaska", "Arizona", "Arkansas", "California", "Colorado", "Connecticut",
		"Delaware", "Florida", "Georgia", "Hawaii", "Idaho", "Illinois", "Indiana",
		"Iowa", "Kansas", "Kentucky", "Louisiana", "Maine", "Maryland", "Massachusetts",
		"Michigan", "Minnesota", "Mississippi", "Missouri", "Montana", "Nebraska", "Nevada",
		"New Hampshire", "New Jersey", "New Mexico", "New York", "North Carolina", "North Dakota", "Ohio",
		"Oklahoma", "Oregon", "Pennsylvania", "Rhode Island", "South Carolina", "South Dakota", "Tennessee",
		"Texas", "Utah", "Vermont", "Virginia", "Washington", "West Virginia", "Wisconsin",
		"Wyoming", "Washington D.C.", "Puerto Rico", "Virgin Islands", "Guam", "American Samoa",
		"Northern Mariana Islands",
	}
	aliasTestCases = []aliasTestCase{
		{"District of Columbia", "DC"},
	}

	dummyTerritories = []Territory{
		{Abbreviation: "CA", Name: "California", Neighbors: []string{"NV"}},
		{Abbreviation: "NV", Name: "Nevada", Neighbors: []string{"CA"}},
	}

	validateRequiredValuesTestCases = []configRequiredValuesTestCase{
		{
			desc: "Missing mapFile",
			cfg: &Config{
				Territories: dummyTerritories,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "mapFile", err.(*missingFieldError).field)
			},
		},
		{
			desc: "Missing dbFile",
			cfg: &Config{
				MapFile:     "map.svg",
				Territories: dummyTerritories,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "dbFile", err.(*missingFieldError).field)
			},
		},
		{
			desc: "Missing svgOutFile",
			cfg: &Config{
				MapFile:     "map.svg",
				DBFile:      "db.sqlite",
				Territories: dummyTerritories,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "svgOutFile", err.(*missingFieldError).field)
			},
		},
		{
			desc: "Missing pngOutFile",
			cfg: &Config{
				MapFile:     "map.svg",
				DBFile:      "territories.db",
				SVGOutFile:  "output.svg",
				Territories: dummyTerritories,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "pngOutFile", err.(*missingFieldError).field)
			},
		},
		{
			desc: "fail if no territories",
			cfg: &Config{
				MapFile:    "map.svg",
				DBFile:     "territories.db",
				SVGOutFile: "output.svg",
				PNGOutFile: "output.png",
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "at least one territory is required", err.Error())
			},
		},
		{
			desc: "turnEndsWhenAllPlayersDone or turnEndsWhenTimeExpires required",
			cfg: &Config{
				MapFile:     "map.svg",
				DBFile:      "territories.db",
				SVGOutFile:  "output.svg",
				PNGOutFile:  "output.png",
				Territories: dummyTerritories,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "either turnEndsWhenAllPlayersDone or turnEndsWhenTimeExpires (or both) must be true", err.Error())
			},
		},
		{
			desc: "turnEndsWhenTimeExpires requires turnDuration",
			cfg: &Config{
				MapFile:                 "map.svg",
				DBFile:                  "territories.db",
				SVGOutFile:              "output.svg",
				PNGOutFile:              "output.png",
				Territories:             dummyTerritories,
				TurnEndsWhenTimeExpires: true,
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.Equal(t, "turnEndsWhenTimeExpires is true, but turnDuration is not set", err.Error())
			},
		},
		{
			desc: "invalid turnDuration format",
			cfg: &Config{
				MapFile:                 "map.svg",
				DBFile:                  "territories.db",
				SVGOutFile:              "output.svg",
				PNGOutFile:              "output.png",
				Territories:             dummyTerritories,
				TurnEndsWhenTimeExpires: true,
				TurnDurationString:      "lol",
			},
			expectError: true,
			validateFunc: func(t *testing.T, _ *Config, err error) {
				assert.ErrorIs(t, err, durationutil.ErrInvalidDurationString)
			},
		},
		{
			desc: "valid configuration, optional fields set",
			cfg: &Config{
				MapFile:                    "map.svg",
				DBFile:                     "territories.db",
				SVGOutFile:                 "output.svg",
				PNGOutFile:                 "output.png",
				Territories:                dummyTerritories,
				TurnEndsWhenAllPlayersDone: true,
				TurnEndsWhenTimeExpires:    true,
				TurnDurationString:         "1h",
			},
			validateFunc: func(t *testing.T, cfg *Config, err error) {
				assert.NoError(t, err)

				assert.False(t, cfg.DoCounterattack)
				assert.Equal(t, 3, cfg.InitialArmies)
				assert.Equal(t, 3, cfg.MinimumNationsToStart)
				assert.Equal(t, 5, cfg.MaxArmiesPerTerritory)
				assert.False(t, cfg.UnclaimedTerritoriesHave1Army)
				assert.Equal(t, 3.0, cfg.ActionsPerTurnHoldingsDivisor)
				assert.True(t, cfg.TurnEndsWhenAllPlayersDone)
				assert.True(t, cfg.TurnEndsWhenTimeExpires)
			},
		},
	}
)

type aliasTestCase struct {
	alias  string
	expect string
}

type configRequiredValuesTestCase struct {
	desc         string
	cfg          *Config
	expectError  bool
	validateFunc func(t *testing.T, cfg *Config, err error)
}

func getTestConfig() *Config {
	tcfg := &Config{
		Territories: make([]Territory, len(territoryAbbreviations)),
	}
	for i, abbr := range territoryAbbreviations {
		tcfg.Territories[i] = Territory{
			Abbreviation: strings.ToUpper(abbr),
			Name:         territoryNames[i],
			cfg:          tcfg,
		}
		if abbr == "dc" {
			tcfg.Territories[i].Aliases = []string{"Washington DC", "Washington D.C", "District of Columbia", "D.C.", "D.C"}
		}
	}
	return tcfg
}

func TestResolveAbbrToAbbr(t *testing.T) {
	tcfg := getTestConfig()

	found, err := tcfg.ResolveTerritory("")
	assert.Error(t, err)
	for _, abbr := range territoryAbbreviations {
		t.Run(abbr, func(t *testing.T) {
			found, err = tcfg.ResolveTerritory(abbr)
			assert.NoError(t, err)
			assert.Equal(t, strings.ToUpper(abbr), found.Abbreviation)
		})
	}
}

func TestResolveNameToAbbr(t *testing.T) {
	tcfg := getTestConfig()
	for n, name := range territoryNames {
		abbr := strings.ToUpper(territoryAbbreviations[n])
		t.Run(name+"->"+abbr, func(t *testing.T) {
			found, err := tcfg.ResolveTerritory(name)
			assert.NoError(t, err)
			assert.Equal(t, abbr, found.Abbreviation)
		})
	}
}

func TestResolveAliasToAbbr(t *testing.T) {
	tcfg := getTestConfig()
	for _, tC := range aliasTestCases {
		t.Run(tC.alias, func(t *testing.T) {
			resolved, err := tcfg.ResolveTerritory(tC.alias)
			assert.NoError(t, err)
			assert.Equal(t, tC.expect, resolved.Abbreviation)
		})
	}
}

func TestUniquenessValidation(t *testing.T) {
	uniqueCfg := Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1"},
			{Abbreviation: "abbr2", Name: "name2", Aliases: []string{"alias1"}},
		},
	}
	duplicateAbbrCfg := Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1"},
			{Abbreviation: "abbr1", Name: "name2", Aliases: []string{"alias1"}},
		},
	}
	duplicateNameCfg := Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1"},
			{Abbreviation: "abbr2", Name: "name1", Aliases: []string{"alias1"}},
		},
	}
	duplicateAliasCfg := Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1"},
			{Abbreviation: "abbr2", Name: "name2", Aliases: []string{"alias1"}},
			{Abbreviation: "abbr3", Name: "name3", Aliases: []string{"alias1"}},
		},
	}
	err := uniqueCfg.validateUniqueness()
	assert.NoError(t, err)
	err = duplicateAbbrCfg.validateUniqueness()
	assert.Error(t, err)
	err = duplicateNameCfg.validateUniqueness()
	assert.Error(t, err)
	err = duplicateAliasCfg.validateUniqueness()
	assert.Error(t, err)
}

func TestMutualNeighborValidation(t *testing.T) {
	var validCfg Config
	var noNeighborsCfg Config
	var selfNeighborCfg Config
	var invalidNeighborCfg Config
	var nonMutualNeighborsCfg Config

	validCfg = Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1", Neighbors: []string{"abbr2", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr2", Name: "name2", Neighbors: []string{"abbr1", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr3", Name: "name3", Neighbors: []string{"abbr1", "abbr2", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr4", Name: "name4", Neighbors: []string{"abbr1", "abbr2", "abbr3"}, cfg: &validCfg},
		},
	}
	noNeighborsCfg = Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1", cfg: &validCfg},
			{Abbreviation: "abbr2", Name: "name2", Neighbors: []string{"abbr1", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr3", Name: "name3", Neighbors: []string{"abbr1", "abbr2", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr4", Name: "name4", Neighbors: []string{"abbr1", "abbr2", "abbr3"}, cfg: &validCfg},
		},
	}
	selfNeighborCfg = Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1", Neighbors: []string{"abbr1", "abbr2", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr2", Name: "name2", Neighbors: []string{"abbr1", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr3", Name: "name3", Neighbors: []string{"abbr1", "abbr2", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr4", Name: "name4", Neighbors: []string{"abbr1", "abbr2", "abbr3"}, cfg: &validCfg},
		},
	}
	invalidNeighborCfg = Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1", Neighbors: []string{"abbr", "abbr2", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr2", Name: "name2", Neighbors: []string{"abbr1", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr3", Name: "name3", Neighbors: []string{"abbr1", "abbr2", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr4", Name: "name4", Neighbors: []string{"abbr1", "abbr2", "abbr3"}, cfg: &validCfg},
		},
	}
	nonMutualNeighborsCfg = Config{
		Territories: []Territory{
			{Abbreviation: "abbr1", Name: "name1", Neighbors: []string{"abbr2", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr2", Name: "name2", Neighbors: []string{"abbr1", "abbr3", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr3", Name: "name3", Neighbors: []string{"abbr1", "abbr2", "abbr4"}, cfg: &validCfg},
			{Abbreviation: "abbr4", Name: "name4", Neighbors: []string{"abbr1", "abbr2"}, cfg: &validCfg},
		},
	}
	err := validCfg.validateNeighborMutuality()
	assert.NoError(t, err)
	err = noNeighborsCfg.validateNeighborMutuality()
	assert.Error(t, err)
	err = selfNeighborCfg.validateNeighborMutuality()
	assert.Error(t, err)
	err = invalidNeighborCfg.validateNeighborMutuality()
	assert.Error(t, err)
	err = nonMutualNeighborsCfg.validateNeighborMutuality()
	assert.Error(t, err)
}

func TestValidateRequiredValues(t *testing.T) {
	for _, tc := range validateRequiredValuesTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.validateRequiredValues()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.validateFunc != nil {
				tc.validateFunc(t, tc.cfg, err)
			}
		})
	}
}
