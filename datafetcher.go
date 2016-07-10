package main

import (
	"fmt"
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
var RANK_CACHE_DIR string = BASE + "/data/rank/"

func main() {
	rand.Seed(time.Now().Unix())
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)

	client.LoadCheck()

	friend_id := 679923520
	if len(os.Args) > 1 {
		friend_id, _ = strconv.Atoi(os.Args[1])
	}
	data := client.Call("/profile/get_profile", map[string]interface{}{"friend_id": friend_id})
	yy, _ := yaml.Marshal(data)
	fmt.Println(string(yy))

	client.GetPage(1, 9, "r1.009")
}

// 9269 23784
