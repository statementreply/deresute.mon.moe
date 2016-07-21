package resource_mgr

import (
	"bufio"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

var URLBASE = "http://storage.game.starlight-stage.jp/"

type ResourceMgr struct {
	res_ver   string
	cache_dir string
	platform  string
	alvl      string
	slvl      string
	EventList []*EventDetail
}

func NewResourceMgr(res_ver string, cache_dir string) *ResourceMgr {
	r := &ResourceMgr{}
	r.res_ver = res_ver
	r.cache_dir = cache_dir
	r.platform = "Android"
	r.alvl = "High"
	r.slvl = "High"
	r.EventList = make([]*EventDetail, 0)
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

func (r *ResourceMgr) LoadManifest() string {
	base := fmt.Sprintf("dl/%s/", r.res_ver)
	//content, _ := ioutil.ReadFile(r.Fetch(base + "manifests/all_dbmanifest"))
	//log.Println(string(content))
	fh, err := os.Open(r.Fetch(base + "manifests/all_dbmanifest"))
	if err != nil {
		log.Fatal(err)
	}
	bh := bufio.NewReader(fh)
	var manifest_name string
	var md5 string
	for err != io.EOF {
		var line []byte
		line, _, err = bh.ReadLine()
		field := strings.Split(string(line), ",")
		log.Println(field)
		if len(field) < 5 {
			continue
		}
		if field[2] == r.platform && field[3] == r.alvl && field[4] == r.slvl {
			manifest_name = field[0]
			md5 = field[1]
		}
	}
	dest := r.FetchLz4(base + "manifests/" + manifest_name)
	log.Println("to open", dest, md5)
	db, err := sql.Open("sqlite3", dest)
	if err != nil {
		log.Fatal("x1", err)
	}
	defer db.Close()
	rows, err := db.Query("select * from manifests;")
	log.Println(rows)
	defer rows.Close()
	if err != nil {
		log.Fatal("x2", err)
	}
	var master string
	for rows.Next() {
		var name string
		var hash string
		var attr int
		var category string
		var decrypt_key []byte
		err = rows.Scan(&name, &hash, &attr, &category, &decrypt_key)
		if name == "master.mdb" {
			log.Println(name, hash, attr, category, decrypt_key)
			master = r.FetchLz4("dl/resources/Generic//" + hash)
		}
	}
	log.Println("master.mdb:", master)
	return master
}

func (r *ResourceMgr) ParseEvent() {
	master := r.LoadManifest()
	db, err := sql.Open("sqlite3", master)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("select * from event_data;")
	defer rows.Close()
	for rows.Next() {
		var id, typ int
		var name string
		var notice_start, event_start, second_half_start, event_end, calc_start, result_start, result_end string
		var limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count int
		err = rows.Scan(&id, &typ,
			&name,
			&notice_start, &event_start, &second_half_start,
			&event_end, &calc_start, &result_start, &result_end,
			&limit_flag, &bg_type, &bg_id, &login_bonus_type, &login_bonus_count)
		//log.Println(id, typ, name,
		//ParseTime(notice_start), event_start, second_half_start, event_end, calc_start, result_start, result_end, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count)
		//log.Println(ParseTime(event_start), ParseTime(calc_start), ParseTime(result_start), ParseTime(result_end))
		log.Println(id, typ, name, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count)
		//ParseTime(notice_start), event_start, second_half_start, event_end, calc_start, result_start, result_end,
		e := &EventDetail{id, typ, name,
			ParseTime(notice_start), ParseTime(event_start), ParseTime(second_half_start), ParseTime(event_end), ParseTime(calc_start), ParseTime(result_start), ParseTime(result_end),
			limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count}
		r.EventList = append(r.EventList, e)
	}
}

func ParseTime(tstr string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", tstr+" +0900 JST")
	if err != nil {
		log.Fatal(err)
	}
	return t
}

func (r *ResourceMgr) FindCurrentEvent() *EventDetail {
	return FindCurrentEvent(r.EventList)
}
