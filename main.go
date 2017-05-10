package main

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"
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
			log.WithFields(log.Fields{"timeUntilExpiry": timeUntilExpiration}).Info("Caching publice keys")

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
		plot.Values[measurement.Name] = measurement.Value
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

func getUser(r *http.Request) string {
	return context.Get(r, "user").(string)
	//return "CzWRnH3U5TUV4CLFniC4NfMyYlC3"

}

type Env struct {
	db *Database
}

func (env *Env) addMeasurements(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
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

func (env *Env) getAllData(w http.ResponseWriter, r *http.Request) {

	user := getUser(r)

	vars := mux.Vars(r)
	plotId, err := strconv.Atoi(vars["plotId"])

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	measurements, err := env.db.readDataFromPlot(user, plotId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plots := mapMeasurements(measurements)

	jsonData, err := json.Marshal(plots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getHourlyData(w http.ResponseWriter, r *http.Request) {

	user := getUser(r)

	vars := mux.Vars(r)
	plotId, err := strconv.Atoi(vars["plotId"])

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	measurements, err := env.db.readHourlyDataFromPlot(user, plotId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plots := mapMeasurements(measurements)
	jsonData, err := json.Marshal(plots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getLatestData(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)

	vars := mux.Vars(r)
	plotId, err := strconv.Atoi(vars["plotId"])

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	jsonData, err := json.Marshal(plots[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func (env *Env) getPlots(w http.ResponseWriter, r *http.Request) {

	user := getUser(r)

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

	user := getUser(r)
	vars := mux.Vars(r)
	plotId, err := strconv.Atoi(vars["plotId"])

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
	return plot, nil
}

func (env *Env) addPlot(w http.ResponseWriter, r *http.Request) {

	user := getUser(r)

	plot, err := parsePlot(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func (env *Env) getKey(w http.ResponseWriter, r *http.Request) {
	userId := getUser(r)
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

	var err error

	//setup database
	var dbUri = os.Getenv("DATABASE_URI")
	if dbUri == "" {
		dbUri = "postgres://tilt:password@localhost:15432/tilt"
	}

	db, err := sqlx.Open("postgres", dbUri)
	if err != nil {
		log.Fatalln(err)
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
	plotsRouter.HandleFunc("/plots/{plotId}/data/all/", env.getAllData).Methods("GET")
	plotsRouter.HandleFunc("/plots/{plotId}/data/latest/", env.getLatestData).Methods("GET")
	plotsRouter.HandleFunc("/plots/{plotId}/data/hourly/", env.getHourlyData).Methods("GET")

	plotsRouter.HandleFunc("/plots/", env.getPlots).Methods("GET")
	plotsRouter.HandleFunc("/plots/{plotId}", env.getPlot).Methods("GET")
	plotsRouter.HandleFunc("/plots/", env.addPlot).Methods("POST")

	router.PathPrefix("/plots").Handler(negroni.New(
		jwtCheckHandler,
		negroni.Wrap(plotsRouter),
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
