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

type ShareLink struct {
	PlotId int    `db:"plot_id" json:"-"`
	Uuid   string `json:"uuid"`
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
	Key       string    `db:"key"`
	Timestamp Timestamp `db:"timestamp"`
	Value     float64   `db:"value"`
	Login     string    `db:"login"`
}

type Instrument struct {
	Id   int    `db:"id" json:"-"`
	Name string `db:"name" json:"name"`
	Type string `db:"type" json:"type"`
	Key  string `db:"key" json:"key"`
	Plot int    `db:"plot" json:"-"`
}

type Plot struct {
	Id          int          `db:"id" json:"id"`
	Name        string       `db:"name" json:"name"`
	StartTime   time.Time    `db:"start_time" json:"startTime"`
	EndTime     *time.Time   `db:"end_time" json:"endTime,omitempty"`
	Instruments []Instrument `json:"instruments,omitempty"`
	Login       string       `db:"login" json:"-"`
	Active      bool         `db:"active" json:"active"`
	ShareLink   *string      `json:"sharelink,omitempty"`
}

type User struct {
	Key string `json:"key"`
}
