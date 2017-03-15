package main
import (
    "time"
    "database/sql/driver"
    "strings"
    "errors"
)


type Measurement struct {
    Color string `db:"color"`
    Timestamp IsoTime `db:"ts"`
    Gravity int64 `db:"gravity"`
    Temp float64 `db:"temperature"`
    User string `db:"login"`
}

type IsoTime struct {
    time.Time
}

func (isotime *IsoTime) UnmarshalJSON(b []byte) (err error) {
    s := strings.Trim(string(b), "\"")
    if s == "null" {
       isotime.Time = time.Time{}
       return
    }
    isotime.Time, err = time.Parse("2006-01-02T15:04:05", s)
    return
}

func (t IsoTime) Value() (driver.Value, error) {
    return t.Time, nil
}

func (isotime *IsoTime) Scan(src interface{}) error {
    switch src.(type) {
        case time.Time:
            *isotime = IsoTime{src.(time.Time)}
        //case nil:
        //    *isotime = nil
        default:
            return errors.New("Incompatible type for IsoTime")
    }
    return nil
}