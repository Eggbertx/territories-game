package actions

import (
	"database/sql"
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
			doValidateQueries: func(t *testing.T, db *sql.DB) {
				var nationName string
				err := db.QueryRow("SELECT country_name FROM nations WHERE player = 'Test User'").Scan(&nationName)
				if !assert.NoError(t, err, "failed to query for empty nation name") {
					t.FailNow()
				}
				assert.NotEmpty(t, nationName, "expected country name to not be empty")
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
			doValidateQueries: func(t *testing.T, d *sql.DB) {
				var color string
				err := d.QueryRow("SELECT color FROM nations WHERE player = 'Test User'").Scan(&color)
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
	}
)

type eventsTestCase struct {
	desc              string
	events            []GameEvent
	expectError       bool
	doValidateQueries func(*testing.T, *sql.DB)

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
	if tc.expectError {
		assert.Error(t, err, "expected error for event: %v", errEvent)
		return
	} else {
		assert.NoError(t, err, "unexpected error for event: %v", errEvent)
	}
	if tc.doValidateQueries != nil {
		tc.doValidateQueries(t, tc.db)
	}
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
