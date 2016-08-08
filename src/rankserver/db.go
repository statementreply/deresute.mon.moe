package rankserver

import (
	"sort"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"time"
	ts "timestamp"
)

// tag: database, sqlite
func (r *RankServer) UpdateTimestamp() {
	rows, err := r.db.Query("SELECT timestamp FROM timestamp")
	if err != nil {
		r.logger.Println("sql error", err)
		return
	}
	defer rows.Close()
	var local_list_timestamp []string
	for rows.Next() {
		var ts string
		err = rows.Scan(&ts)
		if err != nil {
			r.logger.Println("sql error", err)
			return
		}
		local_list_timestamp = append(local_list_timestamp, ts)
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
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
	row := r.db.QueryRow("SELECT timestamp FROM rank WHERE timestamp == $1", timestamp)
	err := row.Scan(&ts_discard)
	// FIXME err
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		} else {
			r.logger.Println("sql error", err)
			return false
		}
	} else {
		// row exists
		return true
	}
}


// tag: database, sqlite
func (r *RankServer) CheckData(timestamp string) {
	r.UpdateTimestamp()
	latest := r.latestTimestamp()
	latest_time := time.Unix(0, 0)
	if latest != "" {
		latest_time = ts.TimestampToTime(latest)
	}
	// check new res_ver
	// FIXME need some test
	if (time.Now().Sub(r.lastCheck) >= 5*time.Hour/2) || ((r.currentEvent == nil) && (time.Now().Sub(latest_time) <= 2*time.Hour)) {
		r.logger.Println("recheck res_ver, lastcheck:", r.lastCheck, "latest_time:", latest_time)
		r.client.LoadCheck()
		rv := r.client.Get_res_ver()
		r.resourceMgr.Set_res_ver(rv)
		r.resourceMgr.ParseEvent()
		r.currentEvent = r.resourceMgr.FindCurrentEvent()
		r.lastCheck = time.Now()
	}

	if timestamp == "" {
		timestamp = latest
	}
}


// tag: database
func (r *RankServer) fetchData(timestamp string, rankingType int, rank int) int {
	var score int
	row := r.db.QueryRow("SELECT score FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3", timestamp, rankingType+1, rank)
	err := row.Scan(&score)
	if err != nil {
		if err == sql.ErrNoRows {
			score = -1
		} else {
			if err != nil {
				r.logger.Println("sql error", err)
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
	rows, err := r.db.Query("SELECT rank FROM rank WHERE timestamp == $1 AND type == $2", timestamp, rankingType+1)
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var rank int
		err = rows.Scan(&rank)
		if err != nil {
			r.logger.Println("sql error", err)
			return nil
		}
		if rank%10 == 1 {
			listRank = append(listRank, rank)
		}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return listRank
}



// tag: database
func (r *RankServer) fetchDataSlice(timestamp string) []map[int]int {
	slice := make([]map[int]int, 2)
	slice[0] = map[int]int{}
	slice[1] = map[int]int{}

	rows, err := r.db.Query("SELECT type, rank, score FROM rank WHERE timestamp == $1", timestamp)
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
		if rank%10 == 1 {
			slice[rankingType][rank] = score
		}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return slice
}
