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

package rankserver

import (
	"apiclient"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"resource_mgr"
	"runtime/pprof"
	"sort"
	"stoppableListener"
	"strings"
	"sync"
	"syscall"
	"time"
	ts "timestamp"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var wg sync.WaitGroup

var BASE string = path.Dir(os.Args[0])
var RANK_DB string = BASE + "/data/rank.db"
var RESOURCE_CACHE_DIR string = BASE + "/data/resourcesbeta/"

// 15min update interval
// *4 for hour
//var INTERVAL int = 15 * 60 * 4
var INTERVAL0 time.Duration = 15 * time.Minute
var INTERVAL time.Duration = 4 * INTERVAL0 // * 24
var LOG_FILE = "rankserver.log"
var CONFIG_FILE = "rankserver.yaml"
var SECRET_FILE = "secret.yaml"

func MakeRankServer() *RankServer {
	r := &RankServer{}
	//r.speed = make(map[string][]map[int]float32)
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

	r.openLog()

	r.rankDB = RANK_DB
	r.db, err = sql.Open("sqlite3", "file:"+r.rankDB+"?mode=ro")
	if err != nil {
		r.logger.Fatalln("sql error open file", err)
	}
	//r.setCacheSize()

	var ok bool
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
			Addr: ":4002",
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				// can be omitted
				NextProtos: []string{"h2", "http/1.1"},
			},
		}
		r.plainServer = &http.Server{Addr: ":4001", Handler: http.NewServeMux()}
		r.plainServer.Handler.(*http.ServeMux).HandleFunc("/", r.redirectHandler)
	} else {
		r.logger.Print("use http plaintext")
		r.plainServer = &http.Server{Addr: ":4001"}
	}
	r.setHandleFunc()

	// stoppable listener prepare
	listenerHTTP, err := net.Listen("tcp", r.plainServer.Addr)
	if err != nil {
		r.logger.Fatalln("listenerHTTP", err)
	}
	slHTTP, err := stoppableListener.New(listenerHTTP)
	if err != nil {
		r.logger.Fatalln("listenerHTTP", err)
	}
	r.slHTTP = slHTTP

	if r.tlsServer != nil {
		listenerTLS, err := net.Listen("tcp", r.tlsServer.Addr)
		if err != nil {
			r.logger.Fatalln("listenerTLS", err)
		}
		slTLS, err := stoppableListener.New(listenerTLS)
		if err != nil {
			r.logger.Fatalln("listenerTLS", err)
		}
		r.slTLS = slTLS
	}

	r.client = apiclient.NewApiClientFromConfig(SECRET_FILE)
	r.client.LoadCheck()
	rv := r.client.Get_res_ver()

	r.resourceMgr = resource_mgr.NewResourceMgr(rv, RESOURCE_CACHE_DIR)
	//r.resourceMgr.LoadManifest()
	r.resourceMgr.ParseEvent()
	r.currentEvent = r.resourceMgr.FindCurrentEvent()
	r.latestEvent = r.resourceMgr.FindLatestEvent()
	//log.Println(r.currentEvent.Name(), r.latestEvent.Name())
	r.lastCheck = time.Now()
	return r
}

func (r *RankServer) openLog() {
	confLOG_FILE, ok := r.config["LOG_FILE"]
	if ok {
		LOG_FILE = confLOG_FILE
	}
	log.Print("logfile is ", LOG_FILE)
	fh, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		log.Fatalln("open log file", err)
	}
	if r.logger == nil {
		r.logger = log.New(fh, "", log.LstdFlags)
	} else {
		r.logger.SetOutput(fh)
	}
	if r.log_fh != nil {
		r.log_fh.Close()
	}
	r.log_fh = fh
}

func (r *RankServer) setHandleFunc() {
	// for DefaultServeMux
	// html/template
	http.HandleFunc("/", r.homeHandler)
	http.HandleFunc("/qchart", r.qchartHandler)
	http.HandleFunc("/q", r.qHandler)
	http.HandleFunc("/log", r.logHandler)
	http.HandleFunc("/event", r.eventHandler)
	http.HandleFunc("/dist", r.distHandler)
	http.HandleFunc("/dist_compare", r.distCompareHandler)
	http.HandleFunc("/time_compare", r.timeCompareHandler)
	http.HandleFunc("/twc", r.twcHandler)
	http.HandleFunc("/twc_test", r.twcTestHandler)
	// auxiliary
	http.HandleFunc("/static/", r.staticHandler)
	// early testing
	http.HandleFunc("/m/", r.homeMHandler) // only for test
	// API/plaintext
	http.HandleFunc("/twitter", r.twitterHandler)
	http.HandleFunc("/twitter_emblem", r.twitterEmblemHandler)
	http.HandleFunc("/twitter_trophy", r.twitterTrophyHandler)
	http.HandleFunc("/res_ver", r.res_verHandler)
	http.HandleFunc("/latest_data", r.latestDataHandler)
	http.HandleFunc("/d", r.dataHandler)
	http.HandleFunc("/d2", r.dataHandlerV2)
	http.HandleFunc("/d_dist", r.distDataHandler)
	http.HandleFunc("/d2_dist", r.distDataHandlerV2)
	http.HandleFunc("/d_dist_compare", r.distCompareDataHandler)
	http.HandleFunc("/d_event", r.eventDataHandler)
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

// bad efficiency?
func (r *RankServer) latestEventTimestamp(event *resource_mgr.EventDetail) string {
	r.UpdateTimestamp()
	local_timestamp := r.GetListTimestamp()
	for ind := len(local_timestamp) - 1; ind >= 0; ind-- {
		t1 := local_timestamp[ind]
		time1 := ts.TimestampToTime(t1)
		if event.InPeriod(time1) {
			return t1
		}
	}
	return ""
}

// latestTimestamp() == truncateTimestamp(time.Now())
func (r *RankServer) truncateTimestamp(t time.Time) string {
	r.UpdateTimestamp()
	var latest string
	latest = ""
	// skip empty timestamps
	local_timestamp := r.GetListTimestamp()
	for ind := len(local_timestamp) - 1; ind >= 0; ind-- {
		latest = local_timestamp[ind]
		latest_t := ts.TimestampToTime(latest)
		if (!latest_t.After(t)) && r.checkDir(latest) {
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

// return current event or the latest previous event
/*
func (r *RankServer) latestEvent() *resource_mgr.EventDetail {
	// reverse pass
	for i := len(r.resourceMgr.EventList)-1; i >= 0; i-- {
		e := r.resourceMgr.EventList[i]
		if e.HasRanking() && !e.EventStart().After(time.Now()) {
			return e
		}
	}
	return nil
}
*/

func (r *RankServer) fetchData_i(timestamp string, rankingType int, rank int, delta time.Duration) interface{} {
	return r.fetchData(timestamp, rankingType, rank)
}

// speed per hour
func (r *RankServer) getSpeed(timestamp string, rankingType int, rank int, delta time.Duration) float32 {
	t_i := ts.TimestampToTime(timestamp)
	t_prev := t_i.Add(-delta)
	prev_timestamp := ts.TimeToTimestamp(t_prev)

	cur_score := r.fetchData(timestamp, rankingType, rank)
	prev_score := r.fetchData(prev_timestamp, rankingType, rank)
	if (cur_score >= 0) && (prev_score >= 0) {
		// both score are valid
		// nanoseconds
		speed := (float32(cur_score - prev_score)) / float32(delta) * float32(time.Hour)
		return speed
	} else {
		// one of them is missing data
		return -1.0
	}
}

// new api
func (r *RankServer) getSpeedBorder(timestamp_start, timestamp_end string, rankingType int, rank int, delta time.Duration) map[string]float32 {
	var timestamp_start_prev string
	{
		t_i := ts.TimestampToTime(timestamp_start)
		t_prev := t_i.Add(-delta)
		timestamp_start_prev = ts.TimeToTimestamp(t_prev)
	}
	border := r.fetchDataBorder(timestamp_start_prev, timestamp_end, rankingType, rank)
	borderSpeed := map[string]float32{}
	for timestamp, cur_score := range border {
		if timestamp < timestamp_start {
			continue
		}
		//cur_score := border[timestamp]
		var timestamp_prev string
		{
			t_i := ts.TimestampToTime(timestamp)
			t_prev := t_i.Add(-delta)
			timestamp_prev = ts.TimeToTimestamp(t_prev)
		}
		prev_score, ok := border[timestamp_prev]
		if ok {
			borderSpeed[timestamp] = (float32(cur_score - prev_score)) / float32(delta) * float32(time.Hour)
		} else {
			borderSpeed[timestamp] = -1.0
		}
	}
	return borderSpeed
}

func (r *RankServer) getSpeed_i(timestamp string, rankingType int, rank int, delta time.Duration) interface{} {
	return r.getSpeed(timestamp, rankingType, rank, delta)
}

// doesn't block
func (r *RankServer) run() {
	if r.tlsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//err := r.tlsServer.ListenAndServeTLS(r.certFile, r.keyFile)
			tlsListener := tls.NewListener(
				tcpKeepAliveListener{r.slTLS},
				r.tlsServer.TLSConfig)
			err := r.tlsServer.Serve(tlsListener)
			if err != nil {
				r.logger.Println("tlsServer stopped", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		//err := r.plainServer.ListenAndServe()
		err := r.plainServer.Serve(tcpKeepAliveListener{r.slHTTP})
		if err != nil {
			r.logger.Println("plainServer stopped", err)
		}
	}()
}

func (r *RankServer) stop() {
	r.logger.Println("stopping server")
	r.slHTTP.Stop()
	if r.slTLS != nil {
		r.slTLS.Stop()
	}
	wg.Wait()
	r.db.Close()
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

// json
func (r *RankServer) jsonData(rankingType int, list_rank []int, dataSource func(string, int, int, time.Duration) interface{}, event *resource_mgr.EventDetail, delta time.Duration) string {
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
			score := dataSource(timestamp, rankingType, rank, delta)
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

func Main() {
	log.Print("RankServer running")
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	r := MakeRankServer()
	r.run()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	ch_reload := make(chan os.Signal)
	signal.Notify(ch_reload, syscall.SIGHUP)
	go func() {
		for {
			select {
			case s := <-ch_reload:
				r.openLog()
				r.logger.Println("reopened logfile", s)
			}
		}
	}()
	// wait for SIGTERM
	r.logger.Println(<-ch)
	r.stop()
	log.Print("RankServer exiting")
}
