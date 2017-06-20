package main

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/meatballhat/negroni-logrus"
	"github.com/patrickmn/go-cache"
	"github.com/pquerna/cachecontrol"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

var myClient = &http.Client{Timeout: 10 * time.Second}
var myCache = cache.New(5*time.Hour, 10*time.Minute)

func getKey(kid string) (*rsa.PublicKey, error) {
	pemData, found := myCache.Get(kid)
	if found {
		log.WithFields(log.Fields{"id": kid}).Info("Public key found in cache")
	} else {
		log.WithFields(log.Fields{"id": kid}).Info("No public key found in cache")

		// Get the new public keys (pem data) from google
		req, _ := http.NewRequest("GET", "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		// The response contains multiple PEMs
		decoder := json.NewDecoder(res.Body)
		personMap := make(map[string]interface{})
		err = decoder.Decode(&personMap)
		if err != nil {
			return nil, err
		}

		// Try to cache the values for next requests
		reasons, expires, _ := cachecontrol.CachableResponse(req, res, cachecontrol.Options{})
		if len(reasons) == 0 {
			timeUntilExpiration := time.Until(expires)
			log.WithFields(log.Fields{"timeUntilExpiry": timeUntilExpiration}).Info("Caching public keys")

			// Save all the identities
			for id, publicKeyData := range personMap {
				myCache.Set(id, publicKeyData, timeUntilExpiration)
			}
		} else {
			log.Println("Unable to cache public key:", reasons)
		}

		pemData = personMap[kid]
	}

	block, _ := pem.Decode([]byte(pemData.(string)))
	var cert *x509.Certificate
	cert, _ = x509.ParseCertificate(block.Bytes)
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	return rsaPublicKey, nil
}

func parseMeasurements(r *http.Request) ([]Measurement, error) {
	decoder := json.NewDecoder(r.Body)
	var measurements []Measurement
	err := decoder.Decode(&measurements)
	if err != nil {
		return measurements, err
	}
	defer r.Body.Close()
	return measurements, nil
}

func mapMeasurements(measurements []Measurement) []PlotData {
	dates := make(map[time.Time]PlotData)
	for _, measurement := range measurements {
		_, ok := dates[measurement.Timestamp.Time]
		if !ok {
			values := make(map[string]float64)
			plot := PlotData{Date: measurement.Timestamp.Time, Values: values}
			dates[measurement.Timestamp.Time] = plot
		}
		plot, ok := dates[measurement.Timestamp.Time]
		plot.Values[measurement.Key] = measurement.Value
	}

	var plots = []PlotData{}
	for _, plot := range dates {
		plots = append(plots, plot)
	}

	sort.Slice(plots, func(i, j int) bool {
		return plots[i].Date.Sub(plots[j].Date) < 0
	})
	return plots
}

func getUser(r *http.Request) (string, error) {
	user, ok := r.Context().Value("user").(string)
	if !ok {
		return "", errors.New("user not found in context")
	}

	return user, nil
}

func getPlotId(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	plotId, err := strconv.Atoi(vars["plotId"])
	if err != nil {
		return plotId, errors.New("Invalid plot id: " + vars["plotId"])
	}

	return plotId, err
}

func parseDatetime(r *http.Request, key string, defaultValue time.Time) (time.Time, error) {
	vars := r.URL.Query()
	if vals, ok := vars[key]; ok {
		// Expecting only one key for each date time
		if len(vals) != 1 {
			return defaultValue, errors.New("Multiple values for key: " + key)
		}

		// Key is present in request: try to parse it.
		val := vals[0]
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return defaultValue, errors.New("Invalid datetime (should be rfc3339): " + key + ": " + val)
		}
		return t, nil

	} else {
		// Key is not present in request: use default.
		return defaultValue, nil
	}
}

func parseResolution(r *http.Request) (Resolution, error) {
	vars := r.URL.Query()
	if vals, ok := vars["resolution"]; ok {
		// Expecting only one key for resolution
		if len(vals) != 1 {
			return All, errors.New("Multiple values for resolution")
		}

		// Key is present in request: try to parse it.
		val := vals[0]
		return ResolutionFromString(val)
	} else {
		return All, nil
	}
}

type Env struct {
	db *Database
}

func (env *Env) addMeasurements(w http.ResponseWriter, r *http.Request) {
	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	measurements, err := parseMeasurements(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = env.db.saveMeasurements(measurements, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (env *Env) getPlotData(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plotId, err := getPlotId(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	startTime, err := parseDatetime(r, "start", time.Time{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	endTime, err := parseDatetime(r, "end", time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if endTime.Before(startTime) {
		http.Error(w, "Incorrect interval: start time must be before end time.",
			http.StatusBadRequest)
		return
	}

	resolution, err := parseResolution(r)
	if err != nil {
		http.Error(w, "Incorrect resolution.", http.StatusBadRequest)
		return
	}

	start := time.Now()
	measurements, err := env.db.readDataFromPlot(user, plotId, startTime, endTime, resolution)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dbReadTime := time.Since(start)

	start = time.Now()
	plots := mapMeasurements(measurements)
	mappingTime := time.Since(start)

	start = time.Now()
	jsonData, err := json.Marshal(plots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonTime := time.Since(start)

	log.WithFields(log.Fields{
		"database-time": dbReadTime,
		"map-time":      mappingTime,
		"json-time":     jsonTime,
		"user-id":       user,
		"plot-id":       plotId,
		"start-time":    startTime,
		"end-time":      endTime,
		"resolution":    resolution,
	}).Info("Getting plot data")

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getLatestData(w http.ResponseWriter, r *http.Request) {
	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plotId, err := getPlotId(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	measurements, err := env.db.readLatestDataFromPlot(user, plotId)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Color not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	plots := mapMeasurements(measurements)

	if len(plots) == 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	jsonData, err := json.Marshal(plots[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getPlots(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plots, err := env.db.getPlots(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonData, err := json.Marshal(plots)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getPlot(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plotId, err := getPlotId(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plot, err := env.db.getPlot(plotId, user)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Plot not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	instruments, err := env.db.getInstruments(plotId)
	plot.Instruments = instruments

	jsonData, err := json.Marshal(plot)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func parsePlot(r *http.Request) (Plot, error) {

	decoder := json.NewDecoder(r.Body)
	var plot Plot
	err := decoder.Decode(&plot)
	if err != nil {
		return Plot{}, err
	}
	defer r.Body.Close()

	if plot.EndTime != nil && !plot.StartTime.Before(*plot.EndTime) {
		return Plot{}, errors.New("Invalid plot: start time must be before end time.")
	}

	return plot, nil
}

func (env *Env) addPlot(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plot, err := parsePlot(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated_plot, err := env.db.savePlot(plot, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonData, _ := json.Marshal(updated_plot)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonData)
}

func (env *Env) updatePlot(w http.ResponseWriter, r *http.Request) {

	user, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plot, err := parsePlot(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Must use the id from the url, not the json
	plotId, err := getPlotId(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plot.Id = plotId

	updated_plot, err := env.db.updatePlot(plot, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonData, _ := json.Marshal(updated_plot)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

}

func (env *Env) getKey(w http.ResponseWriter, r *http.Request) {
	userId, err := getUser(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	key, err := env.db.getkeyForUser(userId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	user := User{Key: key}
	jsonData, _ := json.Marshal(user)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pitilt :)\n")
}

func main() {

	//setup database
	var dbUri = os.Getenv("DATABASE_URI")
	if dbUri == "" {
		dbUri = "postgres://tilt:password@localhost:15432/tilt"
	}

	db, err := sqlx.Open("postgres", dbUri)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Unable to connect to database")
	}

	err = db.Ping()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("Unable to ping database")
	}

	database := &Database{db: db}
	env := &Env{db: database}

	//create middleware for authing on google jwt from firebase
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			key, err := getKey(token.Header["kid"].(string))
			return key, err
		},
		SigningMethod: jwt.SigningMethodRS256,
	})

	jwtCheckHandler := &JwtCheckHandler{db: database, jwtMiddleware: jwtMiddleware}
	keyCheckHandler := &KeyCheckHandler{db: database}

	//define routes
	router := mux.NewRouter()
	router.HandleFunc("/", hello).Methods("GET")

	measurementRouter := mux.NewRouter()
	measurementRouter.HandleFunc("/measurements/", env.addMeasurements).Methods("POST")
	router.PathPrefix("/measurements").Handler(negroni.New(
		keyCheckHandler,
		negroni.Wrap(measurementRouter),
	))

	plotsRouter := mux.NewRouter()
	plotsRouter.HandleFunc("/plots/{plotId}/data/", env.getPlotData).Methods("GET")
	plotsRouter.HandleFunc("/plots/{plotId}/data/latest/", env.getLatestData).Methods("GET")

	plotsRouter.HandleFunc("/plots/", env.getPlots).Methods("GET")
	plotsRouter.HandleFunc("/plots/{plotId}", env.getPlot).Methods("GET")
	plotsRouter.HandleFunc("/plots/", env.addPlot).Methods("POST")
	plotsRouter.HandleFunc("/plots/{plotId}", env.updatePlot).Methods("PUT")

	router.PathPrefix("/plots").Handler(negroni.New(
		jwtCheckHandler,
		negroni.Wrap(plotsRouter),
	))

	userRouter := mux.NewRouter()
	userRouter.HandleFunc("/user/key/", env.getKey).Methods("GET")

	router.PathPrefix("/user").Handler(negroni.New(
		jwtCheckHandler,
		negroni.Wrap(userRouter),
	))

	// Setup CORS-handling
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		AllowedMethods: []string{"GET", "HEAD", "POST", "PUT", "OPTIONS"},
	})

	n := negroni.New()
	n.Use(negronilogrus.NewMiddleware())
	n.Use(c)
	n.UseHandler(router)

	http.ListenAndServe(":8080", n)
}
