package storage

import (
	"database/sql"
	"time"
)

func b(v bool) int {
	if v {
		return 1
	}
	return 0
}
func parseTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	x, e := time.Parse(time.RFC3339Nano, v.String)
	if e != nil {
		return nil
	}
	return &x
}
func validActivitySeconds(v int) int {
	if v < 30 {
		return 60
	}
	if v > 3600 {
		return 3600
	}
	return v
}
