package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	//"strconv"
	"time"
	"apiclient"
	"gopkg.in/yaml.v2"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rankbeta/"

func main() {
	rand.Seed(time.Now().Unix())
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)
	//fmt.Println(client)
	client.LoadCheck()

	//p1 := client.GetAtaponRanking(1, 9)
	//DumpToFile(p1, "r1.009.20")
	//DumpToStdout(p1)
	fmt.Println(apiclient.GetLocalTimestamp())
	fmt.Println(apiclient.RoundTimestamp(time.Now()).String())

	key_point := [][2]int{
		[2]int{1,1},
		[2]int{1,501},  // pt ranking emblem-1
		[2]int{1,2001}, // tier 1
		[2]int{1,5001}, // emblem-2
		[2]int{1,10001},
		[2]int{1,20001},
		[2]int{1,50001},
		[2]int{1,60001},
		[2]int{1,100001},
		[2]int{1,120001},
		[2]int{1,300001},
		[2]int{1,500001},
		[2]int{1,1000001},
		[2]int{2,1},
		[2]int{2,5001},
		[2]int{2,10001},
		[2]int{2,40001},
		[2]int{2,50001},
	}
	for index := 0; index < 61; index++ {
		key_point = append(key_point, [2]int{1, index*10000+1})
		key_point = append(key_point, [2]int{2, index*10000+1})
	}
	//GetCache(client, RANK_CACHE_DIR, 0, 51)
	for _, key := range key_point {
		fmt.Println(key)
		GetCache(client, RANK_CACHE_DIR, key[0], RankToPage(key[1]))
	}
}

func RankToPage(rank int) int {
	var page int
	page = ( (rank - 1) / 10) + 1
	return page
}

func DumpToStdout(v interface{}) {
	yy, _ := yaml.Marshal(v)
	fmt.Println(string(yy))
}

func DumpToFile(v interface{}, fileName string) {
	yy, _ := yaml.Marshal(v)
	ioutil.WriteFile(fileName, yy, 0644)
}

func GetCache(client *apiclient.ApiClient, cache_dir string, ranking_type int, page int) {
	local_timestamp := apiclient.GetLocalTimestamp()
	dirname := cache_dir + local_timestamp + "/"
	path := dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
	if Exists(path) {
		// cache hit
		return
	} else {
		// cache miss
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	time.Sleep(11 * 100 * 1000 * 1000)
	ranking_list, servertime := get_page(client, ranking_type, page)
	server_timestamp_i := apiclient.RoundTimestamp(time.Unix(int64(servertime), 0)).Unix()
	server_timestamp := fmt.Sprintf("%d", server_timestamp_i)

	if server_timestamp != local_timestamp {
		fmt.Println("server:", server_timestamp, "local:", local_timestamp)
		dirname = cache_dir + server_timestamp + "/"
		path = dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	fmt.Println("write to path", path)
	DumpToFile(ranking_list, path)
	//DumpToStdout(ranking_list)
	//fmt.Println(ranking_list)
}

func get_page(client *apiclient.ApiClient, ranking_type, page int) ([]interface{}, uint64) {
	resp := client.GetAtaponRanking(ranking_type, page)
	servertime := resp["data_headers"].(map[interface{}]interface{})["servertime"].(uint64)
	ranking_list := resp["data"].(map[interface{}]interface{})["ranking_list"].([]interface{})
	return ranking_list, servertime
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
