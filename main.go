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
	measurements := parseMeasurements(r)

	err := env.db.saveMeasurements(measurements, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusOK)
}

func (env *Env) getAllData(w http.ResponseWriter, r *http.Request) {

	user := context.Get(r, "user").(string)

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

	user := context.Get(r, "user").(string)

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
	user := context.Get(r, "user").(string)

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
		dbUri = "postgres://dvh2user:pass@localhost:15432/dvh2"
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
	handle(r, "POST", "/", env.addData, securityHandler.KeyCheckHandler)
	handle(r, "GET", "/data/latest/{color}", env.getLatestData, securityHandler.JwtCheckHandler)
	handle(r, "GET", "/data/hourly/{color}", env.getHourlyData, securityHandler.JwtCheckHandler)
	handle(r, "GET", "/data/all/{color}", env.getAllData, securityHandler.JwtCheckHandler)

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
