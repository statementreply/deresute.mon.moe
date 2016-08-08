package main

import (
	"log"
	"os"
	"path"
	"database/sql"
	sqlite3 "github.com/mattn/go-sqlite3"
)

var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
var RANK_DB string = BASE + "/data/rankbeta.db"

func main() {
	db, err := sql.Open("sqlite3", RANK_DB)
	if err != nil {
		log.Println("cannot open db", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS rank (timestamp TEXT, type INTEGER, rank INTEGER, score INTEGER, viewer_id INTEGER);")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS timestamp (timestamp TEXT UNIQUE);")
	if err != nil {
		log.Println("create table", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	/*
	_, err = db.Exec("INSERT OR IGNORE INTO timestamp (timestamp) VALUES ($1)", ts)
	if err != nil && err != sqlite3.ErrConstraintUnique {
		log.Println("db insert err", err)
		log.Printf("%#v", err)
		log.Printf("%d %d", err.(sqlite3.Error).Code, err.(sqlite3.Error).ExtendedCode)
	}

	vmap := value
	rank := vmap["rank"]
	score := vmap["score"]
	viewer_id := vmap["user_info"].(map[interface{}]interface{})["viewer_id"]
	_, err := db.Exec("INSERT INTO rank (timestamp, type, rank, score, viewer_id) VALUES ($1, $2, $3, $4, $5)",
	server_timestamp,
	ranking_type,
	rank,
	score,
	viewer_id)
	if err != nil {
		log.Println("db insert err", err)
	}
	*/
}
