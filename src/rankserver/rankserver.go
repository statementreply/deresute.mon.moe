package rankserver

import (
	"apiclient"
	"crypto/tls"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"resource_mgr"
	"sort"
	"strings"
	"sync"
	"time"
	ts "timestamp"
)

var wg sync.WaitGroup

var BASE string = path.Dir(os.Args[0])
var RANK_DB string = BASE + "/data/rank.db"
var RESOURCE_CACHE_DIR string = BASE + "/data/resourcesbeta/"

// 15min update interval
// *4 for hour
//var INTERVAL int = 15 * 60 * 4
var INTERVAL0 time.Duration = 15 * time.Minute
var INTERVAL time.Duration = 4 * INTERVAL0
var LOG_FILE = "rankserver.log"
var CONFIG_FILE = "rankserver.yaml"
var SECRET_FILE = "secret.yaml"

type RankServer struct {
	//    map[timestamp][rankingType][rank] = score
	// {"1467555420":   [{10: 2034} ,{30: 203021} ]  }
	speed          map[string][]map[int]float32 // need mux
	list_timestamp []string                     // need mutex?
	// for both read and write
	mux_speed     sync.RWMutex
	mux_timestamp sync.RWMutex
	// sql
	rankDB       string
	db           *sql.DB
	logger       *log.Logger
	keyFile      string
	certFile     string
	plainServer  *http.Server
	tlsServer    *http.Server
	hostname     string
	resourceMgr  *resource_mgr.ResourceMgr
	currentEvent *resource_mgr.EventDetail
	client       *apiclient.ApiClient
	lastCheck    time.Time
	config       map[string]string
}

func MakeRankServer() *RankServer {
	r := &RankServer{}
	r.speed = make(map[string][]map[int]float32)
	//r.list_timestamp doesn't need initialization
	r.plainServer = nil
	r.tlsServer = nil

	content, err := ioutil.ReadFile(CONFIG_FILE)
	if err != nil {
		log.Fatalln("read config file", err)
	}
	var config map[string]string
	yaml.Unmarshal(content, &config)
	r.config = config
	fmt.Println(config)

	confLOG_FILE, ok := config["LOG_FILE"]
	if ok {
		LOG_FILE = confLOG_FILE
	}
	log.Print("logfile is ", LOG_FILE)
	fh, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln("open log file", err)
	}
	r.logger = log.New(fh, "", log.LstdFlags)

	r.rankDB = RANK_DB
	r.db, err = sql.Open("sqlite3", "file:"+r.rankDB+"?mode=ro")
	if err != nil {
		r.logger.Fatalln("sql error", err)
	}

	r.keyFile, ok = config["KEY_FILE"]
	if !ok {
		r.keyFile = ""
	}
	r.certFile, ok = config["CERT_FILE"]
	if !ok {
		r.certFile = ""
	}
	r.hostname, ok = config["hostname"]
	if !ok {
		r.logger.Fatalln("no hostname in config")
	}

	if (r.keyFile != "") && (r.certFile != "") {
		r.logger.Print("use https TLS")
		r.logger.Print("keyFile " + r.keyFile + " certFile " + r.certFile)
		cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
		if err != nil {
			r.logger.Fatalln("load keypair", err)
		}
		r.tlsServer = &http.Server{
			Addr:      ":4002",
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		r.plainServer = &http.Server{Addr: ":4001", Handler: http.NewServeMux()}
		r.plainServer.Handler.(*http.ServeMux).HandleFunc("/", r.redirectHandler)
	} else {
		r.logger.Print("use http plaintext")
		r.plainServer = &http.Server{Addr: ":4001"}
	}
	r.setHandleFunc()

	r.client = apiclient.NewApiClientFromConfig(SECRET_FILE)
	r.client.LoadCheck()
	rv := r.client.Get_res_ver()

	r.resourceMgr = resource_mgr.NewResourceMgr(rv, RESOURCE_CACHE_DIR)
	//r.resourceMgr.LoadManifest()
	r.resourceMgr.ParseEvent()
	r.currentEvent = r.resourceMgr.FindCurrentEvent()
	r.lastCheck = time.Now()
	return r
}

func (r *RankServer) setHandleFunc() {
	// for DefaultServeMux
	http.HandleFunc("/", r.homeHandler)
	http.HandleFunc("/m/", r.homeMHandler)
	http.HandleFunc("/event", r.eventHandler)
	http.HandleFunc("/q", r.qHandler)
	http.HandleFunc("/log", r.logHandler)
	http.HandleFunc("/qchart", r.qchartHandler)
	http.HandleFunc("/static/", r.staticHandler)
	// API/plaintext
	http.HandleFunc("/twitter", r.twitterHandler)
	http.HandleFunc("/twitter_emblem", r.twitterEmblemHandler)
	http.HandleFunc("/twitter_trophy", r.twitterTrophyHandler)
	http.HandleFunc("/res_ver", r.res_verHandler)
	http.HandleFunc("/latest_data", r.latestDataHandler)
	http.HandleFunc("/d", r.dataHandler)
}


func (r *RankServer) latestTimestamp() string {
	r.UpdateTimestamp()
	var latest string
	latest = ""
	// skip empty timestamps
	local_timestamp := r.GetListTimestamp()
	for ind := len(local_timestamp) - 1; ind >= 0; ind-- {
		latest = local_timestamp[ind]
		if r.checkDir(latest) {
			break
		}
	}
	return latest
}

func (r *RankServer) inEvent(timestamp string, event *resource_mgr.EventDetail) bool {
	if event == nil {
		return true
	}
	t := ts.TimestampToTime(timestamp)
	if (!t.Before(event.EventStart())) && (!t.After(event.ResultEnd())) {
		return true
	} else {
		return false
	}
}

func (r *RankServer) inEventActive(timestamp string, event *resource_mgr.EventDetail) bool {
	if event == nil {
		return true
	}
	t := ts.TimestampToTime(timestamp)
	if (!t.Before(event.EventStart())) && (!t.After(event.EventEnd())) {
		return true
	} else {
		return false
	}
}

func (r *RankServer) fetchData_i(timestamp string, rankingType int, rank int) interface{} {
	return r.fetchData(timestamp, rankingType, rank)
}

// speed per hour
func (r *RankServer) getSpeed(timestamp string, rankingType int, rank int) float32 {
	r.mux_speed.RLock()
	_, ok := r.speed[timestamp]
	r.mux_speed.RUnlock()
	if !ok {
		// initialize keyvalue
		r.mux_speed.Lock()
		r.speed[timestamp] = make([]map[int]float32, 2)
		r.speed[timestamp][0] = make(map[int]float32)
		r.speed[timestamp][1] = make(map[int]float32)
		r.mux_speed.Unlock()
	} else {
		r.mux_speed.RLock()
		val, ok := r.speed[timestamp][rankingType][rank]
		r.mux_speed.RUnlock()
		if ok {
			return val
		}
	}
	t_i := ts.TimestampToTime(timestamp)
	t_prev := t_i.Add(-INTERVAL)
	prev_timestamp := ts.TimeToTimestamp(t_prev)

	cur_score := r.fetchData(timestamp, rankingType, rank)
	prev_score := r.fetchData(prev_timestamp, rankingType, rank)
	if (cur_score >= 0) && (prev_score >= 0) {
		// both score are valid
		// nanoseconds
		speed := (float32(cur_score - prev_score)) / float32(INTERVAL) * float32(time.Hour)
		r.mux_speed.Lock()
		r.speed[timestamp][rankingType][rank] = speed
		r.mux_speed.Unlock()
		return speed
	} else {
		// one of them is missing data
		return -1.0
	}
}

func (r *RankServer) getSpeed_i(timestamp string, rankingType int, rank int) interface{} {
	return r.getSpeed(timestamp, rankingType, rank)
}

func (r *RankServer) run() {
	if r.tlsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.tlsServer.ListenAndServeTLS(r.certFile, r.keyFile)
			if err != nil {
				r.logger.Fatalln("tlsServer", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := r.plainServer.ListenAndServe()
		if err != nil {
			r.logger.Fatalln("plainServer", err)
		}
	}()
}

func (r *RankServer) latestData() string {
	timestamp := r.latestTimestamp()
	return r.showData(timestamp)
}



func (r *RankServer) showData(timestamp string) string {
	item := r.fetchDataSlice(timestamp)
	yy, err := yaml.Marshal(item)
	if err != nil {
		log.Println(err)
		return ""
	}
	st := ts.FormatTimestamp(timestamp)
	return timestamp + "\n" + st + "\n" + string(yy)
}

func (r *RankServer) GetListTimestamp() []string {
	r.mux_timestamp.RLock()
	local_timestamp := make([]string, len(r.list_timestamp))
	copy(local_timestamp, r.list_timestamp)
	r.mux_timestamp.RUnlock()
	return local_timestamp
}

func (r *RankServer) get_list_rank(timestamp string, rankingType int) []int {
	list_rank := r.fetchDataListRank(timestamp, rankingType)
	sort.Ints(list_rank)
	return list_rank
}

// js map syntax
// {"cols":  [{"id":"timestamp","label":"timestamp","type":"date"}, {"id":"score","label":"score","type":"number"}],
//  "rows":  [{"c":[{"v":"new Date(1467770520)"}, {"v":14908}]}] }
func (r *RankServer) rankData_list_f_e(rankingType int, list_rank []int, dataSource func(string, int, int) interface{}, event *resource_mgr.EventDetail) string {
	r.UpdateTimestamp()
	raw := ""
	raw += `{"cols":[{"id":"timestamp","label":"timestamp","type":"datetime"},`
	for _, rank := range list_rank {
		raw += fmt.Sprintf(`{"id":"%d","label":"%d","type":"number"},`, rank, rank)
	}
	raw += "\n"
	raw += `],"rows":[`

	local_timestamp := r.GetListTimestamp()
	for _, timestamp := range local_timestamp {
		if !r.inEvent(timestamp, event) {
			continue
		}
		// time in milliseconds
		raw += fmt.Sprintf(`{"c":[{"v":new Date(%s000)},`, timestamp)
		for _, rank := range list_rank {
			score := dataSource(timestamp, rankingType, rank)
			switch score.(type) {
			case int:
				score_i := score.(int)
				if score_i >= 0 {
					raw += fmt.Sprintf(`{"v":%d},`, score_i)
				} else {
					// null: missing point
					raw += fmt.Sprintf(`{"v":null},`)
				}
			case float32:
				score_f := score.(float32)
				if score_f >= 0 {
					raw += fmt.Sprintf(`{"v":%f},`, score_f)
				} else {
					// null: missing point
					raw += fmt.Sprintf(`{"v":null},`)
				}
			}
		}
		raw += fmt.Sprintf(`]},`)
		raw += "\n"
	}
	raw += `]}`
	return raw
}

func (r *RankServer) jsonData(rankingType int, list_rank []int, dataSource func(string, int, int) interface{}, event *resource_mgr.EventDetail) string {
	r.UpdateTimestamp()
	// begin list
	raw := "[["
	for _, rank := range list_rank {
		raw += fmt.Sprintf(`"%d",`, rank)
	}
	raw = strings.TrimSuffix(raw, ",")
	raw += "],\n"

	local_timestamp := r.GetListTimestamp()
	for _, timestamp := range local_timestamp {
		if !r.inEvent(timestamp, event) {
			continue
		}
		// time in milliseconds
		raw += fmt.Sprintf(`["%s",`, timestamp)
		for _, rank := range list_rank {
			score := dataSource(timestamp, rankingType, rank)
			switch score.(type) {
			case int:
				score_i := score.(int)
				if score_i >= 0 {
					raw += fmt.Sprintf(`%d,`, score_i)
				} else {
					// null: missing point
					raw += fmt.Sprintf(`null,`)
				}
			case float32:
				score_f := score.(float32)
				if score_f >= 0 {
					raw += fmt.Sprintf(`%f,`, score_f)
				} else {
					// null: missing point
					raw += fmt.Sprintf(`null,`)
				}
			}
		}
		raw = strings.TrimSuffix(raw, ",")
		raw += fmt.Sprintf("]\n,")
		//raw += "\n"
	}
	raw = strings.TrimSuffix(raw, ",")
	raw += `]`
	return raw
}

func (r *RankServer) rankData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.fetchData_i, event)
}

func (r *RankServer) speedData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.getSpeed_i, event)
}


func Main() {
	log.Print("RankServer running")
	r := MakeRankServer()
	r.run()
	wg.Wait()
}
