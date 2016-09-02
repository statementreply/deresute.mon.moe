package main

import (
	"apiclient"
	"datafetcher"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"
	ts "timestamp"
)

// parameters
var SECRET_FILE string = "secret.yaml"
var LOG_FILE string = "dfticker.log"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
var RANK_DB string = BASE + "/data/rank.db"
var RESOURCE_CACHE_DIR string = BASE + "/data/resourcesbeta/"

// global vars
var lock sync.Mutex
var _isRunning bool
var lastRun = time.Unix(0, 0)

// global const
var sleepDuration = time.Second * 120

func main() {
	fh, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("logfile", err)
	}
	log.SetOutput(fh)

	log.Println("local-timestamp", ts.GetLocalTimestamp())
	key_point := [][2]int{
		[2]int{1, 2001},    // tier 1
		[2]int{1, 10001},   // tier 2
		[2]int{1, 20001},   // tier 3
		[2]int{1, 60001},   // tier 4
		[2]int{1, 120001},  // tier 5
		[2]int{1, 1},       // top
		[2]int{1, 501},     // emblem-1
		[2]int{1, 5001},    // emblem-2
		[2]int{1, 50001},   // emblem-3 tier 4-old
		[2]int{1, 100001},  // tier 5-old
		[2]int{1, 300001},  // tier 6
		[2]int{1, 500001},  // tier 7 emblem-4
		[2]int{1, 1000001}, // tier 8
		[2]int{2, 1},       // top
		[2]int{2, 5001},    // tier 1
		[2]int{2, 10001},   // tier 2
		[2]int{2, 40001},   // tier 3 atapon
		[2]int{2, 50001},   // tier 3 medley
	}
	// extra data points
	for index := 0; index < 61; index++ {
		key_point = append(key_point, [2]int{1, index*10000 + 1})
		key_point = append(key_point, [2]int{2, index*10000 + 1})
	}
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)
	df := datafetcher.NewDataFetcher(client, key_point, RANK_DB, RESOURCE_CACHE_DIR)

	ticker := time.NewTicker(time.Second * 1)
	var q, q0 time.Duration
	var r, mod time.Duration
	r = time.Minute * 2
	mod = time.Minute * 15
	q0 = (time.Duration(time.Now().UnixNano()) - r) / mod

	for {
		select {
		case t := <-ticker.C:
			//log.Println(t.String(), _isRunning, lastRun.String())
			q = (time.Duration(t.UnixNano()) - r) / mod
			if (q > q0) || NeedToRun() {
				fmt.Println("runCommand", t.String())
				q0 = q
				go runCommand(df)
			}
		}
	}
}

func runCommand(df *datafetcher.DataFetcher) {
	if !IsRunning() {
		SetRunning()
		err := df.Run()
		SetFinished()
		if err != nil {
			log.Println("dfticker runCommand error: ", err)
			log.Printf("dfticker runCommand error: %#v\n", err)
			// reset unconditionally
			if err != nil {
				df.Client.Reset_sid()
			}
			// err timeout?
			if err == apiclient.ErrSession || err == datafetcher.ErrRerun ||
			   err == datafetcher.ErrNoResponse {
				// run again immediately
				lock.Lock()
				lastRun = time.Unix(0, 0)
				lock.Unlock()
			}
		}

		log.Println("[INFO] current:", time.Now().String())
		lock.Lock()
		log.Println("[INFO] lastRun:", lastRun.String())
		lock.Unlock()
	}
}

func IsRunning() bool {
	lock.Lock()
	ret := _isRunning
	lock.Unlock()
	return ret
}

func SetRunning() {
	lock.Lock()
	_isRunning = true
	lastRun = time.Now()
	lock.Unlock()
}

func SetFinished() {
	lock.Lock()
	_isRunning = false
	lock.Unlock()
}

func NeedToRun() bool {
	lock.Lock()
	diff := time.Now().Sub(lastRun)
	lock.Unlock()
	ret := false
	if diff > sleepDuration {
		ret = true
	}
	return ret
}
