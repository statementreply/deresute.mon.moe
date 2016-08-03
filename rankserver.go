package main

import (
	"apiclient"
	"crypto/tls"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"resource_mgr"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	ts "timestamp"
	"unicode/utf8"
)

var wg sync.WaitGroup

var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
var RESOURCE_CACHE_DIR string = BASE + "/data/resourcesbeta/"

// 15min update interval
// *4 for hour
//var INTERVAL int = 15 * 60 * 4
var INTERVAL0 time.Duration = 15 * time.Minute
var INTERVAL time.Duration = 4 * INTERVAL0
var LOG_FILE = "rankserver.log"
var CONFIG_FILE = "rankserver.yaml"
var SECRET_FILE = "secret.yaml"
var dirNameFilter = regexp.MustCompile("^\\d+$")
var fileNameFilter = regexp.MustCompile("r\\d{2}\\.(\\d+)$")
var rankingTypeFilter = regexp.MustCompile("r01\\.\\d+$")

type RankServer struct {
	//    map[timestamp][rankingType][rank] = score
	// {"1467555420":   [{10: 2034} ,{30: 203021} ]  }
	data           map[string][]map[int]int     // need mux
	speed          map[string][]map[int]float32 // need mux
	list_timestamp []string                     // need mutex?
	// for both read and write
	mux           sync.RWMutex
	mux_speed     sync.RWMutex
	mux_timestamp sync.RWMutex
	logger        *log.Logger
	keyFile       string
	certFile      string
	plainServer   *http.Server
	tlsServer     *http.Server
	hostname      string
	resourceMgr   *resource_mgr.ResourceMgr
	currentEvent  *resource_mgr.EventDetail
	client        *apiclient.ApiClient
	lastCheck     time.Time
	config        map[string]string
}

func MakeRankServer() *RankServer {
	r := &RankServer{}
	r.data = make(map[string][]map[int]int)
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
}

func (r *RankServer) updateTimestamp() {
	dir, err := os.Open(RANK_CACHE_DIR)
	if err != nil {
		// FIXME
		r.logger.Println("rank cache dir doesnt exist", RANK_CACHE_DIR, err)
		os.MkdirAll(RANK_CACHE_DIR, 0755)
		return
	}
	defer dir.Close()

	fi, err := dir.Readdir(0)
	if err != nil {
		// FIXME
		r.logger.Println(err)
		return
	}
	r.mux_timestamp.Lock()
	r.list_timestamp = make([]string, 0, len(fi))
	// sub: dir name 1467555420
	for _, sub := range fi {
		subdirName := sub.Name()
		if dirNameFilter.MatchString(subdirName) {
			r.list_timestamp = append(r.list_timestamp, sub.Name())
		}
	}
	sort.Strings(r.list_timestamp)
	r.mux_timestamp.Unlock()
}

func (r *RankServer) latestTimestamp() string {
	r.updateTimestamp()
	var latest string
	latest = ""
	// skip empty timestamps
	local_timestamp := r.get_list_timestamp()
	for ind := len(local_timestamp) - 1; ind >= 0; ind-- {
		latest = local_timestamp[ind]
		if r.checkDir(latest) {
			break
		}
	}
	return latest
}

// true: nonempty; false: empty
func (r *RankServer) checkDir(timestamp string) bool {
	subdirPath := RANK_CACHE_DIR + timestamp + "/"
	subdir, err := os.Open(subdirPath)
	if err != nil {
		// FIXME
		r.logger.Println("opendir", err)
		return false
	}
	defer subdir.Close()
	key, err := subdir.Readdir(0)
	if err != nil {
		r.logger.Println("readdir", err)
		return false
	}
	if len(key) > 0 {
		return true
	} else {
		return false
	}
}

func (r *RankServer) checkData(timestamp string) {
	r.updateTimestamp()
	latest := r.latestTimestamp()
	latest_time := time.Unix(0, 0)
	if latest != "" {
		latest_time = ts.TimestampToTime(latest)
	}
	// check new res_ver
	// FIXME need some test
	if (time.Now().Sub(r.lastCheck) >= 5*time.Hour/2) || ((r.currentEvent == nil) && (time.Now().Sub(latest_time) <= 2*time.Hour)) {
		r.logger.Println("recheck res_ver, lastcheck:", r.lastCheck, "latest_time:", latest_time)
		r.client.LoadCheck()
		rv := r.client.Get_res_ver()
		r.resourceMgr.Set_res_ver(rv)
		r.resourceMgr.ParseEvent()
		r.currentEvent = r.resourceMgr.FindCurrentEvent()
		r.lastCheck = time.Now()
	}

	if timestamp == "" {
		timestamp = latest
	}
	subdirPath := RANK_CACHE_DIR + timestamp + "/"

	subdir, err := os.Open(subdirPath)
	if err != nil {
		r.logger.Println("opendir", err)
		return
	}
	defer subdir.Close()

	key, err := subdir.Readdir(0)
	if err != nil {
		r.logger.Println("readdir", err)
		return
	}
	for _, pt := range key {
		rankingType := r.RankingType(pt.Name())
		rank := r.FilenameToRank(pt.Name())
		if rank == 0 {
			// lock file
			continue
		}
		r.fetchData(timestamp, rankingType, rank)
	}
}

func (r *RankServer) getFilename(timestamp string, rankingType, rank int) string {
	subdirPath := RANK_CACHE_DIR + timestamp + "/"
	a := rankingType + 1
	b := int((rank-1)/10) + 1
	fileName := subdirPath + fmt.Sprintf("r%02d.%06d", a, b)

	return fileName
}

func (r *RankServer) FilenameToRank(fileName string) int {
	//log.Print("fileName", fileName)
	submatch := fileNameFilter.FindStringSubmatch(fileName)
	if len(submatch) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(submatch[1])
	//log.Print("fileName", fileName, "n", n, "submatch", submatch)
	return (n-1)*10 + 1
}

func (r *RankServer) RankingType(fileName string) int {
	if rankingTypeFilter.MatchString(fileName) {
		// event pt
		return 0 // r01.xxxxxx
	} else {
		// high score
		return 1 // r02.xxxxxx
	}
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

func (r *RankServer) fetchData(timestamp string, rankingType int, rank int) int {
	fileName := r.getFilename(timestamp, rankingType, rank)
	return r.fetchData_internal(timestamp, rankingType, rank, fileName)
}

func (r *RankServer) fetchData_i(timestamp string, rankingType int, rank int) interface{} {
	return r.fetchData(timestamp, rankingType, rank)
}

func (r *RankServer) fetchData_internal(timestamp string, rankingType int, rank int, fileName string) int {
	r.mux.RLock()
	_, ok := r.data[timestamp]
	r.mux.RUnlock()
	if !ok {
		// initialize keyvalue
		r.mux.Lock()
		r.data[timestamp] = make([]map[int]int, 2)
		r.data[timestamp][0] = make(map[int]int)
		r.data[timestamp][1] = make(map[int]int)
		r.mux.Unlock()
	} else {
		r.mux.RLock()
		score, ok := r.data[timestamp][rankingType][rank]
		r.mux.RUnlock()
		if ok {
			return score
		}
	}

	if r.isLocked(fileName) {
		r.logger.Println("data/rank/... locked, return -1")
		return -1
	}

	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		// file doesn't exist?
		// return -1 for missing data
		//r.logger.Println(err, "return -1")
		return -1
	}
	// potential read/write race
	if len(content) == 0 {
		r.logger.Println(fileName, "empty content return -1")
		return -1
	}

	var local_rank_list []map[string]interface{}
	err = yaml.Unmarshal(content, &local_rank_list)
	if err != nil {
		r.logger.Println("YAML error, return -1", err)
		return -1
	}

	var score int
	score = 0
	if len(local_rank_list) > 0 {
		score = local_rank_list[0]["score"].(int)
	}
	r.mux.Lock()
	r.data[timestamp][rankingType][rank] = score
	r.mux.Unlock()
	return score
}

func (r *RankServer) isLocked(fileName string) bool {
	lockFile := "lock"
	dirname := path.Base(fileName)
	lockPath := path.Join(dirname, lockFile)
	_, err := os.Stat(lockPath)
	if (err != nil) && (os.IsNotExist(err)) {
		return false
	} else {
		return true
	}
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
		r.mux.RLock()
		val, ok := r.speed[timestamp][rankingType][rank]
		r.mux.RUnlock()
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

func (r *RankServer) dumpData() string {
	r.mux.RLock()
	yy, _ := yaml.Marshal(r.data)
	r.mux.RUnlock()
	return string(yy)
}

func (r *RankServer) latestData() string {
	timestamp := r.latestTimestamp()
	return r.showData(timestamp)
}

func (r *RankServer) showData(timestamp string) string {
	r.mux.RLock()
	item, ok := r.data[timestamp]
	if !ok {
		return ""
	}
	yy, _ := yaml.Marshal(item)
	r.mux.RUnlock()
	st := ts.FormatTimestamp(timestamp)
	return timestamp + "\n" + st + "\n" + string(yy)
}

func (r *RankServer) get_list_timestamp() []string {
	r.mux_timestamp.RLock()
	local_timestamp := make([]string, len(r.list_timestamp))
	copy(local_timestamp, r.list_timestamp)
	r.mux_timestamp.RUnlock()
	return local_timestamp
}

func (r *RankServer) get_list_rank(timestamp string, rankingType int) []int {
	r.mux.RLock()
	local_map := r.data[timestamp][rankingType]
	list_rank := make([]int, 0, len(local_map))
	for k := range local_map {
		list_rank = append(list_rank, k)
	}
	r.mux.RUnlock()
	sort.Ints(list_rank)
	return list_rank
}

// js map syntax
// {"cols":  [{"id":"timestamp","label":"timestamp","type":"date"}, {"id":"score","label":"score","type":"number"}],
//  "rows":  [{"c":[{"v":"new Date(1467770520)"}, {"v":14908}]}] }
func (r *RankServer) rankData_list_f_e(rankingType int, list_rank []int, dataSource func(string, int, int) interface{}, event *resource_mgr.EventDetail) string {
	r.updateTimestamp()
	raw := ""
	raw += `{"cols":[{"id":"timestamp","label":"timestamp","type":"datetime"},`
	for _, rank := range list_rank {
		raw += fmt.Sprintf(`{"id":"%d","label":"%d","type":"number"},`, rank, rank)
	}
	raw += "\n"
	raw += `],"rows":[`

	local_timestamp := r.get_list_timestamp()
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

func (r *RankServer) rankData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.fetchData_i, event)
}

func (r *RankServer) speedData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.getSpeed_i, event)
}

func (r *RankServer) init_req(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	r.logger.Printf("[INFO] %T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
}

type qchartParam struct {
	rankingType int
	list_rank []int
	event *resource_mgr.EventDetail
	fancyChart bool
}

func (r *RankServer) preload_html(w http.ResponseWriter, req *http.Request, param *qchartParam) {
	rankingType := 0
	fancyChart := false
	var list_rank []int
	var event *resource_mgr.EventDetail
	if param != nil {
		rankingType = param.rankingType
		list_rank = param.list_rank
		event = param.event
		fancyChart = param.fancyChart
	}
	r.init_req(w, req)
	fmt.Fprint(w, "<!DOCTYPE html>\n")
	fmt.Fprint(w, "<head>\n")
	fmt.Fprint(w, `<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="keywords" content="デレステ, イベントランキング, ボーダー, アイマス, アイドルマスターシンデレラガールズスターライトステージ">
<title>デレステボーダーbotβ+</title>`)
	fmt.Fprint(w, `<link rel="stylesheet" type="text/css" href="/static/style.css" />
`)
	fmt.Fprint(w, `<script language="javascript" type="text/javascript" src="/static/jquery-1.12.3.min.js"></script>`)

	if list_rank != nil {
		fmt.Fprint(w, `
<script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
<script type="text/javascript">
`)

		chartType := ""
		if fancyChart {
			chartType = "AnnotationChart"
			fmt.Fprint(w, `google.charts.load('current', {packages: ['corechart', 'annotationchart']});`)
		} else {
			chartType = "LineChart"
			fmt.Fprint(w, `google.charts.load('current', {packages: ['corechart']});`)
		}
		fmt.Fprint(w, `google.charts.setOnLoadCallback(drawLineChart);`)
		fmt.Fprint(w, `google.charts.setOnLoadCallback(orientationChange);
		function orientationChange() {
			$(window).on("orientationchange",drawLineChart);
		};
		`)

		fmt.Fprint(w, `function drawLineChart() {`)
		fmt.Fprint(w, "\nvar data_rank = new google.visualization.DataTable(", r.rankData_list_e(rankingType, list_rank, event), ")")
		fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list_e(rankingType, list_rank, event), ")")
		fmt.Fprintf(w, `
	// first get the size from the window
	// if that didn't work, get it from the body
	var size = {
		width: window.innerWidth || document.body.clientWidth,
		height: window.innerHeight || document.body.clientHeight,
	};
	size_min = Math.min(size.width, size.height)
	var options = {
		width: size.width * 0.9,
		height: size.width * 0.5,
        hAxis: {
            format: 'MM/dd HH:mm',
            gridlines: {count: 12}
        },
        vAxis: {
            //gridlines: {color: 'none'},
            minValue: 0,
			textPosition: 'in',
        },
        interpolateNulls: true,
        explorer: {maxZoomIn: 0.1},
		fontSize: 0.035 * size_min,
		chartArea: {width: '100%%', height: '80%%'},

		legend: {position: 'top', alignment: 'center'},
		//theme: "maximized",
    };
	var options_speed = $.extend({}, options);
	options_speed['interpolateNulls'] = false;
	console.log(options);
	console.log(options_speed);
    var chart = new google.visualization.%s(document.getElementById('myLineChart'));
    var chart_speed = new google.visualization.%s(document.getElementById('mySpeedChart'));
    chart.draw(data_rank, options);
    chart_speed.draw(data_speed, options_speed);
    }`, chartType, chartType)
		fmt.Fprint(w, `</script>`)
	}
	fmt.Fprint(w, "</head>")
	fmt.Fprint(w, `<html lang="ja">`)
	fmt.Fprint(w, "<body>")
}

func (r *RankServer) postload_html(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, "</body>")
	fmt.Fprint(w, "</html>")
}

func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	//fmt.Fprint( w, r.dumpData() )
	req.ParseForm()
	timestamp, ok := req.Form["t"]
	if !ok {
		r.checkData("")
		fmt.Fprint(w, r.latestData())
	} else {
		r.checkData(timestamp[0])
		fmt.Fprint(w, r.showData(timestamp[0]))
	}
}

func (r *RankServer) homeHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, &qchartParam{
		rankingType: 0,
		list_rank: []int{120001},
		event: r.currentEvent,
		fancyChart: false,
	})
	defer r.postload_html(w, req)
	fmt.Fprint(w, `<div id="wrapper">`)
	defer fmt.Fprint(w, `</div`)
	fmt.Fprintf(w, "<h2>デレステイベントボーダーbotβ+</h2>")
	if r.currentEvent != nil {
		fmt.Fprintf(w, "<p>")
		fmt.Fprintf(w, "イベント開催中：%s", r.currentEvent.Name())
		if r.currentEvent.LoginBonusType() > 0 {
			fmt.Fprintf(w, "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。")
		}
		fmt.Fprintf(w, "</p>")
	}
	fmt.Fprintf(w, `<p>twitter bot：十五分毎にイベントptボーダーを更新し、一時間毎にトロフィーと称号ボーダーを更新します。
	<a href="https://twitter/deresuteborder0">@deresuteborder0</a></p>`)

	fmt.Fprintf(w, "<a href=\"event\">%s</a><br>\n", "過去のイベント (new)")
	fmt.Fprintf(w, "<a href=\"log\">%s</a><br>\n", "過去のデータ")
	fmt.Fprint(w, "<hr>")
	fmt.Fprintf(w, "<h3>%s</h3>\n", "12万位ボーダーグラフ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=2001&rank=10001&rank=20001&rank=60001&rank=120001\">%s</a>）<br>\n", "他のボーダーはここ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=501&rank=5001&rank=50001&rank=500001\">%s</a>）<br>\n", "イベント称号ボーダー")
	// insert graph here
	fmt.Fprint(w, `
    <table class="columns">
<tr><td><div id="myLineChart"/></td></tr>
<tr><td>時速</td></tr>
<tr><td><div id="mySpeedChart"/></td></tr>
    </table>
    `)

	fmt.Fprint(w, "<hr>")
	fmt.Fprintf(w, "<h3>%s</h3>\n", "最新ボーダー")
	r.checkData("")
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	fmt.Fprint(w, r.latestData())
}

func (r *RankServer) eventHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprintf(w, `<table class="columns">`)
	fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", "event", "start", "second-half", "end")
	for _, e := range r.resourceMgr.EventList {
		name := e.Name()
		if (e.Type() == 1 || e.Type() == 3) && e.EventEnd().After(time.Unix(1467552720, 0)) {
			// ranking information available
			name = fmt.Sprintf(`<a href="qchart?event=%d">%s</a>`, e.Id(), name)
		}
		fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", name, ts.FormatTime(e.EventStart()), ts.FormatTime(e.SecondHalfStart()), ts.FormatTime(e.EventEnd()))
	}
	fmt.Fprintf(w, `</table>`)
}

func (r *RankServer) logHandler(w http.ResponseWriter, req *http.Request) {
	r.updateTimestamp()
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprintf(w, "<br>デレステイベントボーダー<br><br>")
	fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "最新ボーダー")

	local_timestamp := r.get_list_timestamp()
	for _, timestamp := range local_timestamp {
		fmt.Fprintf(w, "<a href=\"q?t=%s\">%s</a><br>\n", timestamp, ts.FormatTimestamp(timestamp))
	}
}

func (r *RankServer) qchartHandler(w http.ResponseWriter, req *http.Request) {
	r.checkData("")

	// parse parameters
	req.ParseForm()
	list_rank_str, ok := req.Form["rank"]
	var list_rank []int
	if ok {
		list_rank = make([]int, 0, len(list_rank_str))
		for _, v := range list_rank_str {
			// now v can contain more than one number
			//fmt.Println("str<"+v+">")
			subv := strings.Split(v, " ")
			for _, vv := range subv {
				n, err := strconv.Atoi(vv)
				if (err == nil) && (n >= 1) && (n <= 1000001) {
					list_rank = append(list_rank, n)
				}
			}
		}
	} else {
		list_rank = []int{60001, 120001}
	}

	event_id_str_list, ok := req.Form["event"]
	event := r.currentEvent
	var prefill_event string = ""
	// this block output: prefill_event, event
	if ok {
		event_id_str := event_id_str_list[0]
		// skip empty string
		if event_id_str == "" {
			event = r.currentEvent
		} else {
			event_id, err := strconv.Atoi(event_id_str)
			if err == nil {
				prefill_event = event_id_str
				event = r.resourceMgr.FindEventById(event_id)
				if event == nil {
					event = r.currentEvent
				}
			} else {
				r.logger.Println("bad event id", err, event_id_str)
			}
		}
	}
	var prefill string = "2001 10001 20001 60001 120001"
	{
		n_rank := []string{}
		for _, n := range list_rank {
			n_rank = append(n_rank, fmt.Sprintf("%d", n))
		}
		prefill = strings.Join(n_rank, " ")
	}

	var rankingType int
	rankingType_str_list, ok := req.Form["type"]
	if ok {
		rankingType_str := rankingType_str_list[0]
		rankingType_i, err := strconv.Atoi(rankingType_str)
		if err == nil {
			if rankingType_i > 0 {
				rankingType = 1
			}
		} else {
			rankingType = 0
		}
	} else {
		rankingType = 0
	}
	checked_type := []string{"", ""}
	checked_type[rankingType] = " checked"

	fancyChart := false
	fancyChart_checked := ""
	fancyChart_str_list, ok := req.Form["achart"]
	if ok {
		fancyChart_str := fancyChart_str_list[0]
		if len(fancyChart_str) > 0 {
			fancyChart = true
			fancyChart_checked = " checked"
		}
	}

	// generate html
	r.preload_html(w, req, &qchartParam{
		rankingType: rankingType,
		list_rank: list_rank,
		event: event,
		fancyChart: fancyChart,
	})
	defer r.postload_html(w, req)
	fmt.Fprintf(w, "<p><a href=\"..\">%s</a></p>\n", "ホームページ")
	fmt.Fprintf(w, `<div class="form"><p>
<form action="qchart" method="get">customized border graph：<br>
  順位：<input class="t0" type="text" name="rank" size=35 value="%s"></input><br>
  <input type="hidden" name="event" value="%s"></input>
  <input class="r0" type="radio" name="type" value="0"%s>イベントpt</input>
  <input class="r0" type="radio" name="type" value="1"%s>ハイスコア</input><br>
  <input class="c0" type="checkbox" name="achart" value="1"%s>AnnotationChart</input><br>
  <input class="s0" type="submit" value="更新">
</form>
</p></div>`, prefill, prefill_event, checked_type[0], checked_type[1], fancyChart_checked)


	fmt.Fprint(w, `
    <table class="columns">
<tr><td><div id="myLineChart"/></td></tr>
<tr><td>時速</td></tr>
<tr><td><div id="mySpeedChart"/></td></tr>
    </table>
    `)
	fmt.Fprintf(w, `<div class="note"><p>表示できる順位<br>
	イベントpt：%d<br>ハイスコア：%d
	</p></div>`,
		r.get_list_rank(r.latestTimestamp(), 0),
		r.get_list_rank(r.latestTimestamp(), 1))
	fmt.Fprint(w, `<div class="note"><p>javascript library from <code>https://www.gstatic.com/charts/loader.js</code></p></div>`)
}

var staticFilter = regexp.MustCompile("^/static")

func (r *RankServer) staticHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	if !staticFilter.MatchString(req.URL.Path) {
		r.logger.Println("bad req url path", req.URL.Path)
		return
	}
	path := req.URL.Path
	path = strings.Replace(path, "/static", "", 1)
	filename := r.config["staticdir"] + "/" + path

	//r.logger.Println(req.URL, filename, "<"+path+">")
	r.logger.Println("serverfile", filename)
	http.ServeFile(w, req, filename)
}

type twitterParam struct {
	title_suffix string
	list_rank    []int
	map_rank     map[int]string
	rankingType  int
	interval     time.Duration
}

func (r *RankServer) twitterHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "",
		list_rank:    []int{2001, 10001, 20001, 60001, 120001},
		map_rank: map[int]string{
			2001:   "2千位",
			10001:  "1万位",
			20001:  "2万位",
			60001:  "6万位",
			120001: "12万位",
		},
		rankingType: 0,
		interval:    INTERVAL0,
	}
	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterEmblemHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "\n" + "イベント称号ボーダー（時速）",
		list_rank:    []int{501, 5001, 50001, 500001},
		map_rank: map[int]string{
			501:    "5百位",
			5001:   "5千位",
			50001:  "5万位",
			500001: "50万位",
		},
		rankingType: 0,
		interval:    INTERVAL0 * 4,
	}
	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterTrophyHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "\n" + "トロフィーボーダー（時速）",
		//list_rank:    []int{5001, 10001, 50001},
		map_rank: map[int]string{
			5001:  "5千位",
			10001: "1万位",
			40001: "4万位",
			50001: "5万位",
		},
		rankingType: 1,
		interval:    INTERVAL0 * 4,
	}
	if r.currentEvent != nil {
		if r.currentEvent.Type() == 1 {
			param.list_rank = []int{5001, 10001, 40001}
		} else if r.currentEvent.Type() == 3 {
			param.list_rank = []int{5001, 10001, 50001}
		}
	}

	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterHandler_common(w http.ResponseWriter, req *http.Request, param twitterParam) {
	var status string
	r.checkData("")
	timestamp := r.latestTimestamp()
	r.init_req(w, req)
	var title string

	timestamp_str := ts.FormatTimestamp_short(timestamp)

	if r.currentEvent != nil {
		t := ts.TimestampToTime(timestamp)
		// FIXME wait only after 2 hour
		if r.currentEvent.IsCalc(time.Now().Add(-2 * time.Hour)) {
			timestamp_str = "WAITING"
		}
		if r.currentEvent.IsFinal(t) {
			timestamp_str = "【結果発表】"
		}
		title = r.currentEvent.ShortName() + " " + timestamp_str + param.title_suffix + "\n"
	} else {
		r.logger.Println("no event")
		fmt.Fprint(w, "EMPTY")
		return
	}
	status += title
	list_rank := param.list_rank
	map_rank := param.map_rank
	rankingType := param.rankingType
	for _, rank := range list_rank {
		border := r.fetchData(timestamp, rankingType, rank)
		name_rank := map_rank[rank]
		t := ts.TimestampToTime(timestamp)
		t_prev := t.Add(-param.interval)
		timestamp_prev := ts.TimeToTimestamp(t_prev)
		border_prev := r.fetchData(timestamp_prev, rankingType, rank)
		delta := -1
		if border < 0 {
			status += "UPDATING\n"
			break
		}
		if border_prev >= 0 {
			delta = border - border_prev
			status += fmt.Sprintf("%s：%d (+%d)\n", name_rank, border, delta)
		} else {
			status += fmt.Sprintf("%s：%d\n", name_rank, border)
		}
	}

	statusLen := utf8.RuneCountInString(status)
	statusLenFinal := statusLen
	if statusLen > 140 {
		r.logger.Println("[WARN] twitter status limit exceeded", "<"+status+">")
	}
	tail1 := "\n" + "https://" + r.hostname
	tail1Len := 1 + 23 // twitter URL shortener
	tail2 := "\n" + fmt.Sprint("#デレステ")
	tail2Len := utf8.RuneCountInString(tail2)

	if statusLen+tail1Len <= 140 {
		status += tail1
		statusLenFinal += tail1Len
	}
	if statusLen+tail1Len+tail2Len <= 140 {
		status += tail2
		statusLenFinal += tail2Len
	}

	r.logger.Println("[INFO] len/twitter of status", statusLenFinal, "status", "<"+strings.Replace(status, "\n", "<NL>", -1)+">")
	//log.Println("status: <" + status + ">")
	fmt.Fprint(w, status)
	if statusLenFinal > 140 {
		r.logger.Println("[WARN] twitter status limit exceeded", "<"+status+">")
	}
}

func (r *RankServer) res_verHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	fmt.Fprint(w, r.client.Get_res_ver())
}

func (r *RankServer) redirectHandler(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = r.hostname + ":4002"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}

func main() {
	log.Print("RankServer running")
	r := MakeRankServer()
	r.run()
	wg.Wait()
}
