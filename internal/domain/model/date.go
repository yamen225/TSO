package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Date is a calendar date (no time component), serialized as "YYYY-MM-DD".
// It carries the sort contract across all boundaries: JSON, SQL, and domain logic.
type Date struct {
	t time.Time
}

// NewDate constructs a Date from a time.Time, stripping the time component.
func NewDate(t time.Time) Date {
	y, m, d := t.Date()
	return Date{t: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses a "YYYY-MM-DD" string into a Date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	return Date{t: t}, nil
}

// MustParseDate parses s or panics. Intended for test fixtures and package-level vars.
func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// String returns the date in "YYYY-MM-DD" format.
func (d Date) String() string {
	return d.t.Format(dateLayout)
}

// Equal reports whether d and other represent the same calendar date.
func (d Date) Equal(other Date) bool {
	return d.t.Equal(other.t)
}

// MarshalJSON encodes the date as a JSON string "YYYY-MM-DD".
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.t.Format(dateLayout))
}

// UnmarshalJSON decodes a JSON string "YYYY-MM-DD" into Date.
// Returns an error for values that do not match the expected layout.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("invalid date %q: expected YYYY-MM-DD", s)
	}
	d.t = t
	return nil
}

// Value implements driver.Valuer so Date can be passed as a SQL query parameter.
func (d Date) Value() (driver.Value, error) {
	return d.t, nil
}

// Scan implements sql.Scanner so Date can be populated from a SQL column.
// Accepts time.Time (from pgx DATE columns), string, or []byte (from mocks/text drivers).
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		d.t = NewDate(v).t
		return nil
	case string:
		t, err := time.Parse(dateLayout, v)
		if err != nil {
			return fmt.Errorf("scanning date %q: expected YYYY-MM-DD", v)
		}
		d.t = t
		return nil
	case []byte:
		t, err := time.Parse(dateLayout, string(v))
		if err != nil {
			return fmt.Errorf("scanning date %q: expected YYYY-MM-DD", string(v))
		}
		d.t = t
		return nil
	default:
		return fmt.Errorf("unsupported type for Date.Scan: %T", src)
	}
}
