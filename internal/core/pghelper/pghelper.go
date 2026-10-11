// Package pghelper provides conversion utilities between Go scalar types and
// pgtype nullable types used by the sqlc-generated pgx/v5 code.
// Use these helpers in repository files to keep conversion logic centralised.
package pghelper

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Text converts a *string to pgtype.Text. Nil pointer → invalid (NULL).
func Text(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// TextVal converts a string value to a valid pgtype.Text.
func TextVal(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// Bool converts a *bool to pgtype.Bool. Nil pointer → invalid (NULL).
func Bool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// Int4 converts a *int32 to pgtype.Int4. Nil pointer → invalid (NULL).
func Int4(n *int32) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *n, Valid: true}
}

// UUID converts a string UUID to pgtype.UUID. Invalid strings → invalid UUID.
func UUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// NullUUID converts a *string UUID to pgtype.UUID. Nil → invalid (NULL).
func NullUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return UUID(*s)
}

// Timestamptz converts a *time.Time to pgtype.Timestamptz.
func Timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// TimestamptzVal converts a time.Time value to a valid pgtype.Timestamptz.
func TimestamptzVal(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Float8 converts a *float64 to pgtype.Float8.
func Float8(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

// StringFromText returns the string value of a pgtype.Text, or "" if invalid.
func StringFromText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// StringFromUUID returns the string representation of a pgtype.UUID.
func StringFromUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return pgtype.UUID(u).String()
}

// TimeFromTimestamptz returns the time.Time of a pgtype.Timestamptz, or zero if invalid.
func TimeFromTimestamptz(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// Float64FromFloat8 returns the float64 value of a pgtype.Float8, or 0 if invalid.
func Float64FromFloat8(f pgtype.Float8) float64 {
	if !f.Valid {
		return 0
	}
	return f.Float64
}
