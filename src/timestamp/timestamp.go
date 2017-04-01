// Copyright 2016 GUO Yixuan <culy.gyx@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
		log.Println("TimestampToTime(): timestamp format incorrect?", err)
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

func FormatDate(t time.Time) string {
	st := t.In(tz).Format("060102")
	return st
}

// for datafetcher
func RoundTimestamp(in time.Time) time.Time {
	s := in.Unix()
	// FIXME: time step hardcode
	rounded := ((s-120)/900)*900 + 120
	return time.Unix(rounded, 0)
}

// FIXME: time step hardcode
func IsWholeHour(timestamp string) bool {
	t := TimestampToTime(timestamp)
	if (t.Minute() >=0 && t.Minute() <15) {
		return true
	} else {
		return false
	}
}

func GetLocalTimestamp() string {
	return TimeToTimestamp(RoundTimestamp(time.Now()))
}
