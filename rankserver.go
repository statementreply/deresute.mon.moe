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
	data  map[string][]map[int]int     // need mux
	speed map[string][]map[int]float32 // need mux
	// {"1467555420":   [{10: 2034} ,{30: 203021} ]  }
	list_timestamp []string // need mutex?
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
	tz            *time.Location
	resourceMgr   *resource_mgr.ResourceMgr
	currentEvent  *resource_mgr.EventDetail
	client        *apiclient.ApiClient
	lastCheck     time.Time
}

func MakeRankServer() *RankServer {
	r := &RankServer{}
	r.data = make(map[string][]map[int]int)
	r.speed = make(map[string][]map[int]float32)
	//r.list_timestamp doesn't need initialization
	r.plainServer = nil
	r.tlsServer = nil

	tz, err := time.LoadLocation("Asia/Tokyo")
	r.tz = tz
	if err != nil {
		log.Fatalln("load timezone", err)
	}

	content, err := ioutil.ReadFile(CONFIG_FILE)
	if err != nil {
		log.Fatalln("read config file", err)
	}
	var config map[string]string
	yaml.Unmarshal(content, &config)
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
	http.HandleFunc("/chart", r.chartHandler)
	http.HandleFunc("/qchart", r.qchartHandler)
	http.HandleFunc("/twitter", r.twitterHandler)
	http.HandleFunc("/twitter_emblem", r.twitterEmblemHandler)
}

func (r *RankServer) updateTimestamp() {
	dir, err := os.Open(RANK_CACHE_DIR)
	if err != nil {
		// FIXME
		r.logger.Println(err)
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
	// check new res_ver
	if time.Now().Sub(r.lastCheck) >= 6*time.Hour {
		r.client.LoadCheck()
		rv := r.client.Get_res_ver()
		r.resourceMgr.Set_res_ver(rv)
		r.resourceMgr.ParseEvent()
		r.currentEvent = r.resourceMgr.FindCurrentEvent()
		r.lastCheck = time.Now()
	}

	r.updateTimestamp()
	latest := r.latestTimestamp()
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
	//log.Print(subdir)
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

func (r *RankServer) timestampToTime(timestamp string) time.Time {
	itime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		r.logger.Println("timestamp format incorrect?", err)
		itime = 0
	}
	t := time.Unix(itime, 0).In(r.tz)
	return t
}

func (r *RankServer) timeToTimestamp(t time.Time) string {
	itime := t.Unix()
	timestamp := fmt.Sprintf("%d", itime)
	return timestamp
}

func (r *RankServer) formatTimestamp(timestamp string) string {
	t := r.timestampToTime(timestamp)
	st := t.Format(time.RFC3339)
	return st
}

func (r *RankServer) formatTimestamp_short(timestamp string) string {
	t := r.timestampToTime(timestamp)
	st := t.Format("01/02 15:04")
	return st
}

func (r *RankServer) formatTime(t time.Time) string {
	st := t.Format("2006-01-02 15:04")
	return st
}

func (r *RankServer) inEvent(timestamp string, event *resource_mgr.EventDetail) bool {
	if event == nil {
		return true
	}
	t := r.timestampToTime(timestamp)
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
		// -1 for missing data
		r.logger.Println(err, "return -1")
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
	/*if score == 0 {
		r.logger.Println(timestamp, fileName, len(local_rank_list), "return 0", content)
	}*/
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
	t_i := r.timestampToTime(timestamp)
	t_prev := t_i.Add(-INTERVAL)
	prev_timestamp := r.timeToTimestamp(t_prev)

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
	//fmt.Println("here+1")
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
	r.mux.RUnlock()
	if !ok {
		return ""
	}
	yy, _ := yaml.Marshal(item)
	st := r.formatTimestamp(timestamp)
	return timestamp + "\n" + st + "\n" + string(yy)
}

// js map
// {"cols":
//   [{"id":"timestamp","label":"timestamp","type":"date"},{"id":"score","label":"score","type":"number"}],
//  "rows":[{"c":[{"v":"new Date(1467770520)"},{"v":14908}]}]}

func (r *RankServer) rankData_list_f(rankingType int, list_rank []int, dataSource func(string, int, int) interface{}) string {
	return r.rankData_list_f_e(rankingType, list_rank, dataSource, r.currentEvent)
}

func (r *RankServer) get_list_timestamp() []string {
	r.mux_timestamp.RLock()
	local_timestamp := make([]string, len(r.list_timestamp))
	copy(local_timestamp, r.list_timestamp)
	r.mux_timestamp.RUnlock()
	return local_timestamp
}

func (r *RankServer) rankData_list_f_e(rankingType int, list_rank []int, dataSource func(string, int, int) interface{}, event *resource_mgr.EventDetail) string {
	//log.Print("functional version of rankData_list_f()")
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
			//log.Print("timestamp ", timestamp, " score ", score)
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

func (r *RankServer) rankData_list(rankingType int, list_rank []int) string {
	return r.rankData_list_f(rankingType, list_rank, r.fetchData_i)
}

func (r *RankServer) rankData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.fetchData_i, event)
}

func (r *RankServer) speedData_list(rankingType int, list_rank []int) string {
	return r.rankData_list_f(rankingType, list_rank, r.getSpeed_i)
}

func (r *RankServer) speedData_list_e(rankingType int, list_rank []int, event *resource_mgr.EventDetail) string {
	return r.rankData_list_f_e(rankingType, list_rank, r.getSpeed_i, event)
}

func (r *RankServer) init_req(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	r.logger.Printf("%T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
}

func (r *RankServer) preload(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	fmt.Fprint(w, "<!DOCTYPE html>")
	fmt.Fprint(w, "<html>")
	fmt.Fprint(w, "<body>")
}

func (r *RankServer) preload_c(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	fmt.Fprint(w, "<!DOCTYPE html>")
	fmt.Fprint(w, "<head>")
	fmt.Fprint(w, `
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
      google.charts.load('current', {packages: ['corechart', 'annotationchart']});
      google.charts.setOnLoadCallback(drawLineChart);
      `)
	fmt.Fprint(w, `
    function drawLineChart() {
      // Define the chart to be drawn.
      //var data = new google.visualization.DataTable();`)
	fmt.Fprint(w, "\nvar data_r = new google.visualization.DataTable(", r.rankData_list(0, []int{2001, 10001, 20001, 60001, 120001, 300001}), ")")
	fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list(0, []int{2001, 10001, 20001, 60001, 120001, 300001}), ")")
	fmt.Fprint(w, "\nvar data_speed_12 = new google.visualization.DataTable(", r.speedData_list(0, []int{60001, 120001}), ")")
	fmt.Fprint(w, "\nvar data_speed_2 = new google.visualization.DataTable(", r.speedData_list(0, []int{2001, 10001, 20001}), ")")
	fmt.Fprint(w, `
      var options = {
        //title: 'Rate the Day on a Scale of 1 to 10',
        width: 900,
        height: 500,
        hAxis: {
            format: 'MM/dd HH:mm',
            gridlines: {count: 12}
        },
        vAxis: {
            //gridlines: {color: 'none'},
            minValue: 0
        },
        interpolateNulls: true,
        explorer: {},
    };
    var options_a = {width: 900, height: 500,};
    var options_speed = {width: 900, height: 500,title: '時速'};
    // Instantiate and draw the chart.
    var chart = new google.visualization.LineChart(document.getElementById('myLineChart'));
    var chart_a = new google.visualization.AnnotationChart(document.getElementById('myAnnotationChart'));
    var chart_speed = new google.visualization.AnnotationChart(document.getElementById('mySpeedChart'));
    var chart_speed_12 = new google.visualization.LineChart(document.getElementById('mySpeedChart12'));
    var chart_speed_2 = new google.visualization.LineChart(document.getElementById('mySpeedChart2'));
    chart.draw(data_r, options);
    chart_a.draw(data_r, options_a);
    chart_speed.draw(data_speed, options_speed);
    chart_speed_12.draw(data_speed_12, options);
    chart_speed_2.draw(data_speed_2, options);
    }
    `)
	fmt.Fprint(w, `</script>`)
	fmt.Fprint(w, "</head>")
	fmt.Fprint(w, "<html>")
	fmt.Fprint(w, "<body>")
}

func (r *RankServer) preload_qchart(w http.ResponseWriter, req *http.Request, list_rank []int, event *resource_mgr.EventDetail) {
	r.init_req(w, req)
	fmt.Fprint(w, "<!DOCTYPE html>")
	fmt.Fprint(w, "<head>")
	fmt.Fprint(w, `
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
      google.charts.load('current', {packages: ['corechart']});
      google.charts.setOnLoadCallback(drawLineChart);
      `)
	fmt.Fprint(w, `
    function drawLineChart() {`)
	fmt.Fprint(w, "\nvar data_rank = new google.visualization.DataTable(", r.rankData_list_e(0, list_rank, event), ")")
	fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list_e(0, list_rank, event), ")")
	fmt.Fprint(w, `
      var options = {
        width: 900,
        height: 500,
        hAxis: {
            format: 'MM/dd HH:mm',
            gridlines: {count: 12}
        },
        vAxis: {
            //gridlines: {color: 'none'},
            minValue: 0
        },
        interpolateNulls: true,
        explorer: {maxZoomIn: 0.1},
    };
      var options_speed = {
        width: 900,
        height: 500,
        hAxis: {
            format: 'MM/dd HH:mm',
            gridlines: {count: 12}
        },
        vAxis: {
            //gridlines: {color: 'none'},
            minValue: 0
        },
        interpolateNulls: false,
        explorer: {maxZoomIn: 0.1},
    };

    var chart = new google.visualization.LineChart(document.getElementById('myLineChart'));
    var chart_speed = new google.visualization.LineChart(document.getElementById('mySpeedChart'));
    chart.draw(data_rank, options);
    chart_speed.draw(data_speed, options_speed);
    }
    `)
	fmt.Fprint(w, `</script>`)
	fmt.Fprint(w, "</head>")
	fmt.Fprint(w, "<html>")
	fmt.Fprint(w, "<body>")
}

func (r *RankServer) postload(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, "</body>")
	fmt.Fprint(w, "</html>")
}

func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
	r.preload(w, req)
	defer r.postload(w, req)
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
	r.preload_qchart(w, req, []int{120001}, r.currentEvent)
	defer r.postload(w, req)
	fmt.Fprintf(w, "<br>デレステイベントボーダーbotβ+<br><br>")
	if r.currentEvent != nil {
		fmt.Fprintf(w, "イベント開催中：%s<br>", r.currentEvent.Name())
		if r.currentEvent.LoginBonusType() > 0 {
			fmt.Fprintf(w, "ログインボーナスがあるので、イベントページにアクセスを忘れないように。<br>")
		}
	}

	fmt.Fprintf(w, "<a href=\"event\">%s</a><br>\n", "過去のイベント (new)")
	fmt.Fprintf(w, "<a href=\"log\">%s</a><br>\n", "過去のデータ")
	//fmt.Fprintf(w, "<a href=\"chart\">%s</a><br>\n", "グラフβ")
	fmt.Fprintf(w, "%s\n", "12万位ボーダーグラフ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=2001&rank=10001&rank=20001&rank=60001&rank=120001\">%s</a>）<br>\n", "他のボーダーはここ")
	fmt.Fprintf(w, "（<a href=\"qchart?rank=501&rank=5001&rank=50001&rank=500001\">%s</a>）<br>\n", "イベント称号ボーダー")
	// insert graph here
	fmt.Fprint(w, `
    <table class="columns">
<tr><td><div id="myLineChart" style="border: 1px solid #ccc"/></td></tr>
<tr><td>時速</td></tr>
<tr><td><div id="mySpeedChart" style="border: 1px solid #ccc"/></td></tr>
    </table>
    `)

	fmt.Fprintf(w, "<br>%s<br>\n", "最新ボーダー")
	r.checkData("")
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	fmt.Fprint(w, r.latestData())
}

func (r *RankServer) eventHandler(w http.ResponseWriter, req *http.Request) {
	r.preload(w, req)
	defer r.postload(w, req)
	fmt.Fprintf(w, `<table class="columns">`)
	fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", "event", "start", "second-half", "end")
	for _, e := range r.resourceMgr.EventList {
		name := e.Name()
		if (e.Type() == 1 || e.Type() == 3) && e.EventEnd().After(time.Unix(1467552720, 0)) {
			// ranking information available
			name = fmt.Sprintf(`<a href="qchart?event=%d">%s</a>`, e.Id(), name)
		}
		fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", name, r.formatTime(e.EventStart()), r.formatTime(e.SecondHalfStart()), r.formatTime(e.EventEnd()))
	}
	fmt.Fprintf(w, `</table>`)
}

func (r *RankServer) logHandler(w http.ResponseWriter, req *http.Request) {
	r.updateTimestamp()
	r.preload(w, req)
	defer r.postload(w, req)
	fmt.Fprintf(w, "<br>デレステイベントボーダー<br><br>")
	fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "最新ボーダー")

	local_timestamp := r.get_list_timestamp()
	for _, timestamp := range local_timestamp {
		fmt.Fprintf(w, "<a href=\"q?t=%s\">%s</a><br>\n", timestamp, r.formatTimestamp(timestamp))
	}
}

func (r *RankServer) chartHandler(w http.ResponseWriter, req *http.Request) {
	r.checkData("")
	r.preload_c(w, req)
	defer r.postload(w, req)
	fmt.Fprint(w, `
    uses javascript library from <code>https://www.gstatic.com/charts/loader.js</code><br>`)
	fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "ホームページ")
	fmt.Fprint(w, `
<!-- Identify where the chart should be drawn. -->
    <table class="columns">
<tr><td><div id="myLineChart" style="border: 1px solid #ccc"/></td></tr>
<tr><td><div id="myAnnotationChart" /></td></tr>
<tr><td>時速<div id="mySpeedChart12" style="border: 1px solid #ccc"/></td></tr>
<tr><td><div id="mySpeedChart2" style="border: 1px solid #ccc"/></td></tr>
<tr><td><div id="mySpeedChart"/></td></tr>
    </table>
    `)
}

func (r *RankServer) qchartHandler(w http.ResponseWriter, req *http.Request) {
	r.checkData("")
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
	if ok {
		event_id_str := event_id_str_list[0]
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
	var prefill string = "2001 10001 20001 60001 120001"
	{
		n_rank := []string{}
		for _, n := range list_rank {
			n_rank = append(n_rank, fmt.Sprintf("%d", n))
		}
		prefill = strings.Join(n_rank, " ")
	}
	r.preload_qchart(w, req, list_rank, event)
	defer r.postload(w, req)
	fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "ホームページ")
	fmt.Fprintf(w, `
<form action="qchart" method="get">customized border graph：<br>
<input type="text" name="rank" size=35 value="%s"></input>
<input type="submit" value="更新"></form>
	`, prefill)
	fmt.Fprint(w, `
    <table class="columns">
<tr><td><div id="myLineChart" style="border: 1px solid #ccc"/></td></tr>
<tr><td>時速</td></tr>
<tr><td><div id="mySpeedChart" style="border: 1px solid #ccc"/></td></tr>
    </table>
    `)
	fmt.Fprint(w, `javascript library from <code>https://www.gstatic.com/charts/loader.js</code><br>`)
}

func (r *RankServer) twitterHandler(w http.ResponseWriter, req *http.Request) {
	r.checkData("")
	timestamp := r.latestTimestamp()
	r.init_req(w, req)
	title := "デレステボーダーbotβ"
	if r.currentEvent != nil {
		title = r.currentEvent.Name()
	}
	fmt.Fprint(w, title, " ", r.formatTimestamp_short(timestamp), "\n")
	list_rank := []int{2001, 10001, 20001, 60001, 120001}
	map_rank := map[int]string{
		2001:   "2千位",
		10001:  "1万位",
		20001:  "2万位",
		60001:  "6万位",
		120001: "12万位",
	}
	rankingType := 0
	for _, rank := range list_rank {
		border := r.fetchData(timestamp, rankingType, rank)
		name_rank := map_rank[rank]
		t := r.timestampToTime(timestamp)
		t_prev := t.Add(-INTERVAL0)
		timestamp_prev := r.timeToTimestamp(t_prev)
		border_prev := r.fetchData(timestamp_prev, rankingType, rank)
		delta := -1
		if border < 0 {
			fmt.Fprintf(w, "UPDATING\n")
			break
		}
		if border_prev >= 0 {
			delta = border - border_prev
			fmt.Fprintf(w, "%s: %d (+%d)\n", name_rank, border, delta)
		} else {
			fmt.Fprintf(w, "%s: %d\n", name_rank, border)
		}
	}
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, "https://"+r.hostname+"\n")
	fmt.Fprint(w, "#デレステ\n")
}

func (r *RankServer) twitterEmblemHandler(w http.ResponseWriter, req *http.Request) {
	r.checkData("")
	timestamp := r.latestTimestamp()
	r.init_req(w, req)
	title := "デレステボーダーbotβ"
	if r.currentEvent != nil {
		title = r.currentEvent.Name() + "イベント称号ボーダー（時速）"
	} else {
		return
	}
	fmt.Fprint(w, title, " ", r.formatTimestamp_short(timestamp), "\n")
	list_rank := []int{501, 5001, 50001, 500001}
	map_rank := map[int]string{
		501:   "5百位",
		5001:  "5千位",
		50001:  "5万位",
		500001:  "50万位",
	}
	rankingType := 0
	for _, rank := range list_rank {
		border := r.fetchData(timestamp, rankingType, rank)
		name_rank := map_rank[rank]
		t := r.timestampToTime(timestamp)
		t_prev := t.Add(-INTERVAL0 * 4)
		timestamp_prev := r.timeToTimestamp(t_prev)
		border_prev := r.fetchData(timestamp_prev, rankingType, rank)
		delta := -1
		if border < 0 {
			fmt.Fprintf(w, "UPDATING\n")
			break
		}
		if border_prev >= 0 {
			delta = border - border_prev
			fmt.Fprintf(w, "%s: %d (+%d)\n", name_rank, border, delta)
		} else {
			fmt.Fprintf(w, "%s: %d\n", name_rank, border)
		}
	}
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, "https://"+r.hostname+"\n")
	fmt.Fprint(w, "#デレステ\n")
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
