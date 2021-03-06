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

package datafetcher

import (
	"apiclient"
	"database/sql"
	"errors"
	"fmt"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"resource_mgr"
	"time"
	ts "timestamp"
)

var ErrNoEvent = errors.New("no event is running now")
var ErrEventType = errors.New("current event type has no ranking")
var ErrRankingNA = errors.New("current time is not in event/result period")
var ErrNoResponse = errors.New("no response received")
var ErrRerun = errors.New("new server timestamp")

type DataFetcher struct {
	Client      *apiclient.ApiClient
	resourceMgr *resource_mgr.ResourceMgr
	key_point   [][2]int
	rankDB      string
	db          *sql.DB
	// prevent duplicate during [resultstart, resultend]
	currentResultEnd time.Time
}

func NewDataFetcher(client *apiclient.ApiClient, key_point [][2]int, rank_db, resource_cache_dir string) *DataFetcher {
	//log.Println("NewDataFetcher()")
	df := new(DataFetcher)

	df.Client = client
	//client.LoadCheck()
	df.key_point = key_point
	df.rankDB = rank_db

	df.Client.LoadCheck()
	rv := client.Get_res_ver()
	df.resourceMgr = resource_mgr.NewResourceMgr(rv, resource_cache_dir)

	df.currentResultEnd = time.Unix(0, 0)

	//log.Println(GetLocalTimestamp())
	//log.Println(RoundTimestamp(time.Now()).String())
	return df
}

// FIXME
func (df *DataFetcher) FinalResultDuplicate(currentEvent *resource_mgr.EventDetail) bool {
	// condition
	// now is in [result start, result end]
	// latest ts is in [,]
	// latest ts has all key point

	if !currentEvent.IsFinal(time.Now()) {
		return false
	}

	var latest_timestamp string
	row := df.db.QueryRow("SELECT timestamp FROM timestamp ORDER BY timestamp DESC LIMIT 1;")
	err := row.Scan(&latest_timestamp)

	if err != nil {
		if err == sql.ErrNoRows {
			return false
		} else {
			log.Println("sql error", err)
			return false
		}
	}

	latest_time := ts.TimestampToTime(latest_timestamp)
	if !currentEvent.IsFinal(latest_time) {
		return false
	}

	rows, err := df.db.Query("SELECT type, rank FROM rank WHERE timestamp == $1;", latest_timestamp)
	if err != nil {
		log.Println("sql error", err)
		return false
	}
	defer rows.Close()
	local_key_point := make([]map[int]bool, 2)
	local_key_point[0] = map[int]bool{}
	local_key_point[1] = map[int]bool{}
	for rows.Next() {
		var rankingType, rank int
		err = rows.Scan(&rankingType, &rank)
		if err != nil {
			log.Println("sql error", err)
			return false
		}
		rankingType -= 1
		local_key_point[rankingType][rank] = true
	}
	err = rows.Err()
	if err != nil {
		log.Println("sql error", err)
		return false
	}
	for _, k := range df.key_point {
		rankingType := k[0]
		rank := k[1]
		_, ok := local_key_point[rankingType-1][rank]
		if !ok {
			return false
		}
		log.Println("[INFO] key_point available", rankingType, rank)
	}
	return true
}

// tag: database, sqlite
func (df *DataFetcher) setCacheSize() {
	_, err := df.db.Exec("PRAGMA cache_size = -6000;")
	if err != nil {
		log.Println("set cache_size", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}
}

func (df *DataFetcher) Run() error {
	// handle new res_ver
	df.Client.LoadCheck()
	rv := df.Client.Get_res_ver()
	df.resourceMgr.Set_res_ver(rv)

	df.resourceMgr.ParseEvent()
	currentEvent := df.resourceMgr.FindCurrentEvent()

	if currentEvent == nil {
		return ErrNoEvent
	}

	local_timestamp := ts.GetLocalTimestamp()
	local_time := ts.TimestampToTime(local_timestamp)
	if local_time.Before(df.currentResultEnd) {
		log.Println("[NOTICE] duplicate final result prevented")
		return nil
	}

	db, err := sql.Open("sqlite3", "file:"+df.rankDB+"?mode=rwc")
	if err != nil {
		log.Println("cannot open db", err)
	}
	defer db.Close()
	df.db = db
	//df.setCacheSize()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS rank (timestamp TEXT, type INTEGER, rank INTEGER, score INTEGER, viewer_id INTEGER, PRIMARY KEY(timestamp, type, rank));")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS timestamp (timestamp TEXT, PRIMARY KEY(timestamp));")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	if df.FinalResultDuplicate(currentEvent) {
		log.Println("[NOTICE] duplicate final result prevented (sql)")
		return nil
	}

	// looping will be slow because of Sleep(): to reduce load on the server
	for _, key := range df.key_point {
		//log.Println("rankingtype:", key[0], "rank:", key[1])
		timestamp, statusStr, err := df.GetCache(currentEvent, key[0], RankToPage(key[1]), local_timestamp)
		if err != nil {
			//log.Fatal(err)
			return err
		}
		// FIXME: don't return ErrRerun for the result period
		if timestamp != local_timestamp && currentEvent.IsActive(local_time) {
			return ErrRerun
		}
		fmt.Print(statusStr) // progress bar
	}
	// if every datapoint is ok, mark final
	// FIXME: what will happen if we want to add new datapoints during result period
	if currentEvent.IsFinal(local_time) {
		df.currentResultEnd = currentEvent.ResultEnd()
	} else {
		df.currentResultEnd = time.Unix(0, 0)
	}

	fmt.Print("\n")
	return nil
}

func (df *DataFetcher) OpenDb() {
	db, err := sql.Open("sqlite3", "file:"+df.rankDB+"?mode=rwc")
	if err != nil {
		log.Println("cannot open db", err)
	}
	df.db = db
}

func (df *DataFetcher) CloseDb() {
	df.db.Close()
}


func RankToPage(rank int) int {
	var page int
	page = ((rank - 1) / 10) + 1
	return page
}

func DumpToStdout(v interface{}) {
	yy, err := yaml.Marshal(v)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(string(yy))
}

func DumpToFile(v interface{}, fileName string) {
	yy, err := yaml.Marshal(v)
	if err != nil {
		log.Println(err)
		return
	}
	ioutil.WriteFile(fileName, yy, 0644)
}

//return timestamp, statuscode("-", "*", ""), err
func (df *DataFetcher) GetCache(currentEvent *resource_mgr.EventDetail, ranking_type int, page int, local_timestamp_in string) (string, string, error) {

	event_type := currentEvent.Type()
	//log.Println("current event type:", event_type)
	if !currentEvent.HasRanking() {
		return "", "", ErrEventType
	}
	if !currentEvent.RankingAvailable() {
		return "", "", ErrRankingNA
	}

	//localtime := float64(time.Now().UnixNano()) / 1e9 // for debug

	// The timestamp that will be used to commit to db,
	// and query for hit/miss
	// When event is active, get a new commit_timestamp on each GetCache()
	// When event is final/result, use the local_timestamp_in param, so that
	// each Run() will have the same timestamp for each GetCache() call.
	commit_timestamp := local_timestamp_in
	if currentEvent.IsActive(ts.TimestampToTime(commit_timestamp)) {
		commit_timestamp = ts.GetLocalTimestamp()
	}

	hit := true

	// NOTE: decide hit/miss, sqlite3 version
	// query timestamp local_timestamp
	// query rank local_timestamp, ranking_type, page-to-rank
	var ts_discard string
	row := df.db.QueryRow("SELECT timestamp FROM timestamp WHERE timestamp == $1 LIMIT 1;", commit_timestamp)
	err := row.Scan(&ts_discard)
	if err != nil {
		if err == sql.ErrNoRows {
			// sql miss
			//log.Println("not exist", local_timestamp, err)
			hit = false
		} else {
			log.Println("sql error", err)
			return "", "", err
		}
	} else {
		// sql hit
		//log.Println("[INFO] hit table timestamp", local_timestamp)
	}
	row = df.db.QueryRow("SELECT timestamp FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3 LIMIT 1;", commit_timestamp, ranking_type, (page-1)*10+1)
	err = row.Scan(&ts_discard)
	if err != nil {
		if err == sql.ErrNoRows {
			// sql miss
			//log.Println("not exist", local_timestamp, err)
			hit = false
		} else {
			log.Println("sql error", err)
			return "", "", err
		}
	} else {
		// sql cache hit, no need to download again
		//log.Println("[INFO] hit table rank", local_timestamp)
	}

	if hit {
		log.Println("[INFO] hit", commit_timestamp, ranking_type, page)
		return commit_timestamp, "-", nil
	}

	// FIXME: wait between requests
	time.Sleep(2400 * time.Millisecond)
	ranking_list, servertime, err := df.GetPage(event_type, ranking_type, page)
	if err != nil {
		return "", "", err
	}
	//log.Printf("localtime: %f servertime: %d lag: %f\n", localtime, servertime, float64(servertime)-localtime)

	{ // limit scope for server_timestamp*
	server_timestamp_i := ts.RoundTimestamp(time.Unix(int64(servertime), 0)).Unix()
	server_timestamp := fmt.Sprintf("%d", server_timestamp_i)

	// FIXME: Limit this check only to event active period
	// and disable for event result period
	if server_timestamp != commit_timestamp {
		if currentEvent.IsActive(ts.TimestampToTime(commit_timestamp)) {
			log.Println("[NOTICE] server_timestamp different from commit_timestamp:", server_timestamp, "local:", commit_timestamp)
			commit_timestamp = server_timestamp
		}
	}
	} // scope ends for server_timestamp*

	// write to df.db
	for _, value := range ranking_list {
		vmap := value.(map[interface{}]interface{})
		// FIXME what interface? uint64?
		rank := vmap["rank"]
		score := vmap["score"]
		viewer_id := vmap["user_info"].(map[interface{}]interface{})["viewer_id"]
		_, err := df.db.Exec("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);",
			commit_timestamp,
			ranking_type,
			rank,
			score,
			viewer_id)
		if err != nil {
			log.Println("db insert err", err)
		}
	}
	// fill zeros
	for rank := (page-1)*10 + 1 + len(ranking_list); rank <= (page-1)*10+10; rank++ {
		// rank, 0, 0
		_, err := df.db.Exec("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);",
			commit_timestamp, ranking_type, rank, 0, 0)
		if err != nil {
			log.Println("db insert err", err)
		}
	}
	_, err = df.db.Exec("INSERT OR IGNORE INTO timestamp (timestamp) VALUES ($1);", commit_timestamp)
	if err != nil && err != sqlite3.ErrConstraintUnique {
		log.Println("db insert err", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}
	return commit_timestamp, "*", nil
}

func (df *DataFetcher) GetPage(event_type, ranking_type, page int) ([]interface{}, uint64, error) {
	var ranking_list []interface{}
	if !df.Client.IsInitialized() {
		df.Client.LoadCheck()
	}
	// deal with atapon/medley
	var resp map[string]interface{}
	if event_type == 1 {
		resp = df.Client.GetAtaponRanking(ranking_type, page)
	} else if event_type == 3 {
		resp = df.Client.GetMedleyRanking(ranking_type, page)
	} else if event_type == 5 {
		resp = df.Client.GetTourRanking(ranking_type, page)
	} else {
		return nil, 0, ErrEventType
	}
	if resp == nil {
		return nil, 0, ErrNoResponse
	}

	servertime := resp["data_headers"].(map[interface{}]interface{})["servertime"].(uint64)
	err := df.Client.ParseResultCode(resp)
	if err != nil {
		return nil, servertime, err
	}
	log.Println("[INFO] get", servertime, ranking_type, page)
	ranking_list = resp["data"].(map[interface{}]interface{})["ranking_list"].([]interface{})
	// save less data
	for _, value := range ranking_list {
		vmap := value.(map[interface{}]interface{})
		delete(vmap, "leader_card_info")
		viewer_id := vmap["user_info"].(map[interface{}]interface{})["viewer_id"]
		delete(vmap, "user_info")
		vmap["user_info"] = map[interface{}]interface{}{"viewer_id": viewer_id}
	}
	return ranking_list, servertime, err
}

func Exists(fileName string) bool {
	_, err := os.Stat(fileName)
	if err == nil {
		return true
	} else {
		if os.IsNotExist(err) {
			return false
		} else {
			return true
		}
	}
}
