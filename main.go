package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"time"
)

type Env struct {
	db *sqlx.DB
}

func readMeasurements(user string, color string, db *sqlx.DB) ([]Measurement, error) {
	measurements := []Measurement{}
	err := db.Select(&measurements, "SELECT color, ts, gravity, temperature FROM measurement WHERE color = $1 AND login = $2  ORDER BY ts", color, user)
	return measurements, err
}

func readLastMeasurement(user string, color string, db *sqlx.DB) (Measurement, error) {
	measurement := Measurement{}
	err := db.Get(&measurement, "SELECT color, ts, gravity, temperature FROM measurement WHERE color = $1 AND login = $2 ORDER BY ts DESC LIMIT 1", color, user)
	return measurement, err
}

func saveMeasurements(measurements []Measurement, user string, db *sqlx.DB) error {
	tx := db.MustBegin()
	for _, measurement := range measurements {
		measurement.User = user
		tx.NamedExec("INSERT INTO measurement (color, gravity, temperature, ts, login) VALUES (:color, :gravity, :temperature, :ts, :login)", &measurement)
	}
	tx.Commit()
	return nil
}

func parseMeasurements(r *http.Request) []Measurement {
	decoder := json.NewDecoder(r.Body)
	var measurements []Measurement
	err := decoder.Decode(&measurements)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()
	return measurements
}

func getUser(r *http.Request, db *sqlx.DB) (string, error) {
	key := r.Header.Get("X-PYTILT-KEY")

	var id string
	err := db.Get(&id, "SELECT id FROM login WHERE key = $1", key)

	if err == sql.ErrNoRows {
		return "", errors.New("unknown key")
	}
	return id, err
}

func (env *Env) addData(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r, env.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	measurements := parseMeasurements(r)

	err = saveMeasurements(measurements, user, env.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusOK)
}

func (env *Env) getAllData(w http.ResponseWriter, r *http.Request) {

	//todo: the user should come from an OAuth token, not the key
	user, err := getUser(r, env.db)

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	color := vars["color"]

	measurements, err := readMeasurements(user, color, env.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(measurements)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pitilt :)")
}

func (env *Env) getLatestData(w http.ResponseWriter, r *http.Request) {

	//todo: the user should come from an OAuth token, not the key
	user, err := getUser(r, env.db)

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	color := vars["color"]

	measurement, err := readLastMeasurement(user, color, env.db)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Color not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	jsonData, err := json.Marshal(measurement)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func main() {
	var err error

	var dbUri = os.Getenv("DATABASE_URI")
	if dbUri == "" {
		dbUri = "postgres://dvh2user:pass@localhost:15432/dvh2"
	}

	db, err := sqlx.Open("postgres", dbUri)
	if err != nil {
		log.Fatalln(err)
	}
	env := &Env{db: db}

	//TODO Add CORS support
	r := mux.NewRouter()

	//todo: add route that lets a new oauth token generate a key
	r.HandleFunc("/", http.HandlerFunc(env.addData)).Methods("POST")
	r.HandleFunc("/", http.HandlerFunc(hello)).Methods("GET")
	r.HandleFunc("/data/all/{color}", http.HandlerFunc(env.getAllData)).Methods("GET")
	r.HandleFunc("/data/latest/{color}", http.HandlerFunc(env.getLatestData)).Methods("GET")

	s := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
