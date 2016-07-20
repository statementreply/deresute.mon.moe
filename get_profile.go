package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strconv"
	"time"
	//_ "crypto/aes"
	//"crypto/cipher"
	//"rijndael_wrapper"
	"apiclient"
	"gopkg.in/yaml.v2"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rankbeta/"

func main() {
	rand.Seed(time.Now().Unix())
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)
	client.LoadCheck()

	friend_id := 679923520
	if len(os.Args) > 1 {
		friend_id, _ = strconv.Atoi(os.Args[1])
	}
	data := client.GetProfile(friend_id)
	yy, _ := yaml.Marshal(data)
	fmt.Println(string(yy))
	//DumpToFile(data, "user3520")

	//p1 := client.GetPage(1, 9)
	//DumpToFile(p1, "r1.009.20")

	// m@gic 162 165=master
	//d2 := client.GetLiveDetailRanking(165, 2)
	//DumpToStdout(d2)
	//DumpToStdout(client.GetLiveDetailRanking(165, 10))
}

func DumpToStdout(v interface{}) {
	yy, _ := yaml.Marshal(v)
	fmt.Println(string(yy))
}

func DumpToFile(v interface{}, fileName string) {
	yy, _ := yaml.Marshal(v)
	ioutil.WriteFile(fileName, yy, 0644)
}
