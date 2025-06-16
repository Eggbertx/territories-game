package actions

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/Eggbertx/territories-game/pkg/config"
	"github.com/Eggbertx/territories-game/pkg/db"
	"github.com/stretchr/testify/assert"
)

var (
	joinTestCases = []actionsTestCase{
		{
			desc: "valid join events",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "",
					territory: "NV",
				},
			},
			expectError: false,
		},
		{
			desc: "reject join from duplicate user",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 2",
					territory: "NV",
				},
			},
			expectError: true,
		},
		{
			desc: "reject join with duplicate nation name",
			events: []Action{
				&JoinAction{
					user:      "Test User 1",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 1",
					territory: "NV",
				},
			},
			expectError: true,
		},
		{
			desc: "don't reject join with missing nation name",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					territory: "CA",
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, err error) {
				if err != nil {
					t.FailNow()
				}
				var nationName string
				err = db.QueryRow("SELECT country_name FROM nations WHERE player = 'Test User'").Scan(&nationName)
				if !assert.NoError(t, err, "failed to query for empty nation name") {
					t.FailNow()
				}
				assert.NotEmpty(t, nationName, "expected country name to not be empty")
			},
		},
		{
			desc: "reject join, territory already occupied",
			events: []Action{
				&JoinAction{
					user:      "Test User 1",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				if err == nil {
					t.FailNow()
				}
				assert.ErrorIs(t, err, ErrTerritoryAlreadyOccupied)

				var nationCount int
				err = d.QueryRow("SELECT COUNT(*) FROM nations WHERE player = 'Test User 1'").Scan(&nationCount)
				if !assert.NoError(t, err, "failed to query for Test User 1's nation") {
					t.FailNow()
				}
				assert.Equal(t, 1, nationCount, "expected Test User 1 to have one nation")

				err = d.QueryRow("SELECT COUNT(*) FROM nations WHERE player = 'Test User 2'").Scan(&nationCount)
				if !assert.NoError(t, err, "failed to query for Test User 2's nation") {
					t.FailNow()
				}
				assert.Equal(t, 0, nationCount, "expected Test User 2 to not have a nation due to occupation of CA by Test User 1")
			},
		},
	}
	colorTestCases = []actionsTestCase{
		{
			desc: "valid color changes",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&ColorAction{
					user:  "Test User",
					color: "white",
				},
				&ColorAction{
					user:  "Test User",
					color: "ffffff",
				},
				&ColorAction{
					user:  "Test User",
					color: "#ffffff",
				},
			},
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				if err != nil {
					t.FailNow()
				}
				var color string
				err = d.QueryRow("SELECT color FROM nations WHERE player = 'Test User'").Scan(&color)
				if !assert.NoError(t, err, "failed to query for color change") {
					t.FailNow()
				}
				assert.Equal(t, "ffffff", color)
			},
		},
		{
			desc: "reject invalid color",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&ColorAction{
					user:  "Test User",
					color: "invalidcolor",
				},
			},
			expectError: true,
		},
		{
			desc: "don't allow changing someone else's color",
			events: []Action{
				&JoinAction{
					user:      "Test User 1",
					nation:    "Nation 1",
					territory: "CA",
				},
				&ColorAction{
					user:  "Test User 2",
					color: "ffffff",
				},
			},
			expectError: true,
		},
		{
			desc: "don't allow duplicate color",
			events: []Action{
				&JoinAction{
					user:      "Test User 1",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "NV",
				},
				&ColorAction{
					user:  "Test User 1",
					color: "ffffff",
				},
				&ColorAction{
					user:  "Test User 2",
					color: "ffffff",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				if err == nil {
					t.FailNow()
				}
				assert.ErrorIs(t, err, ErrColorInUse)

				var color string
				err = d.QueryRow("SELECT color FROM nations WHERE player = 'Test User 1'").Scan(&color)
				if !assert.NoError(t, err, "failed to query for color change") {
					t.FailNow()
				}
				assert.Equal(t, "ffffff", color, "expected Test User 1's color to be unchanged")
				err = d.QueryRow("SELECT color FROM nations WHERE player = 'Test User 2'").Scan(&color)
				if !assert.NoError(t, err, "failed to query for Test User 2's color") {
					t.FailNow()
				}
				assert.NotEqual(t, "ffffff", color, "expected Test User 2's color to not be changed to Test User 1's color")
			},
		},
		{
			desc: "reject unregistered user",
			events: []Action{
				&ColorAction{
					user:  "Unregistered User",
					color: "ffffff",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorIs(t, err, ErrUserNotRegistered)

				var color string
				err = d.QueryRow("SELECT color FROM nations WHERE player = 'Unregistered User'").Scan(&color)
				assert.ErrorIs(t, err, sql.ErrNoRows)
			},
		},
	}
	attackTestCases = []actionsTestCase{
		{
			desc: "invalid attack territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "lol",
					defendingTerritory: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "unrecognized abbreviation, name, or alias")
			},
		},
		{
			desc: "can't attack own territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "CA",
					defendingTerritory: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "friendly fire not allowed")
			},
		},
		{
			desc: "reject attack from unregistered user",
			events: []Action{
				&AttackAction{
					user:               "Unregistered User",
					attackingTerritory: "CA",
					defendingTerritory: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorIs(t, err, ErrUserNotRegistered)

				var armySize int
				err = d.QueryRow("SELECT army_size FROM holdings WHERE territory = 'CA'").Scan(&armySize)
				assert.ErrorIs(t, err, sql.ErrNoRows, "expected no armies in CA due to unregistered user attack")
			},
		},
		{
			desc: "valid attack",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "NV",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "CA",
					defendingTerritory: "NV",
				},
			},
			doValidateQueries: func(t *testing.T, d *sql.DB, _ error) {
				var attackingArmySize, defendingArmySize int
				err := d.QueryRow("SELECT army_size FROM holdings WHERE territory = 'CA'").Scan(&attackingArmySize)
				if !errors.Is(err, sql.ErrNoRows) && !assert.NoError(t, err) {
					t.FailNow()
				}
				err = d.QueryRow("SELECT army_size FROM holdings WHERE territory = 'NV'").Scan(&defendingArmySize)
				if !errors.Is(err, sql.ErrNoRows) && !assert.NoError(t, err) {
					t.FailNow()
				}
				// TODO: populate battle results in the database
				assert.LessOrEqual(t, defendingArmySize, 3)
				assert.LessOrEqual(t, attackingArmySize, 3)
			},
		},
		{
			desc: "no armies in defending territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "CA",
					defendingTerritory: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "no armies in Nevada")
			},
		},
		{
			desc: "no armies in attacking territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "NV",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "UT",
					defendingTerritory: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "no armies in Utah controlled by Test User to attack with")
			},
		},
		{
			desc: "can't attack non-neighboring territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "AZ",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "OR",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "AZ",
					defendingTerritory: "OR",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "cannot attack Oregon from Arizona: not a neighboring territory")
			},
		},
		{
			desc: "nation is eliminated if no territories left",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "NV",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&AttackAction{
					user:               "Test User",
					attackingTerritory: "CA",
					defendingTerritory: "NV",
				},
			},
			beforeEachEvent: func(t *testing.T, d *sql.DB, i int) error {
				if i > 1 && testInt == nil {
					testInt = new(int)
					*testInt = 19
				}
				return nil
			},
			doValidateQueries: func(t *testing.T, d *sql.DB, _ error) {
				var nation1Count, nation2Count int

				err := d.QueryRow("SELECT COUNT(*) FROM nations WHERE player = 'Test User'").Scan(&nation1Count)
				assert.NoError(t, err)

				err = d.QueryRow("SELECT COUNT(*) FROM nations WHERE player = 'Test User 2'").Scan(&nation2Count)
				assert.NoError(t, err)

				assert.Equal(t, 1, nation1Count, "expected Test User to remain")
				assert.Zero(t, nation2Count, "expected Test User 2 to be eliminated")
			},
		},
	}
	raiseTestCases = []actionsTestCase{
		{
			desc: "valid raise event",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, _ error) {
				var armySize int
				err := db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 5, armySize)
			},
		},
		{
			desc: "enforce max raise limit",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, db *sql.DB, err error) {
				assert.ErrorContains(t, err, "cannot raise army size in California: already at maximum of 5")
				var armySize int
				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 5, armySize, "expected army size to be capped at 5")
			},
		},
		{
			desc: "raise in unowned territory",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, db *sql.DB, err error) {
				assert.ErrorContains(t, err, "no armies in Nevada controlled by Test User to raise")
				var armySize int
				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'NV'").Scan(&armySize)
				assert.ErrorIs(t, err, sql.ErrNoRows, "expected no armies in NV since it is unowned")
			},
		},
		{
			desc: "raise from unregistered user",
			events: []Action{
				&RaiseAction{
					user:      "Unregistered User",
					territory: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, db *sql.DB, err error) {
				assert.ErrorIs(t, err, ErrUserNotRegistered)

				var armySize int
				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				assert.ErrorIs(t, err, sql.ErrNoRows, "expected no armies in CA since Unregistered User cannot raise armies")
			},
		},
	}
	moveTestCases = []actionsTestCase{
		{
			desc: "valid move event (all units)",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&MoveAction{
					user:                 "Test User",
					sourceTerritory:      "CA",
					destinationTerritory: "NV",
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, _ error) {
				var armySize int
				err := db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				assert.ErrorIs(t, err, sql.ErrNoRows, "expected no units left in CA after move")
				assert.Equal(t, 0, armySize, "expected all units to be moved from CA")

				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'NV'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 3, armySize, "expected one unit to be moved to NV")
			},
		},
		{
			desc: "valid move event (some units)",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&RaiseAction{
					user:      "Test User",
					territory: "CA",
				},
				&MoveAction{
					user:                 "Test User",
					sourceTerritory:      "CA",
					destinationTerritory: "NV",
					armies:               1,
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, _ error) {
				var armySize int
				err := db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 3, armySize, "expected one unit left in CA after move")

				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'NV'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 1, armySize, "expected one unit to be moved to NV")
			},
		},
		{
			desc: "territory already occupied",
			events: []Action{
				&JoinAction{
					user:      "Test User",
					nation:    "Nation 1",
					territory: "CA",
				},
				&JoinAction{
					user:      "Test User 2",
					nation:    "Nation 2",
					territory: "NV",
				},
				&MoveAction{
					user:                 "Test User",
					sourceTerritory:      "CA",
					destinationTerritory: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, db *sql.DB, err error) {
				assert.ErrorIs(t, err, ErrTerritoryAlreadyOccupied)
				var armySize int
				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 3, armySize, "expected no units moved from CA due to occupation of NV")
			},
		},
	}
)

type actionsTestCase struct {
	desc              string
	events            []Action
	expectError       bool
	beforeEachEvent   func(*testing.T, *sql.DB, int) error
	doValidateQueries func(*testing.T, *sql.DB, error)

	db *sql.DB
}

func runActionTestCase(t *testing.T, tc *actionsTestCase) {
	var err error
	config.GetTestingConfig()
	tc.db, err = db.GetDB()
	if !assert.NoError(t, err, "failed to get test database") {
		t.FailNow()
	}

	defer func() {
		assert.NoError(t, db.CloseDB())
		config.CloseTestingConfig(t)
		db.CloseDB()
	}()
	var errAction Action
	for e, event := range tc.events {
		if tc.beforeEachEvent != nil {
			err = tc.beforeEachEvent(t, tc.db, e)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
		}

		_, err = event.DoAction(tc.db)
		if err != nil {
			errAction = event
			break
		}
	}

	if tc.expectError {
		assert.Error(t, err, "expected error for event: %v", errAction)
	} else {
		assert.NoError(t, err, "unexpected error for event: %v", errAction)
	}
	if tc.doValidateQueries != nil {
		tc.doValidateQueries(t, tc.db, err)
		testInt = nil
	}
}

func TestJoinEvent(t *testing.T) {
	for _, tc := range joinTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runActionTestCase(t, &tc)
		})
	}
}

func TestColorEvent(t *testing.T) {
	for _, tc := range colorTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runActionTestCase(t, &tc)
		})
	}
}

func TestAttackEvent(t *testing.T) {
	for _, tc := range attackTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runActionTestCase(t, &tc)
		})
	}
}

func TestRaiseEvent(t *testing.T) {
	for _, tc := range raiseTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runActionTestCase(t, &tc)
		})
	}
}

func TestMoveEvent(t *testing.T) {
	for _, tc := range moveTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runActionTestCase(t, &tc)
		})
	}
}
