package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Timestamp struct {
	time.Time
}

type PlotData struct {
	Date   time.Time          `json:"date"`
	Values map[string]float64 `json:"values"`
}

func (t *Timestamp) MarshalJSON() ([]byte, error) {
	ts := t.Time.Unix()
	stamp := fmt.Sprint(ts)

	return []byte(stamp), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	ts, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}

	t.Time = time.Unix(int64(ts), 0)

	return nil
}

func (t Timestamp) Value() (driver.Value, error) {
	return t.Time, nil
}

func (isotime *Timestamp) Scan(src interface{}) error {
	switch src.(type) {
	case time.Time:
		*isotime = Timestamp{src.(time.Time)}
	default:
		return errors.New("Incompatible type for IsoTime")
	}
	return nil
}

type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return json.Marshal(nil)
	}
	return json.Marshal(nt.Time)
}

type Measurement struct {
	Name      string    `db:"name"`
	Type      string    `db:"type"`
	Timestamp Timestamp `db:"timestamp"`
	Value     float64   `db:"value"`
	Login     string    `db:"login"`
}

type Instrument struct {
	Id   int    `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
	Type string `db:"type" json:"type"`
}

type Plot struct {
	Id          int          `db:"id" json:"id"`
	Name        string       `db:"name" json:"name"`
	StartTime   time.Time    `db:"start_time" json:"startTime"`
	EndTime     NullTime     `db:"end_time" json:"endTime"`
	Instruments []Instrument `json:"instruments,omitempty"`
}
