package resource_mgr

import (
	"log"
	"os"
	"net/http"
	"path"
	"io/ioutil"
)

var URLBASE = "http://storage.game.starlight-stage.jp/"

type ResourceMgr struct {
	res_ver string
	cache_dir string
	platform string
	alvl string
	slvl string
}

func NewResourceMgr(res_ver string, cache_dir string) *ResourceMgr {
	r := &ResourceMgr{}
	r.res_ver = res_ver
	r.cache_dir = cache_dir
	r.platform = "Android"
	r.alvl = "High"
	r.slvl = "High"
	return r
}

func (r *ResourceMgr) Fetch(loc string) {
	dest := r.cache_dir + "/storage/" + loc
	if _, err := os.Stat(dest); err == nil {
		return
	} else {
		os.MkdirAll(path.Dir(dest), 0755)
	}
	url := URLBASE + loc
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile(dest, content, 0644)
}
