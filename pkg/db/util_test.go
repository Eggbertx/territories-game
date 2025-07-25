package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	sqliteTimestampTestCases = []timestampTestCase{
		{
			timestampQuery: "SELECT '2025-07-01 12:34:56'",
			expectTime: SQLite3Timestamp{
				Time:  time.Date(2025, 07, 1, 12, 34, 56, 0, time.UTC),
				Valid: true,
			},
			expectParseError: false,
		},
		{
			timestampQuery: "SELECT '2025-07-19 13:40:16.10521208-09:00'",
			expectTime: SQLite3Timestamp{
				Time:  time.Date(2025, 07, 19, 13, 40, 16, 105212080, time.FixedZone("", -9*60*60)),
				Valid: true,
			},
			expectParseError: false,
		},
		{
			timestampQuery:   "SELECT 'invalid-timestamp'",
			expectTime:       SQLite3Timestamp{},
			expectParseError: true,
		},
		{
			timestampQuery:   "SELECT NULL",
			expectTime:       SQLite3Timestamp{},
			expectParseError: false,
		},
		{
			timestampQuery:   "SELECT 42",
			expectTime:       SQLite3Timestamp{},
			expectParseError: true,
		},
	}
)

type timestampTestCase struct {
	timestampQuery   string
	expectTime       SQLite3Timestamp
	expectParseError bool
}

func TestSQLite3TimestampScan(t *testing.T) {
	for _, tc := range sqliteTimestampTestCases {
		t.Run(tc.timestampQuery, func(t *testing.T) {
			var ts SQLite3Timestamp
			db, err := sql.Open("sqlite3", ":memory:")
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			defer db.Close()

			err = db.QueryRow(tc.timestampQuery).Scan(&ts)
			if tc.expectParseError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectTime, ts)
			}
		})
	}
}
