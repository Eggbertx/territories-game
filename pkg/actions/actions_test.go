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
	joinTestCases = []eventsTestCase{
		{
			desc: "valid join events",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "",
					Predicate: "NV",
				},
			},
			expectError: false,
		},
		{
			desc: "reject join from duplicate user",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "NV",
				},
			},
			expectError: true,
		},
		{
			desc: "reject join with duplicate nation name",
			events: []GameEvent{
				{
					User:      "Test User 1",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "NV",
				},
			},
			expectError: true,
		},
		{
			desc: "don't reject join with missing subject",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "",
					Predicate: "CA",
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
			events: []GameEvent{
				{
					User:      "Test User 1",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "CA",
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
	colorTestCases = []eventsTestCase{
		{
			desc: "valid color changes",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "color",
					Subject: "white",
				},
				{
					User:    "Test User",
					Action:  "color",
					Subject: "ffffff",
				},
				{
					User:    "Test User",
					Action:  "color",
					Subject: "#ffffff",
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
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "color",
					Subject: "invalidcolor",
				},
			},
			expectError: true,
		},
		{
			desc: "don't allow changing someone else's color",
			events: []GameEvent{
				{
					User:      "Test User 1",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User 2",
					Action:  "color",
					Subject: "ffffff",
				},
			},
			expectError: true,
		},
		{
			desc: "don't allow duplicate color",
			events: []GameEvent{
				{
					User:      "Test User 1",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "NV",
				},
				{
					User:    "Test User 1",
					Action:  "color",
					Subject: "ffffff",
				},
				{
					User:    "Test User 2",
					Action:  "color",
					Subject: "ffffff",
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
			events: []GameEvent{
				{
					User:    "Unregistered User",
					Action:  "color",
					Subject: "ffffff",
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
	attackTestCases = []eventsTestCase{
		{
			desc: "invalid attack territory",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "attack",
					Subject: "lol",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "unrecognized abbreviation, name, or alias")
			},
		},
		{
			desc: "can't attack own territory",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User",
					Action:    "attack",
					Subject:   "CA",
					Predicate: "CA",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "friendly fire not allowed")
			},
		},
		{
			desc: "reject attack from unregistered user",
			events: []GameEvent{
				{
					User:      "Unregistered User",
					Action:    "attack",
					Subject:   "CA",
					Predicate: "NV",
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
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "NV",
				},
				{
					User:      "Test User",
					Action:    "attack",
					Subject:   "CA",
					Predicate: "NV",
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
				assert.LessOrEqual(t, defendingArmySize, 1)
				assert.LessOrEqual(t, attackingArmySize, 1)
			},
		},
		{
			desc: "no armies in defending territory",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User",
					Action:    "attack",
					Subject:   "CA",
					Predicate: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "no armies in Nevada")
			},
		},
		{
			desc: "no armies in attacking territory",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "NV",
				},
				{
					User:      "Test User",
					Action:    "attack",
					Subject:   "UT",
					Predicate: "NV",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "no armies in Utah controlled by Test User to attack with")
			},
		},
		{
			desc: "can't attack non-neighboring territory",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "AZ",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "OR",
				},
				{
					User:      "Test User",
					Action:    "attack",
					Subject:   "AZ",
					Predicate: "OR",
				},
			},
			expectError: true,
			doValidateQueries: func(t *testing.T, d *sql.DB, err error) {
				assert.ErrorContains(t, err, "cannot attack Oregon from Arizona: not a neighboring territory")
			},
		},
	}
	raiseTestCases = []eventsTestCase{
		{
			desc: "valid raise event",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, _ error) {
				var armySize int
				err := db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 3, armySize)
			},
		},
		{
			desc: "enforce max raise limit",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
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
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "NV",
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
			events: []GameEvent{
				{
					User:    "Unregistered User",
					Action:  "raise",
					Subject: "CA",
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
	moveTestCases = []eventsTestCase{
		{
			desc: "valid move event (all units)",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:      "Test User",
					Action:    "move",
					Subject:   "CA",
					Predicate: "NV",
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
				assert.Equal(t, 2, armySize, "expected one unit to be moved to NV")
			},
		},
		{
			desc: "valid move event (some units)",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:    "Test User",
					Action:  "raise",
					Subject: "CA",
				},
				{
					User:      "Test User",
					Action:    "move",
					Subject:   "CA:1",
					Predicate: "NV",
				},
			},
			doValidateQueries: func(t *testing.T, db *sql.DB, _ error) {
				var armySize int
				err := db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'CA'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 1, armySize, "expected one unit left in CA after move")

				err = db.QueryRow("SELECT army_size FROM v_nation_holdings WHERE territory = 'NV'").Scan(&armySize)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				assert.Equal(t, 1, armySize, "expected one unit to be moved to NV")
			},
		},
		{
			desc: "territory already occupied",
			events: []GameEvent{
				{
					User:      "Test User",
					Action:    "join",
					Subject:   "Nation 1",
					Predicate: "CA",
				},
				{
					User:      "Test User 2",
					Action:    "join",
					Subject:   "Nation 2",
					Predicate: "NV",
				},
				{
					User:      "Test User",
					Action:    "move",
					Subject:   "CA",
					Predicate: "NV",
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
				assert.Equal(t, 1, armySize, "expected no units moved from CA due to occupation of NV")
			},
		},
	}
)

type eventsTestCase struct {
	desc              string
	events            []GameEvent
	expectError       bool
	doValidateQueries func(*testing.T, *sql.DB, error)

	db *sql.DB
}

func runEventTestCase(t *testing.T, tc *eventsTestCase) {
	var err error
	config.GetTestingConfig()
	tc.db, err = db.GetDB()
	if !assert.NoError(t, err, "failed to get test database") {
		t.FailNow()
	}

	defer func() {
		assert.NoError(t, db.CloseDB())
		config.CloseTestingConfig(t)
	}()
	var errEvent GameEvent
	for _, event := range tc.events {
		err = event.DoAction(tc.db)
		if err != nil {
			errEvent = event
			break
		}
	}
	if tc.expectError && !assert.Error(t, err, "expected error for event: %v", errEvent) {
		t.FailNow()
	} else if !tc.expectError && !assert.NoError(t, err, "unexpected error for event: %v", errEvent) {
		t.FailNow()
	}
	if tc.doValidateQueries != nil {
		tc.doValidateQueries(t, tc.db, err)
	}
}

func TestInvalidAction(t *testing.T) {
	invalidEvent := GameEvent{
		User:      "Test User",
		Action:    "test",
		Subject:   "Nation 1",
		Predicate: "CA",
	}
	_, err := config.GetTestingConfig()
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer config.CloseTestingConfig(t)

	// db, err := db.GetDB()
	// if !assert.NoError(t, err) {
	// 	t.FailNow()
	// }
	// defer db.Close()

	// Action should be rejected (no user specified)
	err = invalidEvent.DoAction(nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidAction)

	invalidEvent = GameEvent{
		Action:    "join",
		Subject:   "Test Nation",
		Predicate: "CA",
	}
	err = invalidEvent.DoAction(nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingUser)
}

func TestJoinEvent(t *testing.T) {
	for _, tc := range joinTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEventTestCase(t, &tc)
		})
	}
}

func TestColorEvent(t *testing.T) {
	for _, tc := range colorTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEventTestCase(t, &tc)
		})
	}
}

func TestAttackEvent(t *testing.T) {
	for _, tc := range attackTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEventTestCase(t, &tc)
		})
	}
}

func TestRaiseEvent(t *testing.T) {
	for _, tc := range raiseTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEventTestCase(t, &tc)
		})
	}
}

func TestMoveEvent(t *testing.T) {
	for _, tc := range moveTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEventTestCase(t, &tc)
		})
	}
}
