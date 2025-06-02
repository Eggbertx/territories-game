package config

import (
	"strings"
	"testing"

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
)

type aliasTestCase struct {
	alias  string
	expect string
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
