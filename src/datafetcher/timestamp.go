package datafetcher

import (
	"fmt"
	"time"
	"strconv"
	"log"
)

func RoundTimestamp(in time.Time) time.Time {
	s := in.Unix()
	rounded := ((s-120)/900)*900 + 120
	return time.Unix(rounded, 0)
}

func GetLocalTimestamp() string {
	return TimeToTimestamp(RoundTimestamp(time.Now()))
}

func TimeToTimestamp(t time.Time) string {
	itime := t.Unix()
	timestamp := fmt.Sprintf("%d", itime)
	return timestamp
}

func TimestampToTime(timestamp string) time.Time {
	itime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Println("timestamp format incorrect?", err)
		itime = 0
	}
	tz, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Fatalln("load timezone", err)
	}
	t := time.Unix(itime, 0).In(tz)
	return t

}
