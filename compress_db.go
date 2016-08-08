package main

import (
	"log"
	"os"
	"io/ioutil"
	"path"
	"rankserver"
)

var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
var RANK_DB string = BASE + "/data/rankbeta.db"

func main() {
	r := rankserver.MakeRankServer()
	r.UpdateTimestamp()
	tsList := r.GetListTimestamp()
	for _, ts := range tsList {
		log.Println(ts, typ)
	}

}
