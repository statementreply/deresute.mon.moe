package apiclient

import (
	"fmt"
	"time"
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
