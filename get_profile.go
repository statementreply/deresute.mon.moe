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
	// not necessary
	client.LoadCheck()

	friend_id := 679923520
	if len(os.Args) > 1 {
		friend_id, _ = strconv.Atoi(os.Args[1])
	}
	data := client.GetProfile(friend_id)

	//data = client.GetTourRanking(2, 1)

	yy, _ := yaml.Marshal(data)
	fmt.Println(string(yy))
	fmt.Println("app_ver:", client.GetAppVer())
	fmt.Println("res_ver:", client.GetResVer())
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
