package main

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/urfave/negroni"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

type Env struct {
	db *Database
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

func (env *Env) addData(w http.ResponseWriter, r *http.Request) {

	user := context.Get(r, "user").(string)
	fmt.Println(user)
	measurements := parseMeasurements(r)

	for _, measurement := range measurements {
		fmt.Println(measurement.Name)
		fmt.Println(measurement.Value)
		fmt.Println(measurement.Type)
		fmt.Println(measurement.Timestamp.Time)
		fmt.Println("----")
	}
	err := env.db.saveMeasurements(measurements, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
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

func (env *Env) getAllData(w http.ResponseWriter, r *http.Request) {

	//user := context.Get(r, "user").(string)
	user := "1"

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

	//user := context.Get(r, "user").(string)
	user := "1"

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
	//user := context.Get(r, "user").(string)
	user := "1"

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

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pitilt :)\n")
}

var myClient = &http.Client{Timeout: 10 * time.Second}

func getKey(kid string) (*rsa.PublicKey, error) {
	r, err := myClient.Get("https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com")
	if err != nil {
		return nil, err
	}
	//TODO:
	// Use the value of max-age in the Cache-Control header of the response from that endpoint to know when to refresh the public keys.

	decoder := json.NewDecoder(r.Body)
	personMap := make(map[string]interface{})
	err = decoder.Decode(&personMap)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode([]byte(personMap[kid].(string)))
	var cert *x509.Certificate
	cert, _ = x509.ParseCertificate(block.Bytes)
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	return rsaPublicKey, nil
}

func handle(r *mux.Router, method string, path string, handlerFunc func(w http.ResponseWriter, r *http.Request), middleware func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc)) {

	if middleware == nil {
		r.HandleFunc(path, http.HandlerFunc(handlerFunc)).Methods(method)
	} else {
		r.Handle(path, negroni.New(
			negroni.HandlerFunc(middleware),
			negroni.Wrap(http.HandlerFunc(handlerFunc)),
		)).Methods(method)
	}
}

func main() {

	var err error

	//setup database
	var dbUri = os.Getenv("DATABASE_URI")
	if dbUri == "" {
		dbUri = "postgres://postgres:nisse@localhost:5433/pitilt?sslmode=disable"
	}

	db, err := sqlx.Open("postgres", dbUri)
	if err != nil {
		log.Fatalln(err)
	}
	database := &Database{db: db}
	env := &Env{db: database}

	//create router
	r := mux.NewRouter()

	//create middleware for authing on google jwt from firebase
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			key, err := getKey(token.Header["kid"].(string))
			return key, err
		},
		SigningMethod: jwt.SigningMethodRS256,
	})

	//create a securituhandler that handles both keys and jwt
	securityHandler := &SecurityHandler{db: database, jwtMiddleware: jwtMiddleware}

	//define routes
	handle(r, "GET", "/", hello, nil)
	handle(r, "POST", "/data", env.addData, securityHandler.KeyCheckHandler)
	handle(r, "GET", "/plot/{plotId}/data/all", env.getAllData, nil)
	handle(r, "GET", "/plot/{plotId}/data/latest", env.getLatestData, nil)
	handle(r, "GET", "/plot/{plotId}/data/hourly", env.getHourlyData, nil)

	//setup CORS-handling
	corsObj := handlers.AllowedOrigins([]string{"*"})
	headersOk := handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})
	corsHandler := handlers.CORS(corsObj, headersOk, methodsOk)(r)

	//setup server
	s := &http.Server{
		Addr:           ":8080",
		Handler:        corsHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())

}
