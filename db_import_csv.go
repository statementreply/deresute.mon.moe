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
	sqlite3 "github.com/mattn/go-sqlite3"
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
		queryCompare(db, timestamp, rankingType, rank, score);
	}
	return
}

func openDb() *sql.DB {
	// change to mode=rwc when ready to write
	db, err := sql.Open("sqlite3", "file:"+RANK_DB+"?mode=ro")
	if err != nil {
		log.Fatalln("cannot open db", err)
	}
	return db
}

func queryCompare(db *sql.DB, timestamp string, rankingType int, rank int, newScore int) {
	var score int
	// LIMIT 1: not necessary because of primary key
	row := db.QueryRow("SELECT score FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3 LIMIT 1;", timestamp, rankingType, rank);
	err := row.Scan(&score);
	if err != nil {
		if err == sql.ErrNoRows {
			// no old data for this data point
			fmt.Println("ADD", "score", "null", "newScore", newScore)
			//insertNewScore(db, timestamp, rankingType, rank, newScore)
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
		fmt.Println(status, "score", score, "newScore", newScore);
		if score != newScore {
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
	// else: good
}

func oldMain(db *sql.DB) {
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


	/*for _, fi := range fiList {
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
	}*/
	tx.Commit()
}

func parseDir(tx *sql.Tx, ts string) {
	var rankingType int
	var score int
	var rank int
	var viewer_id int

	_, err := tx.Exec("INSERT OR IGNORE INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5);",
	ts, rankingType, rank, score, viewer_id)
	if err != nil {
		log.Println("db insert err", err)
	}
}
