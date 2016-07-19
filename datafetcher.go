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
	client.LoadCheck()

	p1 := client.GetAtaponRanking(1, 9)
	//DumpToFile(p1, "r1.009.20")
	DumpToStdout(p1)

}

func DumpToStdout(v interface{}) {
	yy, _ := yaml.Marshal(v)
	fmt.Println(string(yy))
}

func DumpToFile(v interface{}, fileName string) {
	yy, _ := yaml.Marshal(v)
	ioutil.WriteFile(fileName, yy, 0644)
}
