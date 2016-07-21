package resource_mgr

import (
	"log"
	"os"
	"net/http"
	"path"
	"io/ioutil"
	"fmt"
	"io"
	"bufio"
	"strings"
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

func (r *ResourceMgr) Fetch(loc string) string {
	dest := r.cache_dir + "/storage/" + loc
	if _, err := os.Stat(dest); err == nil {
		return dest
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
	return dest
}

func (r *ResourceMgr) FetchLz4(loc string) string {
	dest := r.cache_dir + "/unlz4/" + loc
	if _, err := os.Stat(dest); err == nil {
		return dest
	} else {
		os.MkdirAll(path.Dir(dest), 0755)
	}
	src := r.Fetch(loc)
	data := Unlz4(src)
	ioutil.WriteFile(dest, data, 0644)
	return dest
}


func (r *ResourceMgr) LoadManifest() {
	base := fmt.Sprintf("dl/%s/", r.res_ver)
	//content, _ := ioutil.ReadFile(r.Fetch(base + "manifests/all_dbmanifest"))
	//log.Println(string(content))
	fh, err := os.Open(r.Fetch(base + "manifests/all_dbmanifest"))
	bh := bufio.NewReader(fh)
	var manifest_name string
	var md5 string
	for err != io.EOF {
		var line []byte
		line, _, err = bh.ReadLine()
		field := strings.Split(string(line), ",")
		log.Println(field)
		if (len(field) < 5) {
			continue
		}
		if field[2] == r.platform && field[3] == r.alvl && field[4] == r.slvl {
			manifest_name = field[0]
			md5 = field[1]
		}
	}
	pp := r.FetchLz4(base + "manifests/" + manifest_name)
	log.Println(pp, md5)

}
