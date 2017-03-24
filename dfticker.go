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
	"apiclient"
	"datafetcher"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
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
var sleepDuration = time.Second * 150

func openLog() *os.File {
	fh, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		log.Fatalln("logfile", err)
	}
	log.SetOutput(fh)
	return fh
}

func main() {
	fh := openLog()

	ch_reload := make(chan os.Signal)
	signal.Notify(ch_reload, syscall.SIGHUP)
	go func() {
		for {
			select {
			case s := <-ch_reload:
				fh_new := openLog()
				log.Println("reopened logfile", s)
				if fh != nil {
					fh.Close()
				}
				fh = fh_new
			}
		}
	}()

	log.Println("local-timestamp", ts.GetLocalTimestamp())
	// A list of [2]int{rankingType, rank}
	// rank must be 10k+1 (the first of a page)
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
	// FIXME: get highscores at 15k+4 min
	// workaround: schedule them after the 50th keypoint

	// TODO: collect more around borders, only for result[start, end]?
	// (1, 2001), (1, 10001), (1, 20001), (1, 60001), (1, 100001), (1, 120001)
	// (1, 501), (1, 5001), (1, 50001)
	// (2, 5001), (2, 10001), (2, 40001), (2, 50001)
	// +- 1000?: 200pages
	// +- 100: all, +- 2000: by 100: 30pages
	// extra data points
	// from 1 to 100, all
	key_point = appendKeyPoint(key_point, 0, 10, 1, 10);
	key_point = appendKeyPoint(key_point, 0, 10, 2, 10);
	// from 101 to 1000, increase by 100
	key_point = appendKeyPoint(key_point, 1, 10, 1, 100);
	key_point = appendKeyPoint(key_point, 1, 10, 2, 100);
	// from 1k to 10k, by 1k
	key_point = appendKeyPoint(key_point, 1, 10, 1, 1000);
	key_point = appendKeyPoint(key_point, 1, 10, 2, 1000);
	// from 10k to 100k, by 10k
	key_point = appendKeyPoint(key_point, 1, 10, 1, 10000);
	key_point = appendKeyPoint(key_point, 1, 10, 2, 10000);
	// from 1 to 300k+1, by 10k
	key_point = appendKeyPoint(key_point, 0, 31, 1, 10000);
	key_point = appendKeyPoint(key_point, 0, 31, 2, 10000);
	// from 300k+1 to 800k+1, by 20k
	key_point = appendKeyPoint(key_point, 16, 41, 1, 20000);
	key_point = appendKeyPoint(key_point, 16, 41, 2, 20000);
	key_point = appendNeighborhood(key_point, 1, 2001)
	//key_point = appendKeyPoint(key_point, 1, 26, 1, 20000);
	fmt.Println(key_point);
	//return;
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
				q0 = q
				go runCommand(df, t)
			}
		}
	}
}

func appendKeyPoint(key_point [][2]int, start, count, typ, step int) [][2]int {
	for index := start; index < count; index++ {
		key_point = append(key_point, [2]int{typ, index*step + 1})
	}
	return key_point
}

func runCommand(df *datafetcher.DataFetcher, t time.Time) {
	if !IsRunning() {
		SetRunning()
		fmt.Println("runCommand", t.String())
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

// Add datapoints
// +- 100: all, 20pages
// #+- 2000: by 100: 40pages
// +- 2000 by 50: 80pages
// #+- 10000 by 200: 100 pages
func appendNeighborhood(oldKeyPoint [][2]int, rankingType int, basepoint int) [][2]int {
	var newKeyPoint = oldKeyPoint
	// firstly check parameter sanity
	if rankingType != 1 && rankingType != 2 {
		log.Fatalln("bad rankingType", rankingType)
	}
	if basepoint <= 0 || basepoint%10 != 1 {
		log.Fatalln("bad basepoint", basepoint)
	}

	// loop
	for i := -100; i <= 100; i += 10 {
		if basepoint+i > 0 {
			newKeyPoint = append(newKeyPoint, [2]int{rankingType, basepoint + i})
		}
	}
	for i := -2000; i <= 2000; i += 50 {
		if basepoint+i > 0 {
			newKeyPoint = append(newKeyPoint, [2]int{rankingType, basepoint + i})
		}
	}
	return newKeyPoint
}
