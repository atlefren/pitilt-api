package main

import (
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"net/http"
)

type Database struct {
	db *sqlx.DB
}

func (db *Database) readMeasurements(user string, color string) ([]Measurement, error) {
	measurements := []Measurement{}

	var sql = `
        SELECT color, ts, gravity, temperature
        FROM measurement
        WHERE color = $1
        AND login = $2
        ORDER BY ts
    `

	err := db.db.Select(&measurements, sql, color, user)
	return measurements, err
}

func (db *Database) readHourlyMeasurements(user string, color string) ([]Measurement, error) {
	measurements := []Measurement{}

	var sql = `
        SELECT
            color,
            ts::date::timestamp + make_interval(hours => DATE_PART('HOUR', ts)::integer) as ts,
            round(avg(gravity), 0) AS gravity,
            round(cast(avg(temperature) as numeric),0) AS temperature
        FROM measurement
        WHERE color = $1
        AND login = $2
        GROUP by ts, color
        ORDER by ts;
    `

	err := db.db.Select(&measurements, sql, color, user)
	return measurements, err
}

func (db *Database) readLastMeasurement(user string, color string) (Measurement, error) {
	measurement := Measurement{}
	var sql = `
        SELECT color, ts, gravity, temperature
        FROM measurement
        WHERE color = $1
        AND login = $2
        ORDER BY ts DESC
        LIMIT 1
    `
	err := db.db.Get(&measurement, sql, color, user)
	return measurement, err
}

func (db *Database) saveMeasurements(measurements []Measurement, user string) error {
	tx := db.db.MustBegin()
	var sql = `
        INSERT INTO measurement (color, gravity, temperature, ts, login)
        VALUES (:color, :gravity, :temperature, :ts, :login)
    `
	for _, measurement := range measurements {
		measurement.User = user
		tx.NamedExec(sql, &measurement)
	}
	tx.Commit()
	return nil
}

func (db *Database) getUser(r *http.Request) (string, error) {
	key := r.Header.Get("X-PYTILT-KEY")

	var id string
	err := db.db.Get(&id, "SELECT id FROM login WHERE key = $1", key)

	if err == sql.ErrNoRows {
		return "", errors.New("unknown key")
	}
	return id, err
}
