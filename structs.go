package main

import (
	"database/sql/driver"
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

type Measurement struct {
	Name      string    `db:"name"`
	Type      string    `db:"type"`
	Timestamp Timestamp `db:"timestamp"`
	Value     float64   `db:"value"`
	Login     string    `db:"login"`
}
