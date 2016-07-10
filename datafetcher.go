package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path"
	"strconv"
	"time"
	//_ "crypto/aes"
	"crypto/md5"
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
	sum_tmp := md5.Sum([]byte("All your APIs are belong to us"))
	args := map[string]interface{}{"campaign_data": "",
		"campaign_user": 171780,
		"campaign_sign": hex.EncodeToString(sum_tmp[:]),
		"app_type":      0}

	check := client.Call("/load/check", args)
	log.Print(check)
	new_res_ver, ok := check["data_headers"].(map[interface{}]interface{})["required_res_ver"]
	if ok {
		s := new_res_ver.(string)
		client.Set_res_ver(s)
		fmt.Println("Update res_ver to ", s)
		time.Sleep(1.3e9)
		check := client.Call("/load/check", args)
		log.Print(check)
	}

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
