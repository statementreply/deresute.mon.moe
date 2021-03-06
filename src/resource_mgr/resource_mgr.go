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
//
// Ported from the Python implementation in deresute.me
//     <https://github.com/marcan/deresuteme>
//     Copyright 2016-2017 Hector Martin <marcan@marcan.st>
//     Licensed under the Apache License, Version 2.0

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

// FIXME: no https yet?
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
	log.Println("Fetch url is: " + url)
	// add custom header
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("NewRequest bad", err)
		return "", ErrNotOK
	}
	// temp FIX FIXME
	req.Header.Add("X-Unity-Version", "5.1.2f1")
	//resp, err := http.Get(url)
	resp, err := http.DefaultClient.Do(req)
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
	db, err := sql.Open("sqlite3", "file:" + manifest + "?mode=ro")
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
	manifest := r.LoadManifest()
	//log.Println("to open", dest)
	db, err := sql.Open("sqlite3", "file:" + manifest + "?mode=ro")
	if err != nil {
		log.Println("x1", err)
		return ""
	}
	defer db.Close()
	rows, err := db.Query("SELECT name, hash, attr, category, decrypt_key FROM manifests;")
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
		// FIXME err handling
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
	manifest := r.LoadManifest()
	//log.Println("to open", dest)
	db, err := sql.Open("sqlite3", "file:" + manifest + "?mode=ro")
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
	db, err := sql.Open("sqlite3", "file:" + fileName + "?mode=ro")
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
	db, err := sql.Open("sqlite3", "file:" + master + "?mode=ro")
	if err != nil {
		log.Println("open masterdb", err)
		return
	}
	defer db.Close()
	// FIXME: schema dependent
	rows, err := db.Query("SELECT id, type, name, notice_start, event_start, second_half_start, event_end, calc_start, result_start, result_end, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count, master_plus_support FROM event_data;")
	defer rows.Close()
	for rows.Next() {
		var id, typ int
		var name string
		var notice_start, event_start, second_half_start, event_end, calc_start, result_start, result_end string
		var limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count, master_plus_support int
		// CREATE TABLE IF NOT EXISTS 'event_data' ('id' INTEGER NOT NULL, 'type' INTEGER NOT NULL, 'name' TEXT NOT NULL, 'notice_start' TEXT NOT NULL, 'event_start' TEXT NOT NULL, 'second_half_start' TEXT NOT NULL, 'event_end' TEXT NOT NULL, 'calc_start' TEXT NOT NULL, 'result_start' TEXT NOT NULL, 'result_end' TEXT NOT NULL, 'limit_flag' INTEGER NOT NULL, 'bg_type' INTEGER NOT NULL, 'bg_id' INTEGER NOT NULL, 'login_bonus_type'INTEGER NOT NULL, 'login_bonus_count' INTEGER NOT NULL, 'master_plus_support' INTEGER NOT NULL, 'item_re' INTEGER NOT NULL, PRIMARY KEY('id'));
		err = rows.Scan(&id, &typ,
			&name,
			&notice_start, &event_start, &second_half_start,
			&event_end, &calc_start, &result_start, &result_end,
			&limit_flag, &bg_type, &bg_id, &login_bonus_type, &login_bonus_count, &master_plus_support)
		if err != nil {
			log.Println("sql error", err)
		}
		//log.Println(id, typ, name,
		//ParseTime(notice_start), event_start, second_half_start, event_end, calc_start, result_start, result_end, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count)
		//log.Println(ParseTime(event_start), ParseTime(calc_start), ParseTime(result_start), ParseTime(result_end))
		//log.Println(id, typ, name, limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count)
		//ParseTime(notice_start), event_start, second_half_start, event_end, calc_start, result_start, result_end,
		// FIXME: order-dependent**
		e := &EventDetail{id, typ, name,
			ParseTime(notice_start), ParseTime(event_start), ParseTime(second_half_start), ParseTime(event_end), ParseTime(calc_start), ParseTime(result_start), ParseTime(result_end),
			limit_flag, bg_type, bg_id, login_bonus_type, login_bonus_count, master_plus_support, ""}
		if e.typ == 3 || e.typ == 5 {
			e.music_name = r.FindMedleyTitleV2(e, db)
			//log.Println("find groove music name", e.music_name)
		}
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
	//log.Println("tstr is <", tstr, ">")
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

// medley event id to music title
// INPUT: EventDetail struct
// OUTPUT: a string of music title, or "" if not found
// BUG/FIXME: sort is bad...
func (r *ResourceMgr) FindMedleyTitle(e *EventDetail, db *sql.DB) string {
	var id int
	id = e.Id()
	var typ int
	typ = e.Type()

	var story_id int
	story_id = -1
	var category_id int
	var title string

	var row *sql.Row
	var err error

	// FIXME sanity check id is in range 1000-10000?

	if typ == 3 {
		goto Map01Medley
	} else if typ == 5 {
		goto Map01Tour
	} else if typ == 1 {
		goto Map01Atapon
	} else {
		return ""
	}
Map01Medley:
	row = db.QueryRow("SELECT id FROM medley_story_detail WHERE event_id=$1;", id)
	err = row.Scan(&story_id)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("err scan music_data_id")
		}
		return ""
	}
	goto Map02

Map01Tour:
	row = db.QueryRow("SELECT id FROM tour_story_detail WHERE event_id=$1;", id)
	err = row.Scan(&story_id)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("err scan music_data_id")
		}
		return ""
	}
	goto Map02

Map01Atapon:
	row = db.QueryRow("SELECT id FROM atapon_story_detail WHERE event_id=$1;", id)
	err = row.Scan(&story_id)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("err scan music_data_id")
		}
		return ""
	}
	goto Map02

Map02:
	row = db.QueryRow("SELECT category_id FROM story_detail WHERE id=$1;", story_id)
	err = row.Scan(&category_id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		log.Println("err music_data")
		return ""
	}

	//Map03:
	row = db.QueryRow("SELECT title FROM story_category WHERE id=$1;", category_id)
	err = row.Scan(&title)
	if err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		log.Println("err music_data")
		return ""
	}

	log.Println("[INFO]", "event_id", id, "story_id", story_id, "category_id", category_id, "title", title)
	return title
}

func (r *ResourceMgr) FindMedleyTitleV2(e *EventDetail, db *sql.DB) string {
	var id int
	id = e.Id()
	var typ int
	typ = e.Type()

	var story_id int
	story_id = -1
	var category_id int
	var title string

	var row *sql.Row

	// FIXME sanity check id is in range 1000-10000?

	if typ == 3 {
		row = db.QueryRow("SELECT medley_story_detail.id, story_detail.category_id, story_category.title FROM medley_story_detail INNER JOIN story_detail ON medley_story_detail.id = story_detail.id INNER JOIN story_category ON story_category.id = story_detail.category_id WHERE event_id=$1;", id)
	} else if typ == 5 {
		row = db.QueryRow("SELECT tour_story_detail.id, story_detail.category_id, story_category.title FROM tour_story_detail INNER JOIN story_detail ON tour_story_detail.id = story_detail.id INNER JOIN story_category ON story_category.id = story_detail.category_id WHERE event_id=$1;", id)
	} else if typ == 1 {
		row = db.QueryRow("SELECT atapon_story_detail.id, story_detail.category_id, story_category.title FROM atapon_story_detail INNER JOIN story_detail ON atapon_story_detail.id = story_detail.id INNER JOIN story_category ON story_category.id = story_detail.category_id WHERE event_id=$1;", id)
	} else {
		return ""
	}
	err := row.Scan(&story_id, &category_id, &title)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("err NOROWS scan joined query")
		}
		return ""
	}

	log.Println("[INFO]", "event_id", id, "story_id", story_id, "category_id", category_id, "title", title)
	return title
}
