package main

import (
	"database/sql"
	"errors"
	//"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/pborman/uuid"
	"net/http"
)

type Database struct {
	db *sqlx.DB
}

func (db *Database) readDataFromPlot(user string, plotId int) ([]Measurement, error) {
	measurements := []Measurement{}
	var sql = `
        WITH instruments as (
            SELECT key FROM
            instrument
            WHERE plot = $1
        )
        SELECT m.key, m.value, m.timestamp
        FROM measurement m, plot p
        WHERE m.timestamp >= p.start_time
        AND(p.end_time is null OR m.timestamp <= p.end_time)
        AND m.key IN (SELECT key from instruments)
        AND p.id = $1
        AND p.login = $2
        ORDER BY m.timestamp;
    `
	err := db.db.Select(&measurements, sql, plotId, user)
	return measurements, err
}

func (db *Database) readLatestDataFromPlot(user string, plotId int) ([]Measurement, error) {
	measurements := []Measurement{}
	var sql = `
        WITH
        instruments as (
            SELECT i.key AS keys
            FROM instrument i
            WHERE plot = $1
        ),
        latest_measurement as (

            SELECT max(m.timestamp) as timestamp
            FROM measurement m, plot p
            WHERE m.timestamp >= p.start_time
            AND(p.end_time is null OR m.timestamp <= p.end_time)
            AND p.id = $1
        )
        SELECT m.key, m.value, m.timestamp
        FROM measurement m, plot p
        WHERE m.timestamp >= p.start_time
        and m.timestamp = (select timestamp from latest_measurement)
        AND m.key IN (SELECT i.keys from instruments i)
        AND p.id = $1
        AND p.login = $2
        ORDER BY m.timestamp;
    `
	err := db.db.Select(&measurements, sql, plotId, user)
	return measurements, err
}

func (db *Database) readHourlyDataFromPlot(user string, plotId int) ([]Measurement, error) {
	measurements := []Measurement{}
	var sql = `
        WITH instruments as (
            SELECT key AS keys
            FROM instrument
            WHERE plot = $1
        )
        SELECT
            m.key,
            round(cast(avg(value) as numeric),0) AS value,
            m.timestamp::date::timestamp + make_interval(hours => DATE_PART('HOUR', m.timestamp)::integer) as timestamp
        FROM measurement m, plot p
        WHERE m.timestamp >= p.start_time
        AND m.key IN (SELECT i.keys from instruments i)
        AND p.id = $1
        AND p.login = $2
        GROUP BY m.key, timestamp
        ORDER BY timestamp;
    `
	err := db.db.Select(&measurements, sql, plotId, user)
	return measurements, err
}

func (db *Database) readMeasurements(user string, name string) ([]Measurement, error) {
	measurements := []Measurement{}

	var sql = `
        SELECT key, value, timestamp
        FROM measurement
        WHERE name = $1
        AND login = $2
        ORDER BY timestamp
    `

	err := db.db.Select(&measurements, sql, name, user)
	return measurements, err
}

func (db *Database) saveMeasurements(measurements []Measurement, user string) error {
	tx := db.db.MustBegin()
	var sql = `
        INSERT INTO measurement (key, value, timestamp, login)
        VALUES (:key, :value, :timestamp, :login)
    `
	for _, measurement := range measurements {
		measurement.Login = user
		tx.NamedExec(sql, &measurement)
	}

	err := tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) getPlots(user string) ([]Plot, error) {
	plots := []Plot{}

	var sql = `
        SELECT id, start_time, end_time, name
        FROM plot
        WHERE login = $1
        ORDER BY start_time DESC
    `

	err := db.db.Select(&plots, sql, user)
	return plots, err
}

func (db *Database) getInstruments(plotId int) ([]Instrument, error) {
	instruments := []Instrument{}

	var sql = `
        SELECT key, id, name, type
        FROM instrument
        WHERE plot = $1
    `

	err := db.db.Select(&instruments, sql, plotId)
	return instruments, err
}

func (db *Database) getPlot(id int, user string) (Plot, error) {
	plot := Plot{}

	var sql = `
        SELECT id, start_time, end_time, name
        FROM plot
        WHERE login = $1
        AND id = $2
    `

	err := db.db.Get(&plot, sql, user, id)
	return plot, err
}

func (db *Database) savePlot(plot Plot, user string) (Plot, error) {

	plot.Login = user

	var sql = `
        INSERT INTO plot (start_time, end_time, name, login) VALUES (:start_time, :end_time, :name, :login) RETURNING id
    `
	var id int
	rows, err := db.db.NamedQuery(sql, plot)
	if err != nil {
		return plot, err
	}
	if rows.Next() {
		rows.Scan(&id)
	}
	plot.Id = id
	tx := db.db.MustBegin()

	var sql2 = `
        INSERT INTO instrument (key, name, type, plot)
        VALUES (:key, :name, :type, :plot)
    `
	for _, instrument := range plot.Instruments {
		instrument.Plot = plot.Id
		tx.NamedExec(sql2, &instrument)
	}
	tx.Commit()
	return plot, err
}

func (db *Database) getUser(r *http.Request) (string, error) {
	key := r.Header.Get("X-PYTILT-KEY")
	return db.getUserForKey(key)
}

func (db *Database) getUserForKey(key string) (string, error) {
	var id string
	err := db.db.Get(&id, "SELECT id FROM login WHERE key = $1", key)

	if err == sql.ErrNoRows {
		return "", errors.New("unknown key")
	}
	return id, err
}

func (db *Database) getkeyForUser(user string) (string, error) {
	var key string
	err := db.db.Get(&key, "SELECT key FROM login WHERE id = $1", user)

	if err == sql.ErrNoRows {
		return "", errors.New("unknown user")
	}
	return key, err
}

func (db *Database) userExists(id string) (bool, error) {
	var uid string
	if err := db.db.QueryRow("SELECT id FROM login WHERE id = $1", id).Scan(&uid); err == nil {
		return true, nil
	} else if err == sql.ErrNoRows {
		return false, nil
	} else {
		return false, err
	}

}

func (db *Database) createUser(id string, email string, name string) error {
	tx := db.db.MustBegin()
	key := uuid.New()
	var sql = `
        INSERT INTO login (id, first_name, email, key)
        VALUES ($1, $2, $3, $4)
    `
	tx.MustExec(sql, id, name, email, key)
	tx.Commit()
	return nil
}
