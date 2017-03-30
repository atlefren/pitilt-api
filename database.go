package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/pborman/uuid"
	"net/http"
)

type Database struct {
	db *sqlx.DB
}

/*
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
	//TODO some errors here..
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
*/
func (db *Database) readDataFromPlot(user string, plotId int) ([]Measurement, error) {
	measurements := []Measurement{}
	var sql = `
        WITH instruments as (
            SELECT name FROM
            instrument
            WHERE plot = $1
        )
        SELECT m.name, m.type, m.value, m.timestamp 
        FROM measurement m, plot p 
        WHERE m.timestamp >= p.start_time 
        AND(p.end_time is null OR m.timestamp <= p.end_time) 
        AND m.name IN (SELECT name from instruments)
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
            SELECT i.name AS names
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
        SELECT m.name, m.type, m.value, m.timestamp 
        FROM measurement m, plot p 
        WHERE m.timestamp >= p.start_time 
        and m.timestamp = (select timestamp from latest_measurement)
        AND m.name IN (SELECT i.names from instruments i)
        AND p.id = 3
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
            SELECT name AS names 
            FROM instrument 
            WHERE plot = $1
        )
        SELECT
            m.name,
            m.type,
            round(cast(avg(value) as numeric),0) AS value,
            m.timestamp::date::timestamp + make_interval(hours => DATE_PART('HOUR', m.timestamp)::integer) as timestamp 
        FROM measurement m, plot p 
        WHERE m.timestamp >= p.start_time 
        AND m.name IN (SELECT i.names from instruments i)
        AND p.id = 3
        AND p.login = $2
        GROUP BY m.name, m.type, timestamp
        ORDER BY timestamp;
    `
	err := db.db.Select(&measurements, sql, plotId, user)
	return measurements, err
}

func (db *Database) readMeasurements(user string, name string) ([]Measurement, error) {
	measurements := []Measurement{}

	var sql = `
        SELECT name, type, value, timestamp
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
        INSERT INTO measurement (name, type, value, timestamp, login)
        VALUES (:name, :type, :value, :timestamp, :login)
    `
	for _, measurement := range measurements {
		measurement.Login = user
		tx.NamedExec(sql, &measurement)
	}
	a := tx.Commit()
	fmt.Println(a)
	return nil
}

func (db *Database) getUser(r *http.Request) (string, error) {
	key := r.Header.Get("X-PYTILT-KEY")
	fmt.Println(key)
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
