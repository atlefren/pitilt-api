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

func (db *Database) getUser(r *http.Request) (string, error) {
	fmt.Println("?")
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
