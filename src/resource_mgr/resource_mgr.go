package resource_mgr

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var URLBASE = "http://storage.game.starlight-stage.jp/"

var ErrNotOK = errors.New("not ok")

type ResourceMgr struct {
	res_ver   string
	cache_dir string
	platform  string
	alvl      string
	slvl      string
	EventList EventDetailList
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

func (r *ResourceMgr) Fetch(loc string) (string, error) {
	dest := r.cache_dir + "/storage/" + loc
	url := URLBASE + loc
	//log.Println("url is", url)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	} else {
		os.MkdirAll(path.Dir(dest), 0755)
	}
	time.Sleep(500 * time.Millisecond)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("http.Get", err)
		return "", ErrNotOK
	}
	if resp.StatusCode != http.StatusOK {
		log.Println(resp.Status)
		return "", ErrNotOK
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("read resp-body", err)
		return "", ErrNotOK
	}
	ioutil.WriteFile(dest, content, 0644)
	return dest, nil
}

func (r *ResourceMgr) FetchLz4(loc string) (string, error) {
	dest := r.cache_dir + "/unlz4/" + loc
	//log.Println("url is", URLBASE+loc)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	} else {
		os.MkdirAll(path.Dir(dest), 0755)
	}
	src, err := r.Fetch(loc)
	if err != nil {
		return "", err
	}
	data := Unlz4(src)
	ioutil.WriteFile(dest, data, 0644)
	return dest, nil
}

func (r *ResourceMgr) Set_res_ver(res_ver string) {
	r.res_ver = res_ver
}

func (r *ResourceMgr) LoadManifest() string {
	base := fmt.Sprintf("dl/%s/", r.res_ver)
	//content, err := ioutil.ReadFile(r.Fetch(base + "manifests/all_dbmanifest"))
	//log.Println(string(content))
	list, err := r.Fetch(base + "manifests/all_dbmanifest")
	if err != nil {
		return ""
	}
	fh, err := os.Open(list)
	if err != nil {
		log.Println("load manifest", err)
		return ""
	}
	bh := bufio.NewReader(fh)
	var manifest_name string
	//var md5 string
	for err != io.EOF {
		var line []byte
		line, _, err = bh.ReadLine()
		field := strings.Split(string(line), ",")
		//log.Println(field)
		if len(field) < 5 {
			continue
		}
		if field[2] == r.platform && field[3] == r.alvl && field[4] == r.slvl {
			manifest_name = field[0]
			//md5 = field[1]
		}
	}
	dest, err := r.FetchLz4(base + "manifests/" + manifest_name)
	if err != nil {
		return ""
	}
	return dest
}

func (r *ResourceMgr) ParseResource(hash string) string {
	manifest := r.LoadManifest()
	db, err := sql.Open("sqlite3", manifest)
	if err != nil {
		log.Println("x1", err)
		return ""
	}
	defer db.Close()
	row := db.QueryRow("SELECT name FROM manifests WHERE hash == $1;", hash)
	var name string
	err = row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			return ""
		} else {
			log.Println("sql error", err)
			return ""
		}
	}
	return name
}

func (r *ResourceMgr) LoadMaster() string {
	dest := r.LoadManifest()
	//log.Println("to open", dest)
	db, err := sql.Open("sqlite3", dest)
	if err != nil {
		log.Println("x1", err)
		return ""
	}
	defer db.Close()
	rows, err := db.Query("SELECT * FROM manifests;")
	//log.Println(rows)
	if err != nil {
		log.Println("x2", err)
		return ""
	}
	defer rows.Close()
	var master string
	for rows.Next() {
		var name string
		var hash string
		var attr int
		var category string
		var decrypt_key []byte
		err = rows.Scan(&name, &hash, &attr, &category, &decrypt_key)
		if name == "master.mdb" {
			//log.Println(name, hash, attr, category, decrypt_key)
			master, err = r.FetchLz4("dl/resources/Generic//" + hash)
			if err != nil {
				return ""
			}
		}
	}
	//log.Println("master.mdb:", master)
	return master
}

func (r *ResourceMgr) LoadMusic() {
	dest := r.LoadManifest()
	//log.Println("to open", dest)
	db, err := sql.Open("sqlite3", dest)
	if err != nil {
		log.Fatal("x1", err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name, hash FROM manifests WHERE name GLOB 'musicscores_*.bdb';")
	//log.Println(rows)
	defer rows.Close()
	if err != nil {
		log.Fatal("x2", err)
	}
	for rows.Next() {
		var name string
		var hash string
		err = rows.Scan(&name, &hash)
		dest, err := r.FetchLz4("dl/resources/Generic//" + hash)
		if err == nil {
			//log.Println(name, hash, dest)
			r.ParseMusic(dest)
		}
	}
}

func (r *ResourceMgr) ParseMusic(fileName string) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Fatal("x1", err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name, data FROM blobs;")
	defer rows.Close()
	if err != nil {
		log.Fatal("x2", err)
	}
	validName := regexp.MustCompile("^musicscores/m\\d+/[a-z0-9_.]+$")
	for rows.Next() {
		var name string
		var data []byte
		err = rows.Scan(&name, &data)
		if err == nil {
			// FIXME not utf-8?
			//fmt.Println(name)
			// not utf8: musicscores/m042/m042_analyzer.bytes
			if validName.MatchString(name) {
				dest := r.cache_dir + "/" + name
				//fmt.Println("write to file", dest)
				if _, err := os.Stat(path.Dir(dest)); err != nil {
					os.MkdirAll(path.Dir(dest), 0755)
				}
				if _, err := os.Stat(dest); err != nil {
					ioutil.WriteFile(dest, data, 0644)
				}
			} else {
				log.Fatalln(name, "invalid filename")
			}
			if utf8.Valid(data) {
				//fmt.Println(name, "UTF-8 valid")
				//fmt.Println(data)
			} else {
				log.Println(name, "UTF-8 invalid")
			}
		}
	}
}

func (r *ResourceMgr) ParseEvent() {
	master := r.LoadMaster()
	//log.Println("master db is", master)
	db, err := sql.Open("sqlite3", master)
	if err != nil {
		log.Println("open masterdb", err)
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT * FROM event_data;")
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
		//log.Println(id, typ, name, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count)
		//ParseTime(notice_start), event_start, second_half_start, event_end, calc_start, result_start, result_end,
		e := &EventDetail{id, typ, name,
			ParseTime(notice_start), ParseTime(event_start), ParseTime(second_half_start), ParseTime(event_end), ParseTime(calc_start), ParseTime(result_start), ParseTime(result_end),
			limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count}
		// deduplicate
		// new override old
		if r.EventList.FindEventById(e.Id()) == nil {
			r.EventList = append(r.EventList, e)
		} else {
			r.EventList.Overwrite(e)
		}
	}
	sort.Sort(r.EventList)
}

func ParseTime(tstr string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", tstr+" +0900 JST")
	if err != nil {
		log.Println("time parse", err)
		return time.Unix(0, 0)
	}
	return t
}

func (r *ResourceMgr) FindCurrentEvent() *EventDetail {
	return FindCurrentEvent(r.EventList)
}

func (r *ResourceMgr) FindLatestEvent() *EventDetail {
	return FindLatestEvent(r.EventList)
}

func (r *ResourceMgr) FindEventById(id int) *EventDetail {
	return r.EventList.FindEventById(id)
}
