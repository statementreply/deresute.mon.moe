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

// todo: retry in the same run, avoid having to restart the program

import (
	"apiclient"
	"datafetcher"
	"log"
	"os"
	"path"
	"resource_mgr"
	"time"
	ts "timestamp"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
// separate from the main db
var RANK_DB string = BASE + "/data/extra.db"
var RESOURCE_CACHE_DIR string = BASE + "/data/resourcesbeta/"

func main() {
	log.Println("dfnew", os.Args[0])
	//rand.Seed(time.Now().Unix())
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)

	log.Println(ts.GetLocalTimestamp())
	log.Println(ts.RoundTimestamp(time.Now()).String())

	key_point := [][2]int{
		[2]int{1, 1},
		[2]int{1, 501},     // pt ranking emblem-1
		[2]int{1, 2001},    // tier 1
		[2]int{1, 5001},    // emblem-2
		[2]int{1, 10001},   // tier 2
		[2]int{1, 20001},   // tier 3
		[2]int{1, 50001},   // tier 4-old
		[2]int{1, 60001},   // tier 4
		[2]int{1, 100001},  // tier 5-old
		[2]int{1, 120001},  // tier 5
		[2]int{1, 300001},  // tier 6
		[2]int{1, 500001},  // tier 7
		[2]int{1, 1000001}, // tier 8
		[2]int{2, 1},       // score ranking top
		[2]int{2, 5001},    // tier 1
		[2]int{2, 10001},   // tier 2
		[2]int{2, 40001},   // tier 3
		[2]int{2, 50001},   // tier 4
	}
	// extra data points
	for index := 0; index < 61; index++ {
		key_point = append(key_point, [2]int{1, index*10000 + 1})
		key_point = append(key_point, [2]int{2, index*10000 + 1})
	}
	// 1 to 25000
	for index := 0; index < 2501; index++ {
		key_point = append(key_point, [2]int{1, index*10 + 1})
		key_point = append(key_point, [2]int{2, index*10 + 1})
	}
	// 50000-70000
	for index := 5000; index < 7001; index++ {
		key_point = append(key_point, [2]int{1, index*10 + 1})
		key_point = append(key_point, [2]int{2, index*10 + 1})
	}
	// 110000-130000
	for index := 11000; index < 13001; index++ {
		key_point = append(key_point, [2]int{1, index*10 + 1})
		key_point = append(key_point, [2]int{2, index*10 + 1})
	}
	// 1-130000
	for index := 1; index < 1301; index++ {
		key_point = append(key_point, [2]int{1, index*100 + 1})
		key_point = append(key_point, [2]int{2, index*100 + 1})
	}
	df := datafetcher.NewDataFetcher(client, key_point, RANK_DB, RESOURCE_CACHE_DIR)

	// bypass some error handling in datafetch.Run()
	//client.LoadCheck()
	//err := df.Run()

	df.Client.LoadCheck()
	rv := df.Client.Get_res_ver()
	resourceMgr := resource_mgr.NewResourceMgr(rv, RESOURCE_CACHE_DIR)
	resourceMgr.ParseEvent()
	currentEvent := resourceMgr.FindCurrentEvent()
	log.Println(currentEvent)
	//local_timestamp := ts.GetLocalTimestamp()
	if len(os.Args) < 2 {
		log.Fatal("need timestamp as cmdline param")
	}
	local_timestamp := os.Args[1]

	df.OpenDb();
	defer df.CloseDb();

	// local reverse
	//for _, key := range key_point {
	discard := 0
	log.Println(key_point)
	for	i := len(key_point) - 1; i >= 0; i-- {
		key := key_point[i]
		if key[0] == 1 {
			//log.Println("skipping type == 1", key[1])
			continue
		}
		discard += 1
		//if discard % 5 != 0 {
		if key[1] % 100 != 1 {
			log.Println("[INFO] skipping", key[1])
			continue
		}
		_, statusStr, err := df.GetCache(currentEvent, key[0],
				datafetcher.RankToPage(key[1]), local_timestamp)
		if (statusStr == "*") {
			discard += 0
		} else if (statusStr == "-") {
			discard += 0
		}
		_ = statusStr
		if err != nil {
			log.Println(err)
			if err == apiclient.ErrSession {
				time.Sleep(2200 * time.Millisecond)
				df.Client.Reset_sid()
				df.Client.LoadCheck()
			}
			// TODO: append to retry
		}
	}
}
