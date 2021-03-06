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

package main

import (
	"database/sql"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
)

// use primary key
// table timestamp (timestamp) key (timestamp)
// table rank (timestamp, type, rank score id)  key (timestamp, type rank)

var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
var RANK_DB string = BASE + "/data/rankbeta.db"
var tsFilter = regexp.MustCompile("^\\d+$")
var fnFilter = regexp.MustCompile("r\\d{2}\\.(\\d+)$")
var rankingTypeFilter = regexp.MustCompile("r01\\.\\d+$")

func main() {
	db, err := sql.Open("sqlite3", "file:"+RANK_DB+"?mode=rwc")
	if err != nil {
		log.Println("cannot open db", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalln(err)
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS rank (timestamp TEXT, type INTEGER, rank INTEGER, score INTEGER, viewer_id INTEGER, PRIMARY KEY(timestamp, type, rank));")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS timestamp (timestamp TEXT, PRIMARY KEY('timestamp'));")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	fiList, err := ioutil.ReadDir(RANK_CACHE_DIR)
	if err != nil {
		log.Fatalln(err)
	}

	for _, fi := range fiList {
		if tsFilter.MatchString(fi.Name()) && fi.IsDir() {
			ts := fi.Name()
			log.Println(ts)
			_, err = tx.Exec("INSERT OR IGNORE INTO timestamp (timestamp) VALUES ($1);", ts)
			if err != nil {
				log.Println("db insert err", err)
				log.Printf("%#v", err)
				log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
			}
			parseDir(tx, ts)
		}
	}
	tx.Commit()
}

func parseDir(tx *sql.Tx, ts string) {

	dirPath := RANK_CACHE_DIR + ts
	fiList, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatalln(err)
	}

	for _, fi := range fiList {
		fileName := ""
		var rankingType int
		if fnFilter.MatchString(fi.Name()) {
			fileName = fi.Name()
		} else {
			continue
		}

		if rankingTypeFilter.MatchString(fileName) {
			rankingType = 1
		} else {
			rankingType = 2
		}

		// get page from fileName
		submatch := fnFilter.FindStringSubmatch(fileName)
		if len(submatch) < 1 {
			log.Fatalln(fileName, "incorrect")
		}
		page, err := strconv.Atoi(submatch[1])
		if err != nil {
			log.Fatalln(err)
		}

		filePath := dirPath + "/" + fileName
		log.Println(filePath)

		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalln(err)
		}

		var local_rank_list []map[string]interface{}
		err = yaml.Unmarshal(content, &local_rank_list)
		if err != nil {
			log.Fatalln("YAML error", err)
		}

		for _, user := range local_rank_list {
			var score int
			var rank int
			var viewer_id int
			score = user["score"].(int)
			rank = user["rank"].(int)
			viewer_id, ok := user["user_info"].(map[interface{}]interface{})["viewer_id"].(int)
			if !ok {
				// try string
				viewer_id_str := user["user_info"].(map[interface{}]interface{})["viewer_id"].(string)
				viewer_id, err = strconv.Atoi(viewer_id_str)
				if err != nil {
					log.Fatalln(err)
				}
			}
			_, err := tx.Exec("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);",
				ts, rankingType, rank, score, viewer_id)
			if err != nil {
				log.Println("db insert err", err)
			}
		}

		// fill zeros
		for rank := (page-1)*10 + 1 + len(local_rank_list); rank <= (page-1)*10+10; rank++ {
			// rank, 0, 0
			_, err := tx.Exec("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);",
				ts, rankingType, rank, 0, 0)
			if err != nil {
				log.Println("db insert err", err)
			}
		}
	}
}
