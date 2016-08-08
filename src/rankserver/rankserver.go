package rankserver

import (
	"apiclient"
	"crypto/tls"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
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
	rankDB        string
	db			  *sql.DB
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
	r.db, err = sql.Open("sqlite3", "file:" + r.rankDB + "?mode=ro")
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

// tag: database, sqlite
func (r *RankServer) UpdateTimestamp() {
	rows, err := r.db.Query("SELECT timestamp FROM timestamp")
	if err != nil {
		r.logger.Println("sql error", err)
		return
	}
	defer rows.Close()
	var local_list_timestamp []string
	for rows.Next() {
		var ts string
		err = rows.Scan(&ts)
		if err != nil {
			r.logger.Println("sql error", err)
			return
		}
		local_list_timestamp = append(local_list_timestamp, ts)
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return
	}
	r.mux_timestamp.Lock()
	r.list_timestamp = local_list_timestamp
	sort.Strings(r.list_timestamp)
	r.mux_timestamp.Unlock()
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

// true: nonempty; false: empty
// tag: database, sqlite
func (r *RankServer) checkDir(timestamp string) bool {
	var ts_discard string
	row := r.db.QueryRow("SELECT timestamp FROM rank WHERE timestamp == $1", timestamp)
	err := row.Scan(&ts_discard)
	// FIXME err
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		} else {
			r.logger.Println("sql error", err)
			return false
		}
	} else {
		// row exists
		return true
	}
}

// tag: database, sqlite
func (r *RankServer) CheckData(timestamp string) {
	r.UpdateTimestamp()
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

// tag: database
func (r *RankServer) fetchData(timestamp string, rankingType int, rank int) int {
	var score int
	row := r.db.QueryRow("SELECT score FROM rank WHERE timestamp == $1 AND type == $2 AND rank == $3", timestamp, rankingType + 1, rank)
	err := row.Scan(&score)
	if err != nil {
		if err == sql.ErrNoRows {
			score = -1
		} else {
			if err != nil {
				r.logger.Println("sql error", err)
				score = -1
			}
		}
	}
	//log.Println(timestamp, rankingType, rank, score)
	return score
}

// tag: database
func (r *RankServer) fetchDataListRank(timestamp string, rankingType int) []int {
	var listRank []int
	rows, err := r.db.Query("SELECT rank FROM rank WHERE timestamp == $1 AND type == $2", timestamp, rankingType + 1)
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var rank int
		err = rows.Scan(&rank)
		if err != nil {
			r.logger.Println("sql error", err)
			return nil
		}
		if rank % 10 == 1 {
			listRank = append(listRank, rank)
		}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return listRank
}

// tag: database
func (r *RankServer) fetchDataSlice(timestamp string) []map[int]int {
	slice := make([]map[int]int, 2)
	slice[0] = map[int]int{}
	slice[1] = map[int]int{}

	rows, err := r.db.Query("SELECT type, rank, score FROM rank WHERE timestamp == $1", timestamp)
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var rankingType int
		var rank int
		var score int
		err = rows.Scan(&rankingType, &rank, &score)
		if err != nil {
			r.logger.Println("sql error", err)
			return nil
		}
		rankingType -= 1
		if rank % 10 == 1 {
			slice[rankingType][rank] = score
		}
	}
	err = rows.Err()
	if err != nil {
		r.logger.Println("sql error", err)
		return nil
	}
	return slice
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

func (r *RankServer) latestDataHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, r.latestData())
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

func (r *RankServer) init_req(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	r.logger.Printf("[INFO] %T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
}

type qchartParam struct {
	rankingType int
	list_rank   []int
	event       *resource_mgr.EventDetail
	fancyChart  bool
}

func (r *RankServer) generateDURL(param *qchartParam) string {
	u := "/d?"
	if param == nil {
		return u
	}
	u += "event=" + fmt.Sprintf("%d", param.event.Id()) + "&"
	u += "type=" + fmt.Sprintf("%d", param.rankingType) + "&"
	for _, rank := range param.list_rank {
		u += "rank=" + strconv.Itoa(rank) + "&"
	}
	return u
}

// now the script is totally static
func (r *RankServer) preload_html(w http.ResponseWriter, req *http.Request, param *qchartParam) {
	fancyChart := false
	if param != nil {
		fancyChart = param.fancyChart
	}

	r.init_req(w, req)
	fmt.Fprint(w, "<!DOCTYPE html>\n")
	// related to font bug?
	//fmt.Fprint(w, `<html lang="ja">`)
	fmt.Fprint(w, `<html>`)
	fmt.Fprint(w, "<head>\n")
	fmt.Fprint(w, `<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="keywords" content="デレステ, イベントランキング, ボーダー, アイマス, アイドルマスターシンデレラガールズスターライトステージ">
<title>デレステボーダーbotβ+</title>`, "\n")
	fmt.Fprint(w, `<link rel="stylesheet" type="text/css" href="/static/style.css" />`, "\n")
	fmt.Fprint(w, `<link rel="stylesheet" type="text/css" href="/static/jquery.mobile-1.4.5.min.css" />`, "\n")
	fmt.Fprint(w, `<script type="text/javascript" src="/static/jquery-1.12.3.min.js"></script>`, "\n")
	fmt.Fprint(w, `<script type="text/javascript" src="/static/jquery.mobile-1.4.5.min.js"></script>`, "\n")
	//fmt.Fprintf(w, `<script language="javascript" type="text/javascript" src="%s"></script>`, r.generateDURL(param))

	//if list_rank != nil {
	fmt.Fprint(w, `
<script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
<script type="text/javascript">
`)

	fmt.Fprint(w, `
	currentPage = $("body").pagecontainer("getActivePage");
	function setAspectRatio() {
		aratio = 0.75;
		if (($("#myLineChart", currentPage).length == 0) && ($("#mySpeedChart", currentPage).length == 0)) {
			console.log("shorted setAspectRatio()");
			return;
		}
		myLineChart = $("#myLineChart", currentPage)
		mySpeedChart = $("#mySpeedChart", currentPage)
		console.log("setAspectRatio()", myLineChart.width())
		console.log("setAspectRatio()", myLineChart.height())
		myLineChart.height(myLineChart.width() * aratio);
		mySpeedChart.height(mySpeedChart.width() * aratio);
	}
	$(window).on("pagecreate pageload pagechange pageshow throttledresize", function(e) {
		console.log("pagecreate/throttledresize", e);
		setAspectRatio();
	});
	dataurl = $("#dataurl", currentPage).text();
	console.log("dataurl", dataurl);
	fancychart = $("#fancychart", currentPage).text();
	// this doesn't work
	if (fancychart == 0) {
		//google.charts.load('current', {packages: ['corechart']});
	} else {
		//google.charts.load('current', {packages: ['corechart', 'annotationchart']});
	}
	google.charts.load('current', {packages: ['corechart', 'annotationchart']});
	`)

	fmt.Fprint(w, `google.charts.setOnLoadCallback(drawLineChart);`)

	fmt.Fprint(w, `google.charts.setOnLoadCallback(pageChange);
		function pageChange() {
			console.log("pagechange");
			$(window).on("pagechange", function() {
				drawLineChart()
				console.log("pagechange");
			});
			//$("body").pagecontainer()
			$("body").on("pagecontainerload", function () {
				//drawLineChart();
				console.log("pagecontainerload");
			});
			$("body").on("pageshow", function () {
				//drawLineChart();
				console.log("pageshow");
			});
			$(window).on("orientationchange", function() {
				drawLineChart();
				console.log("orientationchange");
			});
		};`)

	// doesn't work
	//$("#myLineChart").html("");
	//$("#mySpeedChart").html("");
	//fmt.Fprint(w, "\nvar data_rank = new google.visualization.DataTable(", r.rankData_list_e(rankingType, list_rank, event), ");\n")
	//fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list_e(rankingType, list_rank, event), ");\n")

	fmt.Fprint(w, `function updateLatestData() {
		currentPage = $("body").pagecontainer("getActivePage");
		latestdata = $("#latestdata", currentPage);
		if (latestdata.length == 0) {
			return;
		}
		jQuery.get("/latest_data", "", function (data) {
			latestdata.html(data);
		}, "text");
	}`)
	// need printf for legacy reasons %%
	fmt.Fprintf(w, `function drawLineChart() {
	updateLatestData();
	currentPage = $("body").pagecontainer("getActivePage");
	dataurl = $("#dataurl", currentPage).text();
	console.log("dataurl", dataurl);
	fancychart = $("#fancychart", currentPage).text();

	// first get the size from the window
	// if that didn't work, get it from the body
	var size = {
		width: window.innerWidth || document.body.clientWidth,
		height: window.innerHeight || document.body.clientHeight,
	};
	size_min = Math.min(size.width, size.height)
	var options = {
		title: "累計",
		//width: size.width * 1.0,
		//height: size.width * 0.5625,
        hAxis: {
            format: 'MM/dd HH:mm',
            gridlines: {count: 12}
        },
        vAxis: {
            minValue: 0,
			textPosition: 'in',
        },
        interpolateNulls: true,
        explorer: {maxZoomIn: 0.1},
		//fontSize: 0.035 * size_min,
		chartArea: {width: '100%%', height: '65%%'},
		legend: {position: 'top', alignment: 'center'},
    };
	var options_speed = $.extend({}, options);
	options_speed['interpolateNulls'] = false;
	options_speed['title'] = "時速";
	//console.log(options);
	//console.log(options_speed);
	if (($("#myLineChart", currentPage).length == 0) && ($("#mySpeedChart", currentPage).length == 0)) {
		return;
	}
	myLineChart = $("#myLineChart", currentPage)
	mySpeedChart = $("#mySpeedChart", currentPage)
	console.log("drawLineChart, call setAspectRatio()")
	setAspectRatio();
	console.log("drawLineChart, call setAspectRatio() return")
	console.log("drawLineChart,", myLineChart, mySpeedChart)
	var chart
	var chart_speed
	if (fancychart == 0) {
		chart = new google.visualization.LineChart(myLineChart.get(0));
		chart_speed = new google.visualization.LineChart(mySpeedChart.get(0));
	} else {
		chart = eval("new google.visualization.AnnotationChart(myLineChart.get(0))");
		chart_speed = eval("new google.visualization.AnnotationChart(mySpeedChart.get(0))");
	}

	$.getJSON(dataurl, "", function (data) {
		var data_list = [];
		for (t=0; t<2; t++) {
			dt = {"cols": [{"id":"timestamp","label":"timestamp","type":"datetime"}],
			"rows":[]}
			cur = data[t];
			//console.log("r", cur[0]);
			// cols
			for (i=0; i<cur[0].length; i++) {
				dt["cols"].push({"id":cur[0][i], "label":cur[0][i], "type":"number"})
			}
			// rows
			for (i=1; i<cur.length; i++) {
				row = cur[i]
				row_map = {"c":[{"v":new Date(row[0] * 1000)}]}
				for (j=1; j<row.length; j++) {
					row_map["c"].push({"v": row[j]})
				}
				dt["rows"].push(row_map)
			}
			//console.log(dt)
			// t=0: dt: ranklist
			// t=1: dt: speedlist
			data_list[t] = dt;
		}

		var data_rank = new google.visualization.DataTable(data_list[0]);
		var data_speed = new google.visualization.DataTable(data_list[1]);
		console.log("dtl",data_list);
		console.log("draw");
		chart.draw(data_rank, options);
	    chart_speed.draw(data_speed, options_speed);
	})
    }`)
	fmt.Fprint(w, `</script>`)
	//}
	fmt.Fprint(w, "</head>\n")
	fmt.Fprint(w, `<body>`)

	//fmt.Fprint(w, `<div data-role="page">`)
	// doesn't work, data-dom-cache=false is the default
	fmt.Fprint(w, `<div data-role="page" data-dom-cache="false">`)
	// data provided to script
	// the only dynamic part of this function
	fmt.Fprintf(w, `<div id="dataurl" style="display:none;">%s</div>`, r.generateDURL(param))
	fmt.Fprint(w, "\n")
	fancyChart_i := 0
	if fancyChart {
		fancyChart_i = 1
	}
	fmt.Fprintf(w, `<div id="fancychart" style="display:none;">%d</div>`, fancyChart_i)
	fmt.Fprint(w, "\n")
}

func (r *RankServer) postload_html(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, `</div>`)
	fmt.Fprint(w, "</body>")
	fmt.Fprint(w, "</html>")
}

var timestampFilter = regexp.MustCompile("^\\d+$")
func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	//fmt.Fprint( w, r.dumpData() )
	req.ParseForm()
	timestamp, ok := req.Form["t"]  // format checked
	if !ok {
		r.CheckData("")
		fmt.Fprint(w, r.latestData())
	} else {
		if timestampFilter.MatchString(timestamp[0]) {
			r.CheckData(timestamp[0])
			fmt.Fprint(w, r.showData(timestamp[0]))
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
}

func (r *RankServer) homeHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, &qchartParam{
		rankingType: 0,
		list_rank:   []int{120001},
		event:       r.currentEvent,
		fancyChart:  false,
	})
	fmt.Fprint(w, "\n")
	defer r.postload_html(w, req)
	fmt.Fprint(w, `<div id="wrapper">`)
	defer fmt.Fprint(w, `</div>`)
	fmt.Fprintf(w, "<h2>デレステイベントボーダーbotβ+</h2>")
	fmt.Fprint(w, "\n")
	if r.currentEvent != nil {
		fmt.Fprintf(w, "<p>")
		fmt.Fprintf(w, "イベント開催中：%s", r.currentEvent.Name())
		if r.currentEvent.LoginBonusType() > 0 {
			fmt.Fprintf(w, "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。")
		}
		fmt.Fprintf(w, "</p>")
	}
	fmt.Fprintf(w, `<p>twitter bot：十五分毎にイベントptボーダーを更新し、一時間毎にトロフィーと称号ボーダーを更新します。
	<a href="https://twitter.com/deresuteborder0">@deresuteborder0</a></p>`)

	fmt.Fprintf(w, "<a href=\"event\">%s</a><br>\n", "過去のイベント (new)")
	fmt.Fprintf(w, "<a href=\"log\">%s</a><br>\n", "過去のデータ")
	fmt.Fprintf(w, "<a href=\"m\">%s</a><br>\n", "m-test")
	fmt.Fprint(w, "<hr>")
	fmt.Fprintf(w, "<h3>%s</h3>\n", "12万位ボーダーグラフ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=2001&rank=10001&rank=20001&rank=60001&rank=120001\">%s</a>）<br>\n", "他のボーダーはここ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=501&rank=5001&rank=50001&rank=500001\">%s</a>）<br>\n", "イベント称号ボーダー")
	fmt.Fprint(w, r.chartSnippet())

	fmt.Fprint(w, "<hr>")

	r.CheckData("")

	/*
		fmt.Fprintf(w, "<h3>%s</h3>\n", "最新ボーダー")
		fmt.Fprint(w, "<pre>")
		fmt.Fprint(w, r.latestData())
		fmt.Fprint(w, "</pre>")
	*/

	// ajax version
	fmt.Fprintf(w, "<h3>%s</h3>\n", "最新ボーダー")
	fmt.Fprint(w, "<pre id=\"latestdata\">")
	fmt.Fprint(w, "</pre>")
}

func (r *RankServer) chartSnippet() string {
	// insert graph here
	return `
<div class="ui-grid-a ui-responsive">
<div class="ui-block-a" id="myLineChart">loading...</div>
<div class="ui-block-b" id="mySpeedChart">loading...</div>
</div>`
}

// mobile landscape optimized
func (r *RankServer) homeMHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, &qchartParam{
		rankingType: 0,
		list_rank:   []int{120001},
		event:       r.currentEvent,
		fancyChart:  false,
	})
	defer r.postload_html(w, req)
	fmt.Fprintf(w, `<div data-role="page"><div data-role="main" class="ui-content">`)
	defer fmt.Fprintf(w, `</div></div>`)

	fmt.Fprintf(w, "<p><a href=\"..\">%s</a></p>\n", "ホームページ")

	fmt.Fprintf(w, `
<form id="mform" action="#">
  <label for="flip-checkbox-1" class="ui-hidden-accessible">Flip toggle switch checkbox:</label>
  <input type="checkbox" data-role="flipswitch" data-on-text="score" data-off-text="pt" data-wrapper-class="custom-size-flipswitch" name="flip-checkbox-1" id="flip-checkbox-1">
</form>
`)
	fmt.Fprintf(w, `<div>
	<div id="myLineChart">aa</div>
	<div id="mySpeedChart" style="display:none">bb</div></div>`)

	fmt.Fprintf(w, `
<script type="text/javascript">

function setMForm () {
  $("#mform").on("change", function() {
  console.log("changemform");
  var cv = $("#flip-checkbox-1").get(0).checked;
  console.log(cv);
  currentPage = $("body").pagecontainer("getActivePage");
  if (cv) {
	  $("#myLineChart", currentPage).css("display","none");
	  $("#mySpeedChart", currentPage).css("display","block");
  } else {
	  $("#mySpeedChart", currentPage).css("display","none");
	  $("#myLineChart", currentPage).css("display","block");
  }
  });
}

//$("body").on("beforeshow", setMForm);
$("body").on("pagechange", setMForm);
setMForm();

</script>
`)
}

func (r *RankServer) eventHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprintf(w, `<table class="columns">`)
	fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", "event", "start", "second-half", "end")
	formatter := ts.FormatTime
	for _, e := range r.resourceMgr.EventList {
		name := e.Name()
		if (e.Type() == 1 || e.Type() == 3) && e.EventEnd().After(time.Unix(1467552720, 0)) {
			// ranking information available
			name = fmt.Sprintf(`<a href="qchart?event=%d">%s</a>`, e.Id(), name)
		}
		fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", name, formatter(e.EventStart()), formatter(e.SecondHalfStart()), formatter(e.EventEnd()))
	}
	fmt.Fprintf(w, `</table>`)
}

func (r *RankServer) logHandler(w http.ResponseWriter, req *http.Request) {
	r.UpdateTimestamp()
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprintf(w, "<br>デレステイベントボーダー<br><br>")
	fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "最新ボーダー")

	local_timestamp := r.GetListTimestamp()
	for _, timestamp := range local_timestamp {
		fmt.Fprintf(w, "<a href=\"q?t=%s\">%s</a><br>\n", timestamp, ts.FormatTimestamp(timestamp))
	}
}

func (r *RankServer) dataHandler(w http.ResponseWriter, req *http.Request) {
	r.CheckData("")

	// parse parameters
	req.ParseForm()
	list_rank_str, ok := req.Form["rank"] // format checked, split, strconv.Atoi
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

	event_id_str_list, ok := req.Form["event"]  // checked Atoi
	event := r.currentEvent
	// this block output: prefill_event, event
	if ok {
		event_id_str := event_id_str_list[0]
		// skip empty string
		if event_id_str == "" {
			event = r.currentEvent
		} else {
			event_id, err := strconv.Atoi(event_id_str)
			if err == nil {
				event = r.resourceMgr.FindEventById(event_id)
				if event == nil {
					event = r.currentEvent
				}
			} else {
				r.logger.Println("bad event id", err, event_id_str)
			}
		}
	}
	{
		n_rank := []string{}
		for _, n := range list_rank {
			n_rank = append(n_rank, fmt.Sprintf("%d", n))
		}
	}

	var rankingType int
	rankingType_str_list, ok := req.Form["type"] // checked Atoi
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

	// generate json

	fmt.Fprint(w,
		"[\n",
		r.jsonData(rankingType, list_rank, r.fetchData_i, event),
		",\n",
		r.jsonData(rankingType, list_rank, r.getSpeed_i, event),
		"]\n")
}

func (r *RankServer) qchartHandler(w http.ResponseWriter, req *http.Request) {
	r.CheckData("")

	// parse parameters
	req.ParseForm()
	list_rank_str, ok := req.Form["rank"] // format checked split, strconv.Atoi
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

	event_id_str_list, ok := req.Form["event"]  // checked Atoi
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
	rankingType_str_list, ok := req.Form["type"] // checked Atoi
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
	fancyChart_str_list, ok := req.Form["achart"] // ignored, len
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
		list_rank:   list_rank,
		event:       event,
		fancyChart:  fancyChart,
	})
	defer r.postload_html(w, req)
	fmt.Fprintf(w, "<p><a href=\"..\">%s</a></p>\n", "ホームページ")
	fmt.Fprintf(w, `<div class="form">
<form action="qchart" method="get">
  customized border graph：<br>

  <label for="textinput-rank">順位：</label>
  <input class="t0" id="textinput-rank" type="text" name="rank" size=35 value="%s">

  <input type="hidden" name="event" value="%s">

  <label for="radio-pt">イベントpt</label>
  <input class="r0" id="radio-pt" type="radio" name="type" value="0"%s>

  <label for="radio-score">ハイスコア</label>
  <input class="r0" id="radio-score" type="radio" name="type" value="1"%s>

  <label for="checkbox-achart">AnnotationChart</label>
  <input class="c0" id="checkbox-achart" type="checkbox" name="achart" value="1"%s>

  <input class="s0" type="submit" value="更新">
</form>
</div>`, prefill, prefill_event, checked_type[0], checked_type[1], fancyChart_checked)

	fmt.Fprint(w, r.chartSnippet())
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
	filename := r.config["staticdir"] + path

	//r.logger.Println(req.URL, filename, "<"+path+">")
	r.logger.Println("[INFO] servefile", filename)
	http.ServeFile(w, req, filename)
}

type twitterParam struct {
	title_suffix string
	title_speed  string
	list_rank    []int
	map_rank     map[int]string
	rankingType  int
	interval     time.Duration
}

func (r *RankServer) twitterHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "",
		title_speed: "",
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
		title_suffix: "\n" + "イベント称号ボーダー",
		title_speed: "（時速）",
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
		title_suffix: "\n" + "トロフィーボーダー",
		title_speed: "（時速）",
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
	r.CheckData("")
	timestamp := r.latestTimestamp()
	r.init_req(w, req)
	var title string

	timestamp_str := ts.FormatTimestamp_short(timestamp)
	var isFinal = false

	if r.currentEvent != nil {
		t := ts.TimestampToTime(timestamp)
		// FIXME wait only after 2 hour
		if r.currentEvent.IsCalc(time.Now().Add(-2 * time.Hour)) {
			timestamp_str = "WAITING"
		}
		if r.currentEvent.IsFinal(t) {
			timestamp_str = "【結果発表】"
			isFinal = true
		}
		title = r.currentEvent.ShortName() + " " + timestamp_str + param.title_suffix + param.title_speed + "\n"
		if isFinal {
			// remove param.title_speed
			title = r.currentEvent.ShortName() + " " + timestamp_str + param.title_suffix + "\n"
		}
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
		if isFinal {
			tsList := r.GetListTimestamp()
			// increasing order
			// timestamp_prev will be the final before EventEnd
			for _, ts := range tsList {
				if r.inEventActive(ts, r.currentEvent) {
					timestamp_prev = ts
				}
			}
		}
		if isFinal {
			r.logger.Println("isFinal debug", "timestamp", timestamp)
			r.logger.Println("isFinal debug", "timestamp_prev", timestamp_prev)
		}

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

func Main() {
	log.Print("RankServer running")
	r := MakeRankServer()
	r.run()
	wg.Wait()
}
