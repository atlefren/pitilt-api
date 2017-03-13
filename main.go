package main

import (
    "github.com/gorilla/mux"
    "fmt"
    "log"
    "time"
    "encoding/json"
    "net/http"
    "errors"
)

type Measurement struct {
    Color string
    Timestamp string
    Gravity int64
    Temp float64
}


func readMeasurements(user int, color string) ([]Measurement, error) {
    //TODO: get all measurements for tilt of color <color> for user id <user> sort by timestamp
    var measurements = []Measurement{Measurement{color, "2017-03-13T23:24:13.391482", 1000, 20.00}}
    return measurements, nil
}

func readLastMeasurement(user int, color string) (Measurement, error) {
    //TODO: get the last measurements for user and color
    var measurement = Measurement{color, "2017-03-13T23:24:13.391482", 1000, 20.00}
    return measurement, nil
}

func saveMeasurements(measurements []Measurement, user int) error {
    //todo: save these measurements
    for _, measurement := range measurements {
        fmt.Println(measurement.Timestamp, measurement.Gravity, measurement.Temp)
    }
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
    return  measurements
}

func getUser(r *http.Request) (int, error){
    key := r.Header.Get("X-PYTILT-KEY")
    if (key == "YOUR_KEY") {
        return 1, nil
    }
    return -1, errors.New("unknown key")
}

func addData(w http.ResponseWriter, r *http.Request) {

    user, err := getUser(r)
    if err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    measurements := parseMeasurements(r)

    err = saveMeasurements(measurements, user)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
}

func getAllData(w http.ResponseWriter, r *http.Request) {

    //todo: the user should come from an OAuth token, not the key
    user, err := getUser(r)

    if err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    vars := mux.Vars(r)
    color := vars["color"]

    measurements, err := readMeasurements(user, color)
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

func getLatestData(w http.ResponseWriter, r *http.Request) {

    //todo: the user should come from an OAuth token, not the key
    user, err := getUser(r)

    if err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    vars := mux.Vars(r)
    color := vars["color"]

    measurement, err := readLastMeasurement(user, color)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
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

    //TODO Add CORS support
    r := mux.NewRouter()

    //todo: add route that lets a new oauth token generate a key
    r.HandleFunc("/", http.HandlerFunc(addData)).Methods("POST")
    r.HandleFunc("/data/all/{color}", http.HandlerFunc(getAllData)).Methods("GET")
    r.HandleFunc("/data/latest/{color}", http.HandlerFunc(getLatestData)).Methods("GET")


    s := &http.Server{
        Addr:           ":8080",
        Handler:        r,
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   10 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }
    log.Fatal(s.ListenAndServe())
}
