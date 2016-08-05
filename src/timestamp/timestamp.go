package timestamp

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

var tz = loadTZ()

func loadTZ() *time.Location {
	tz, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatalln("set timezone", err)
	}
	return tz
}

// debug
func TZ() *time.Location {
	return tz
}

func TimestampToTime(timestamp string) time.Time {
	itime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Println("timestamp format incorrect?", err)
		itime = 0
	}
	t := time.Unix(itime, 0).In(tz)
	return t
}

func TimeToTimestamp(t time.Time) string {
	itime := t.Unix()
	timestamp := fmt.Sprintf("%d", itime)
	return timestamp
}

func FormatTimestamp(timestamp string) string {
	t := TimestampToTime(timestamp)
	st := t.Format(time.RFC3339)
	return st
}

func FormatTimestamp_short(timestamp string) string {
	t := TimestampToTime(timestamp)
	st := t.Format("01/02 15:04")
	return st
}

func FormatTime(t time.Time) string {
	st := t.Format("2006-01-02 15:04")
	return st
}

func FormatTime_short(t time.Time) string {
	st := t.Format("060102 15:04")
	return st
}

// for datafetcher
func RoundTimestamp(in time.Time) time.Time {
	s := in.Unix()
	rounded := ((s-120)/900)*900 + 120
	return time.Unix(rounded, 0)
}

func GetLocalTimestamp() string {
	return TimeToTimestamp(RoundTimestamp(time.Now()))
}
