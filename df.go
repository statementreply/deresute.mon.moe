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
	"log"
	"os"
	"path"
	"time"
	ts "timestamp"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
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
	for index := 0; index < 5001; index++ {
		key_point = append(key_point, [2]int{1, index*10 + 1})
		key_point = append(key_point, [2]int{2, index*10 + 1})
	}
	df := datafetcher.NewDataFetcher(client, key_point, RANK_DB, RESOURCE_CACHE_DIR)
	//client.LoadCheck()
	err := df.Run()
	if err != nil {
		log.Println(err)
	}
}
