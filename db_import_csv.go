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

// read csv, compare with previous data, add if new

// arguments: timestamp filename.csv

// available files and timestamps
// 1474945320 "01_ラブレター.csv"
// 1477623720 "02_Jet_to_the_Future.csv"
// 1478660520 "03_あいくるしい.csv"
// 1480302120 "04_Flip_Flop.csv"
// 1482894120 "05_あんきら！？狂騒曲.csv"
// 1484017320 "06_命燃やして恋せよ乙女.csv"

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	//"io/ioutil"
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
var RANK_DB string = BASE + "/data/rank.db"
var tsFilter = regexp.MustCompile("^\\d+$")
var fnFilter = regexp.MustCompile("r\\d{2}\\.(\\d+)$")
var rankingTypeFilter = regexp.MustCompile("r01\\.\\d+$")

var insertCount = 0
var badCount = 0

func main() {
	var timestamp, csvFilename string
	// only type 1 data (event pt data)
	var rankingType = 1

	// parse cmdline
	if len(os.Args) == 3 {
		timestamp = os.Args[1]
		csvFilename = os.Args[2]
	} else {
		log.Fatalln("usage: " + os.Args[0] + " timestamp filename.csv")
	}
	log.Println(timestamp)

	// open/close db
	db := openDb()
	defer db.Close()

	// open csv, read all rows
	csvFile, err := os.Open(csvFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer csvFile.Close()
	csvReader := csv.NewReader(csvFile)
	csvRecords, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	// parse rows and compare with sql data
	for _, record := range csvRecords {
		var rank int
		var score int
		if len(record) != 2 {
			log.Fatalln("#fields on line is not 2")
		}
		rank, err := strconv.Atoi(record[0])
		if err != nil {
			log.Fatal(err)
		}
		score, err = strconv.Atoi(record[1])
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Println(record)
		fmt.Println("timestamp=" + timestamp + "; type=" + strconv.Itoa(rankingType) + "; rank=" + strconv.Itoa(rank) + "; score=" + strconv.Itoa(score) + "; viewer_id=0")
		queryCompare(db, timestamp, rankingType, rank, score)
	}
	log.Println("insertCount:", insertCount)
	log.Println("badCount:", badCount)
	return
}

func openDb() *sql.DB {
	// change to mode=rw when ready to write
	db, err := sql.Open("sqlite3", "file:"+RANK_DB+"?mode=rw")
	if err != nil {
		log.Fatalln("cannot open db", err)
	}
	return db
}

func queryCompare(db *sql.DB, timestamp string, rankingType int, rank int, newScore int) {
	var score int
	// LIMIT 1: not necessary because of primary key
	row := db.QueryRow("SELECT score FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3 LIMIT 1;", timestamp, rankingType, rank)
	err := row.Scan(&score)
	if err != nil {
		if err == sql.ErrNoRows {
			// no old data for this data point
			fmt.Println("ADD", "score", "null", "newScore", newScore)
			insertNewScore(db, timestamp, rankingType, rank, newScore)
		} else {
			log.Fatal(err)
		}
	} else {
		// compare old data with the new one
		status := "bad"
		if score == newScore {
			status = "good"
		} else {
			status = "bad"
		}
		fmt.Println(status, "score", score, "newScore", newScore)
		if score != newScore {
			badCount += 1
			//log.Fatal("data different")
			// only very few, ignore them for now
		}
	}
}

func insertNewScore(db *sql.DB, timestamp string, rankingType int, rank int, newScore int) {
	result, err := db.Exec("INSERT OR ROLLBACK INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);", timestamp, rankingType, rank, newScore, 0)
	if err != nil {
		log.Fatal("insertion failed", err)
	}
	rs, err := result.RowsAffected()
	if err != nil {
		log.Fatal("result.RowsAffected failed", err)
	}
	if rs != 1 {
		log.Fatal("affected rows", rs)
	}
	insertCount += 1
	// else: good
}
