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

package rankserver

import (
	"database/sql"
	"fmt"
	//sqlite3 "github.com/mattn/go-sqlite3"
	"resource_mgr"
	//"log"
	"sort"
	"time"
	ts "timestamp"
)

// tag: database, sqlite
func (r *RankServer) UpdateTimestamp() {
	rows, err := r.db.Query("SELECT timestamp FROM timestamp;")
	if err != nil {
		r.logger.Println("sql error UpdateTimestamp", err)
		return
	}
	defer rows.Close()
	var local_list_timestamp []string
	for rows.Next() {
		var ts string
		err = rows.Scan(&ts)
		if err != nil {
			r.logger.Println("sql error UpdateTimestamp", err)
			return
		}
		local_list_timestamp = append(local_list_timestamp, ts)
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error UpdateTimestamp", err)
		return
	}
	r.mux_timestamp.Lock()
	r.list_timestamp = local_list_timestamp
	sort.Strings(r.list_timestamp)
	r.mux_timestamp.Unlock()
}

// true: nonempty; false: empty
// tag: database, sqlite
func (r *RankServer) checkDir(timestamp string) bool {
	var ts_discard string
	row := r.db.QueryRow("SELECT timestamp FROM rank WHERE timestamp == $1 LIMIT 1;", timestamp)
	err := row.Scan(&ts_discard)
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		} else {
			r.logger.Println("sql error checkDir", err)
			return false
		}
	} else {
		// row exists
		return true
	}
}

// tag: database, sqlite
func (r *RankServer) CheckData() {
	r.UpdateTimestamp()
	latest := r.latestTimestamp()
	latest_time := time.Unix(0, 0)
	if latest != "" {
		latest_time = ts.TimestampToTime(latest)
	}
	// check new res_ver
	// FIXME need some test
	// FIXME race condition

	if // check every 1 hour
	(time.Now().Sub(r.lastCheck) >= 1*time.Hour) ||
		// if currentEvent is not defined, then every 2 min
		((r.currentEvent == nil) && (time.Now().Sub(r.lastCheck) >= 10*time.Minute)) ||
		// if currentEvent is defined but expired, then immediately
		((r.currentEvent != nil) && !r.currentEvent.InPeriod(time.Now())) {
		r.logger.Println("recheck res_ver, lastcheck:", r.lastCheck, "latest_time:", latest_time)
		r.lastCheck = time.Now()
		// try to restart session (session expires after certain time)
		r.client.Reset_sid()
		old_rv := r.client.Get_res_ver()
		r.client.LoadCheck()
		rv := r.client.Get_res_ver()
		if rv != old_rv {
			r.resourceMgr.Set_res_ver(rv)
			r.logger.Println("new res_ver:", rv)
			r.resourceMgr.ParseEvent()
		}
		// FIXME: depends on current time, update even if res_ver doesn't change
		r.currentEvent = r.resourceMgr.FindCurrentEvent()
		r.latestEvent = r.resourceMgr.FindLatestEvent()
		r.logger.Println("currentEvent", r.currentEvent)
	}
}

// tag: database
func (r *RankServer) fetchData(timestamp string, rankingType int, rank int) int {
	var score int
	row := r.db.QueryRow("SELECT score FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3 LIMIT 1;", timestamp, rankingType+1, rank)
	err := row.Scan(&score)
	if err != nil {
		if err == sql.ErrNoRows {
			score = -1
		} else {
			if err != nil {
				r.logger.Println("sql error fetchData", err)
				score = -1
			}
		}
	}
	//log.Println(timestamp, rankingType, rank, score)
	return score
}

// tag: database
func (r *RankServer) fetchDataListRank(timestamp string, rankingType int) []int {
	var listRank []int
	rows, err := r.db.Query("SELECT rank FROM rank WHERE timestamp == $1 AND type == $2;", timestamp, rankingType+1)
	if err != nil {
		r.logger.Println("sql error fetchDataListRank", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var rank int
		err = rows.Scan(&rank)
		if err != nil {
			r.logger.Println("sql error fetchDataListRank", err)
			return nil
		}
		// FIXME: remove 10k+1 restriction
		//if rank%10 == 1 {
			listRank = append(listRank, rank)
		//}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error fetchDataListRank", err)
		return nil
	}
	return listRank
}

// tag: database
func (r *RankServer) fetchDataSlice(timestamp string) []map[int]int {
	slice := make([]map[int]int, 2)
	slice[0] = map[int]int{}
	slice[1] = map[int]int{}

	rows, err := r.db.Query("SELECT type, rank, score FROM rank WHERE timestamp == $1;", timestamp)
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var rankingType int
		var rank int
		var score int
		err = rows.Scan(&rankingType, &rank, &score)
		if err != nil {
			r.logger.Println("sql error", err)
			return nil
		}
		rankingType -= 1
		// FIXME: remove 10k+1 restriction
		//if rank%10 == 1 {
			slice[rankingType][rank] = score
		//}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return slice
}

// SELECT timestamp, score FROM rank WHERE type == 1 AND rank == 120001 AND timestamp BETWEEN 1470622619 AND 1470644220
//rankingType int, list_rank []int, dataSource func(string, int, int) interface{}, event *resource_mgr.EventDetail)

// tag: database
func (r *RankServer) fetchDataBorder(timestamp_start, timestamp_end string, rankingType int, rank int) map[string]int {
	blist := r.fetchDataBorderV2(timestamp_start, timestamp_end, rankingType, rank)
	border := make(map[string]int)
	for _, v := range blist {
		border[v.string] = v.int
	}
	return border
}



func (r *RankServer) fetchDataBorderV2(timestamp_start, timestamp_end string, rankingType int, rank int) []struct{ string; int } {
	//border := map[string]int{}
	var blist []struct{ string; int }
	//timestamp_start := ts.TimeToTimestamp(event.EventStart())
	//timestamp_end := ts.TimeToTimestamp(event.ResultEnd())
	rows, err := r.db.Query("SELECT timestamp, score FROM rank WHERE type == $1 AND rank == $2 AND timestamp BETWEEN $3 AND $4;", rankingType+1, rank, timestamp_start, timestamp_end)
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var timestamp string
		var score int
		err = rows.Scan(&timestamp, &score)
		if err != nil {
			r.logger.Println("sql error", err)
			return nil
		}
		//border[timestamp] = score
		blist = append(blist, struct{ string; int }{timestamp, score})
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return blist
}

func (r *RankServer) fetchEventBorder(event *resource_mgr.EventDetail, rankingType int, rank int) [][2]string {
	//detail := r.resourceMgr.FindEventById(event)
	eventStart0 := ts.TruncateToDay(event.EventStart())
	eventStart := ts.TimeToTimestamp(event.EventStart())
	eventEnd := ts.TimeToTimestamp(event.EventEnd())
	eventBorder := r.fetchDataBorderV2(eventStart, eventEnd, rankingType, rank)
	// normalize
	//eventBorderNormalized := make(map[string]int)
	//var eventBorderNormalized []struct{string;int}
	var eventBorderNormalized [][2]string
	for _, kv := range eventBorder {
		k := kv.string
		v := kv.int
		//var k1 
		k1 := ts.TimestampToTime(k).Sub(eventStart0) / time.Second
		k2 := fmt.Sprintf("%d", k1)
		v2 := fmt.Sprintf("%d", v)
		//fmt.Println(k2, v)
		eventBorderNormalized = append(eventBorderNormalized, [2]string{k2,v2})
	}
	return eventBorderNormalized
}
