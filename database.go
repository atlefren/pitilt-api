package main

import (
	"database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"net/http"
	"strings"
	"time"
)

type Database struct {
	db *sqlx.DB
}

type Resolution int

const (
	All Resolution = iota
	Day
	Hour
	Minute
)

func (resolution Resolution) ToString() (string, error) {
	switch resolution {
	case All:
		return "All", nil
	case Minute:
		return "Minute", nil
	case Hour:
		return "Hour", nil
	case Day:
		return "Day", nil
	default:
		return "", errors.New("Unknown resolution")
	}
}

func (resolution Resolution) String() string {
	str, _ := resolution.ToString()
	return str
}

func ResolutionFromString(resolutionString string) (Resolution, error) {
	lowerCase := strings.ToLower(resolutionString)
	if lowerCase == strings.ToLower("All") {
		return All, nil
	} else if lowerCase == strings.ToLower("Minute") {
		return Minute, nil
	} else if lowerCase == strings.ToLower("Hour") {
		return Hour, nil
	} else if lowerCase == strings.ToLower("Day") {
		return Day, nil
	} else {
		return All, errors.New("Unknown resolution from string: " + resolutionString)
	}
}

func (db *Database) readDataFromPlot(plotId int, startTime time.Time, endTime time.Time, resolution Resolution) ([]Measurement, error) {

	if resolution == All {
		return db.readAllDataFromPlot(plotId, startTime, endTime)
	} else {
		return db.readAggregatedDataFromPlot(plotId, startTime, endTime, resolution)
	}
}

func (db *Database) readAllDataFromPlot(plotId int, startTime time.Time, endTime time.Time) ([]Measurement, error) {
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
        AND m.timestamp >= $2
        AND m.timestamp <= $3
        ORDER BY m.timestamp desc;
    `
	err := db.db.Select(&measurements, sql, plotId, startTime, endTime)
	return measurements, err
}

func (db *Database) readLatestDataFromPlot(plotId int) ([]Measurement, error) {
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
        ORDER BY m.timestamp desc;
    `
	err := db.db.Select(&measurements, sql, plotId)
	return measurements, err
}

func (db *Database) getIntervalDefinition(resolution Resolution) (string, string, error) {
	switch resolution {
	case All:
		return "", "", errors.New("No interval definition for All")
	case Minute:
		return "mins", "1 minute", nil
	case Hour:
		return "hours", "1 hour", nil
	case Day:
		return "days", "1 day", nil
	default:
		return "", "", errors.New("Unknown resolution")
	}
}

func (db *Database) readAggregatedDataFromPlot(plotId int, startTime time.Time, endTime time.Time, resolution Resolution) ([]Measurement, error) {
	measurements := []Measurement{}
	var sql = `
        WITH instruments as (
            SELECT key AS keys
            FROM instrument
            WHERE plot = $1
        ),
        intervals AS (
            SELECT start_time FROM
            generate_series(date_trunc($4, GREATEST($2, (select start_time from plot where id = $1))),
                            LEAST($3, NOW()),
                            $5) as start_time
        )
        SELECT
            m.key,
            i.start_time as timestamp,
            round(AVG(m.value)::numeric, 2) as value
        FROM measurement m, plot p, intervals i
        WHERE m.timestamp >= p.start_time
        AND(p.end_time is null OR m.timestamp <= p.end_time)
        AND m.key IN (SELECT keys from instruments)
        AND p.id = $1
        AND m.timestamp > i.start_time
        AND m.timestamp < i.start_time + $5::interval
        AND m.timestamp >= $2
        AND m.timestamp <= $3
        GROUP BY m.key, i.start_time
        ORDER BY i.start_time desc
    `

	trunc, interval, err := db.getIntervalDefinition(resolution)
	if err != nil {
		return measurements, err
	}

	err = db.db.Select(&measurements, sql, plotId, startTime, endTime, trunc, interval)
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
	tx, err := db.db.Beginx()
	if err != nil {
		return errors.Wrap(err, "Unable to save measurement")
	}

	var sql = `
        INSERT INTO measurement (key, value, timestamp, login)
        VALUES (:key, :value, :timestamp, :login)
    `
	for _, measurement := range measurements {
		measurement.Login = user
		tx.NamedExec(sql, &measurement)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "Unable to save measurement")
	}
	return nil
}

func (db *Database) getPlots(user string) ([]Plot, error) {
	plots := []Plot{}

	var sql = `
        SELECT id, start_time, end_time, name, case when end_time IS null then true else false end as active, s.uuid as sharelink
        FROM plot
        LEFT JOIN sharelink as s
        ON plot.id = s.plot_id
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

func (db *Database) getPlot(id int) (Plot, error) {
	plot := Plot{}

	var sql = `
        SELECT id, start_time, end_time, name, case when end_time IS null then true else false end as active, s.uuid as sharelink
        FROM plot
        LEFT JOIN sharelink as s
        ON plot.id = s.plot_id
        WHERE id = $1
    `

	err := db.db.Get(&plot, sql, id)
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
		return plot, errors.Wrap(err, "Unable to save plot")
	}
	if rows.Next() {
		rows.Scan(&id)
	}
	plot.Id = id
	tx, err := db.db.Beginx()
	if err != nil {
		return plot, errors.Wrap(err, "Unable to save instruments for plot")
	}

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

func (db *Database) updatePlot(plot Plot, user string) (Plot, error) {

	plot.Login = user

	var sql = `
        UPDATE plot SET start_time = :start_time, end_time = :end_time, name = :name WHERE id = :id and login = :login
    `
	_, err := db.db.NamedQuery(sql, plot)
	if err != nil {
		return plot, err
	}
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

func (db *Database) checkIfUserOwnsPlot(user string, plotId int) (bool, error) {
	var count int
	err := db.db.Get(&count, "SELECT COUNT(*) FROM plot WHERE login = $1 AND id = $2", user, plotId)

	if err == sql.ErrNoRows {
		// No plot with this combination of owner and id exists
		return false, nil
	}

	// There should be only one plot
	return count == 1, err
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
		return false, errors.Wrap(err, "Unable to check if user exists")
	}

}

func (db *Database) createUser(id string, email string, name string) error {
	tx, error := db.db.Beginx()
	if error != nil {
		return errors.New("Unable to connect to database.")
	}

	key := uuid.New()
	var sql = `
        INSERT INTO login (id, name, email, key)
        VALUES ($1, $2, $3, $4)
    `
	_, error = tx.Exec(sql, id, name, email, key)
	if error != nil {
		return errors.New("Unable to create new user")
	}

	tx.Commit()
	return nil
}

func (db *Database) addShareLink(plotId int, user string) (ShareLink, error) {
	shareLink := ShareLink{}

	tx, error := db.db.Beginx()
	if error != nil {
		return shareLink, errors.New("Unable to connect to database.")
	}

	shareLink.PlotId = plotId
	shareLink.Uuid = uuid.New()
	var sql = `
        INSERT INTO sharelink (plot_id, uuid)
        VALUES ($1, $2)
    `
	_, error = tx.Exec(sql, shareLink.PlotId, shareLink.Uuid)
	if error != nil {
		return shareLink, errors.New("Unable to create share link")
	}

	tx.Commit()
	return shareLink, nil
}

func (db *Database) getShareLink(plotId int, user string) (*ShareLink, error) {
	shareLink := ShareLink{}

	var sqlSelect = `
        SELECT plot_id, uuid
        FROM sharelink
        WHERE plot_id = $1
    `

	err := db.db.Get(&shareLink, sqlSelect, plotId)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &shareLink, err
}

func (db *Database) removeShareLink(plotId int, user string) error {
	var sql = `
        DELETE
        FROM sharelink
        WHERE plot_id = $1
    `
	_, err := db.db.Exec(sql, plotId)
	return err
}

func (db *Database) getShareLinkFromUuid(uuidString string) (*ShareLink, error) {
	// Verify uuid format
	parsedUuid := uuid.Parse(uuidString)
	if parsedUuid == nil {
		return nil, errors.New("Invalid uuid format")
	}

	shareLink := ShareLink{}

	var sqlSelect = `
        SELECT plot_id, uuid
        FROM sharelink
        WHERE uuid = $1
    `

	err := db.db.Get(&shareLink, sqlSelect, uuidString)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &shareLink, err
}
