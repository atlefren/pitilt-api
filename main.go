package main

import (
	"database/sql"
	"encoding/json"
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
	db *Database
}

//SELECT  ts::date as date, DATE_PART('HOUR', ts) as hour, round(avg(gravity), 0) as gravity, round(cast(avg(temperature) as numeric),0) as temperature from measurement group by date, hour order by date,hour;

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

func (env *Env) addData(w http.ResponseWriter, r *http.Request) {

	user, err := env.db.getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	measurements := parseMeasurements(r)

	err = env.db.saveMeasurements(measurements, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusOK)
}

func (env *Env) getAllData(w http.ResponseWriter, r *http.Request) {

	//todo: the user should come from an OAuth token, not the key
	user, err := env.db.getUser(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	color := vars["color"]

	measurements, err := env.db.readMeasurements(user, color)
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

func (env *Env) getHourlyData(w http.ResponseWriter, r *http.Request) {

	//todo: the user should come from an OAuth token, not the key
	user, err := env.db.getUser(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	color := vars["color"]

	measurements, err := env.db.readHourlyMeasurements(user, color)
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

func (env *Env) getLatestData(w http.ResponseWriter, r *http.Request) {

	//todo: the user should come from an OAuth token, not the key
	user, err := env.db.getUser(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	color := vars["color"]

	measurement, err := env.db.readLastMeasurement(user, color)
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

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pitilt :)")
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
	database := &Database{db: db}
	env := &Env{db: database}

	//TODO Add CORS support
	r := mux.NewRouter()

	//todo: add route that lets a new oauth token generate a key
	r.HandleFunc("/", http.HandlerFunc(env.addData)).Methods("POST")
	r.HandleFunc("/", http.HandlerFunc(hello)).Methods("GET")
	r.HandleFunc("/data/latest/{color}", http.HandlerFunc(env.getLatestData)).Methods("GET")
	r.HandleFunc("/data/hourly/{color}", http.HandlerFunc(env.getHourlyData)).Methods("GET")
	r.HandleFunc("/data/all/{color}", http.HandlerFunc(env.getAllData)).Methods("GET")

	s := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
