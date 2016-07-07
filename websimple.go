package main
import (
    "fmt"
    "net/http"
    "crypto/tls"
    "os"
    "path"
    "log"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "regexp"
    "sort"
    "time"
    "strconv"
    "encoding/json"
    "sync"
)


var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
// 15min update interval
// *4 for hour
var INTERVAL int = 15 * 60 * 4
var LOG_FILE = "rankserver.log"
var CONFIG_FILE = "rankserver.yaml"

type RankServer struct {
    //data map[string]map[string]string
    data map[string][]map[int]int
    speed map[string][]map[int]float32
    //data_cache map[string][]map[int]bool
    // {"1467555420": 
    //    [{10: 2034} ,{30: 203021} ]
    //  }
    list_timestamp []string
    // lock for write ops
    mux sync.Mutex
    logger *log.Logger
    keyFile string
    certFile string
    plainServer *http.Server
    tlsServer *http.Server
    hostname string
}

func MakeRankServer() *RankServer {
    r := &RankServer{}
    r.data = make(map[string][]map[int]int)
    r.speed = make(map[string][]map[int]float32)
    //r.data_cache = make(map[string][]map[int]bool)
    //r.list_timestamp doesn't need initialization
    r.plainServer = nil
    r.tlsServer = nil

    content, err := ioutil.ReadFile(CONFIG_FILE)
    if err != nil {
        log.Fatal(err)
    }
    var config map[string]string
    yaml.Unmarshal(content, &config)
    fmt.Println(config)
    confLOG_FILE, ok := config["LOG_FILE"]
    if ok {
        LOG_FILE = confLOG_FILE
    }
    log.Print("logfile is ", LOG_FILE)
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
        log.Fatal("no hostname in config")
    }
    fh, err := os.OpenFile(LOG_FILE, os.O_RDWR | os.O_APPEND | os.O_CREATE, 0644)
    if err != nil {
        log.Fatal("cant open log file")
    }
    r.logger = log.New(fh, "", log.LstdFlags)

    //r.plainServer = &http.Server{ Addr: ":4001", }
    if (r.keyFile != "") && (r.certFile != "") {
        r.logger.Print("use https TLS")
        r.logger.Print("keyFile " + r.keyFile + " certFile " + r.certFile)
        //http.ListenAndServeTLS(":4002", r.certFile, r.keyFile, nil)
        cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
        if err != nil {
            r.logger.Fatal(err)
        }
        r.tlsServer = &http.Server{
            Addr: ":4002",
            TLSConfig: &tls.Config{ Certificates: []tls.Certificate{cert} },
        }
        r.plainServer = &http.Server{Addr: ":4001", Handler: http.NewServeMux()}
        r.plainServer.Handler.(*http.ServeMux).HandleFunc("/", r.redirectHandler)
    } else {
        log.Print("use http plaintext")
        //http.ListenAndServe(":4001", nil)
        r.plainServer = &http.Server{ Addr: ":4001" }
    }
    r.setHandleFunc()
    return r
}

func (r *RankServer) setHandleFunc() {
    //var defaultServer *http.Server
    if (r.tlsServer != nil) {
        //defaultServer = r.tlsServer
        // register 302
    } else {
        //defaultServer = r.plainServer
    }
    // for DefaultServMux
    http.HandleFunc("/", r.homeHandler)
    http.HandleFunc("/q", r.qHandler)
    http.HandleFunc("/log", r.logHandler)
    http.HandleFunc("/chart", r.chartHandler)
    http.HandleFunc("/qchart", r.qchartHandler)
}

func (r *RankServer) updateTimestamp() {
    dir, err := os.Open(RANK_CACHE_DIR)
    defer dir.Close()
    if err != nil {
        log.Fatal(err)
    }

    fi, _ := dir.Readdir(0)
    r.mux.Lock()
    r.list_timestamp = make([]string, 0, len(fi))
    // sub: dir name 1467555420
    for _, sub := range fi {
        if sub.IsDir() {
            r.list_timestamp = append(r.list_timestamp, sub.Name())
        }
    }
    sort.Strings(r.list_timestamp)
    r.mux.Unlock()
}

func (r *RankServer) latestTimestamp() string {
    r.updateTimestamp()
    latest := r.list_timestamp[len(r.list_timestamp)-1]
    return latest
}

func (r *RankServer) checkData(timestamp string) {
    r.updateTimestamp()
    latest := r.latestTimestamp()
    if timestamp == "" {
        timestamp = latest
    }
    //r.data[timestamp] = make([]map[int]int, 2)
    //r.data[timestamp][0] = make(map[int]int)
    //r.data[timestamp][1] = make(map[int]int)
    subdirPath := RANK_CACHE_DIR + timestamp + "/"

    subdir, _ := os.Open(subdirPath)
    //log.Print(subdir)
    key, _ := subdir.Readdir(0)
    for _, pt := range key {
        rankingType := r.RankingType(pt.Name())
        //fileName := subdirPath + pt.Name()
        //log.Print(fileName)
        //content, _ := ioutil.ReadFile(fileName)

        rank := r.FilenameToRank(pt.Name())
        r.fetchData(timestamp, rankingType, rank)
    }
}

func (r *RankServer) getFilename(timestamp string, rankingType, rank int) string {
    subdirPath := RANK_CACHE_DIR + timestamp + "/"
    a := rankingType + 1
    b := int((rank - 1) / 10) + 1
    fileName := subdirPath + fmt.Sprintf("r%02d.%06d", a, b)

    return fileName
}

func (r *RankServer) fetchData(timestamp string, rankingType int, rank int) int {
    fileName := r.getFilename(timestamp, rankingType, rank)
    return r.fetchData_internal(timestamp, rankingType, rank, fileName)
}

func (r *RankServer) fetchData_internal(timestamp string, rankingType int, rank int, fileName string) int {
    _, ok := r.data[timestamp]
    //_, ok2 := r.data_cache[timestamp]
    if ! ok {
        r.mux.Lock()
        r.data[timestamp] = make([]map[int]int, 2)
        r.data[timestamp][0] = make(map[int]int)
        r.data[timestamp][1] = make(map[int]int)
        r.mux.Unlock()
        //r.data_cache[timestamp] = make([]map[int]bool, 2)
        //r.data_cache[timestamp][0] = make(map[int]bool)
        //r.data_cache[timestamp][1] = make(map[int]bool)
    } else {
        //log.Print(timestamp, "x", rankingType, "x", rank, r.data_cache)
        score, ok := r.data[timestamp][rankingType][rank]
        //os.Exit(1)
        if ok {
            // do nothing
            return score
        }
    }

    //log.Print(fileName)
    content, err := ioutil.ReadFile(fileName)
    if err != nil {
        // file doesn't exist?
        return -1
    }

    var local_rank_list []map[string]interface{}
    yaml.Unmarshal(content, &local_rank_list)

    if len(local_rank_list) > 0 {
        rank := local_rank_list[0]["rank"].(int)
        score := local_rank_list[0]["score"].(int)
        r.mux.Lock()
        r.data[timestamp][rankingType][rank] = score
        r.mux.Unlock()
    } else {
        r.mux.Lock()
        //rank := r.FilenameToRank(fileName)
        r.data[timestamp][rankingType][rank] = 0
        r.mux.Unlock()
    }
    //}
    //r.data_cache[timestamp][rankingType][rank] = true
    return r.data[timestamp][rankingType][rank]
}

// speed per hour
func (r *RankServer) getSpeed(timestamp string, rankingType int, rank int) float32 {
    _, ok := r.speed[timestamp]
    if ! ok {
        r.mux.Lock()
        r.speed[timestamp] = make([]map[int]float32, 2)
        r.speed[timestamp][0] = make(map[int]float32)
        r.speed[timestamp][1] = make(map[int]float32)
        r.mux.Unlock()
    } else {
        val, ok := r.speed[timestamp][rankingType][rank]
        if ok {
            return val
        }
    }
    timestamp_i, _ := strconv.Atoi(timestamp)
    prev_timestamp := fmt.Sprintf("%d", timestamp_i - INTERVAL)
    cur_score := r.fetchData(timestamp, rankingType, rank)
    prev_score := r.fetchData(prev_timestamp, rankingType, rank)
    if (cur_score >= 0) && (prev_score >= 0) {
        r.mux.Lock()
        r.speed[timestamp][rankingType][rank] = (float32(cur_score - prev_score)) / float32(INTERVAL) * 3600.0;
        r.mux.Unlock()
        return r.speed[timestamp][rankingType][rank]
    } else {
        // one of them is missing data
        return -1.0
    }
}

func (r *RankServer) getSpeed_i(timestamp string, rankingType int, rank int) interface{} {
    var x interface{}
    x = r.getSpeed(timestamp, rankingType, rank)
    return x
}

func (r *RankServer) RankingType(fileName string) int {
    filter, _ := regexp.Compile("r01\\.\\d+$")
    if filter.MatchString(fileName) {
        // event pt
        return 0 // r01.xxxxxx
    } else {
        // high score
        return 1 // r02.xxxxxx
    }
}

func (r *RankServer) FilenameToRank(fileName string) int {
    //log.Print("fileName", fileName)
    filter, _ := regexp.Compile("r\\d{2}\\.(\\d+)$")
    submatch := filter.FindStringSubmatch(fileName)
    n, _ := strconv.Atoi(submatch[1])
    //log.Print("fileName", fileName, "n", n, "submatch", submatch)
    return (n - 1) * 10 + 1
}




func (r *RankServer) run() {
    if r.tlsServer != nil {
        fmt.Println("here-1")
        go r.tlsServer.ListenAndServeTLS(r.certFile, r.keyFile)
        fmt.Println("here")
    }
    fmt.Println("here+1")
    r.plainServer.ListenAndServe()
    fmt.Println("here+1")
}

func (r *RankServer) dumpData() string {
    yy, _ := yaml.Marshal(r.data)
    return string(yy)
}


func (r *RankServer) latestData() string {
    timestamp := r.latestTimestamp()
    return r.showData(timestamp)
}

func (r *RankServer) showData(timestamp string) string {
    item, ok := r.data[timestamp]
    if ! ok {
        return ""
    }
    yy, _ := yaml.Marshal(item)
    st := r.formatTimestamp(timestamp)
    return timestamp + "\n" + st + "\n" + string(yy)
}

func (r *RankServer) jsonData(timestamp string) string {
    //log.Print("jsonData", timestamp)
    item, ok := r.data[timestamp]
    if ! ok {
        return ""
    }
    //log.Print("jsonData", item)
    s_item := make([]map[string]int, 2)
    j_item := make([][]map[string][]map[string]int, 2)
    for ind, sub := range item {
        // ind = 0, 1
        s_item[ind] = make(map[string]int)
        j_item[ind] = make([]map[string][]map[string]int, 0, len(sub))
        keys := make([]int, 0, len(sub))
        // need sort according to k
        for k, v := range sub {
            s_item[ind][strconv.Itoa(k)] = v
            keys = append(keys, k)
        }
        sort.Ints(keys)
        for _, k := range keys {
            v := sub[k]
            vv := make(map[string][]map[string]int)
            vv["c"] = make([]map[string]int, 2)
            vv["c"][0] = make(map[string]int)
            vv["c"][1] = make(map[string]int)
            vv["c"][0]["v"] = k
            vv["c"][1]["v"] = v
            j_item[ind] = append(j_item[ind], vv)
        }
    }
    j_data_col := make([]interface{}, 2)
    j_data_col[0] = map[string]string{"id": "rank", "label": "rank", "type": "number"}
    j_data_col[1] = map[string]string{"id": "score", "label": "score", "type": "number"}
    j_data := map[string]interface{}{"cols": j_data_col, "rows": j_item[0]}
    // type 1
    text, err := json.Marshal(j_data)
    if err != nil {
        log.Fatal(err)
    }
    //log.Print("jsonData", string(text))
    return string(text)
}

    // {"cols":[{"id":"timestamp","label":"timestamp","type":"date"},{"id":"score","label":"score","type":"number"}],"rows":[{"c":[{"v":"new Date(1467770520)"},{"v":14908}]}]}
func (r *RankServer) rankData(rankingType int, rank int) string {
    r.updateTimestamp()
    raw := ""
    raw += `{"cols":[{"id":"timestamp","label":"timestamp","type":"datetime"},{"id":"score","label":"120001","type":"number"}],"rows":[`
    j_item := make([]map[string][]map[string]interface{}, 0, len(r.list_timestamp))
    j_data_col := make([]interface{}, 2)
    j_data_col[0] = map[string]string{"id": "timestamp", "label": "timestamp", "type": "date"}
    j_data_col[1] = map[string]string{"id": "score", "label": "score", "type": "number"}
    for _, timestamp := range r.list_timestamp {
        //timestamp_i, _ := strconv.Atoi(timestamp)
        score := r.fetchData(timestamp, rankingType, rank)

        log.Print("timestamp ", timestamp, " score ", score)
        vv := map[string][]map[string]interface{}{
            "c": []map[string]interface{}{
                map[string]interface{}{"v":"new Date("+timestamp+")"},
                map[string]interface{}{"v":score},
            },
        }
        if score >= 0 {
            j_item = append(j_item, vv)
            raw += fmt.Sprintf(`{"c":[{"v":new Date(%s000)},{"v":%d}]},`, timestamp, score)
        }
    }
    j_data := map[string]interface{}{"cols": j_data_col, "rows": j_item}
    log.Print(j_data)

    text, err := json.Marshal(j_data)
    _ = text
    if err != nil {
        log.Fatal(err)
    }
    raw += `]}`
    //return string(text)
    return raw
}

func (r *RankServer) rankData_list_f(rankingType int, list_rank []int, dataSource func (string, int, int)interface{}) string {
    //log.Print("functional version of rankData_list_f()")
    r.updateTimestamp()
    raw := ""
    raw += `{"cols":[{"id":"timestamp","label":"timestamp","type":"datetime"},`
    for _, rank := range list_rank {
        raw += fmt.Sprintf(`{"id":"%d","label":"%d","type":"number"},`, rank, rank)
    }
    raw += "\n"
    raw += `],"rows":[`

    for _, timestamp := range r.list_timestamp {
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

func (r *RankServer) fetchData_i(timestamp string, rankingType int, rank int) interface{} {
    var x interface{}
    x = r.fetchData(timestamp, rankingType, rank)
    return x
}

func (r *RankServer) rankData_list_2(rankingType int, list_rank []int) string {
    //return r.rankData_list_f(rankingType, list_rank, func(string, int, int)interface{}(r.fetchData))
    return r.rankData_list_f(rankingType, list_rank, r.fetchData_i)
}

func (r *RankServer) speedData_list(rankingType int, list_rank []int) string {
    //return r.rankData_list_f(rankingType, list_rank, func(string, int, int)interface{}(r.fetchData))
    return r.rankData_list_f(rankingType, list_rank, r.getSpeed_i)
}

// deprecated
func (r *RankServer) rankData_list(rankingType int, list_rank []int) string {
    r.updateTimestamp()
    raw := ""
    raw += `{"cols":[{"id":"timestamp","label":"timestamp","type":"datetime"},`
    for _, rank := range list_rank {
        raw += fmt.Sprintf(`{"id":"%d","label":"%d","type":"number"},`, rank, rank)
    }
    raw += "\n"
    raw += `],"rows":[`

    for _, timestamp := range r.list_timestamp {
        // time in milliseconds
        raw += fmt.Sprintf(`{"c":[{"v":new Date(%s000)},`, timestamp)
        for _, rank := range list_rank {
            score := r.fetchData(timestamp, rankingType, rank)
            //log.Print("timestamp ", timestamp, " score ", score)
            if score >= 0 {
                raw += fmt.Sprintf(`{"v":%d},`, score)
            } else {
                // null: missing point
                raw += fmt.Sprintf(`{"v":null},`)
            }
        }
        raw += fmt.Sprintf(`]},`)
        raw += "\n"
    }
    raw += `]}`
    return raw
}

func (r *RankServer) init_req( w http.ResponseWriter, req *http.Request ) {
    req.ParseForm()
    r.logger.Printf("%T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
}

func (r *RankServer) preload( w http.ResponseWriter, req *http.Request ) {
    r.init_req(w, req)
    fmt.Fprint(w, "<!DOCTYPE html>")
    fmt.Fprint(w, "<html>")
    fmt.Fprint(w, "<body>")
}

func (r *RankServer) preload_c( w http.ResponseWriter, req *http.Request ) {
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
    fmt.Fprint(w, "\nvar data = new google.visualization.DataTable(", r.jsonData(r.latestTimestamp()), ")")
    fmt.Fprint(w, "\nvar data_r = new google.visualization.DataTable(", r.rankData_list_2(0, []int{2001, 10001, 20001, 60001, 120001, 300001}), ")")
    fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list(0, []int{2001, 10001, 20001, 60001, 120001, 300001}), ")")
    fmt.Fprint(w, "\nvar data_speed_12 = new google.visualization.DataTable(", r.speedData_list(0, []int{60001, 120001,}), ")")
    fmt.Fprint(w, "\nvar data_speed_2 = new google.visualization.DataTable(", r.speedData_list(0, []int{2001, 10001, 20001,}), ")")
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

func (r *RankServer) preload_qchart( w http.ResponseWriter, req *http.Request, list_rank []int ) {
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
    fmt.Fprint(w, "\nvar data_rank = new google.visualization.DataTable(", r.rankData_list_2(0, list_rank), ")")
    fmt.Fprint(w, "\nvar data_speed = new google.visualization.DataTable(", r.speedData_list(0, list_rank), ")")
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
        explorer: {},
    };
    var chart = new google.visualization.LineChart(document.getElementById('myLineChart'));
    var chart_speed = new google.visualization.LineChart(document.getElementById('mySpeedChart'));
    chart.draw(data_rank, options);
    chart_speed.draw(data_speed, options);
    }
    `)
    fmt.Fprint(w, `</script>`)
    fmt.Fprint(w, "</head>")
    fmt.Fprint(w, "<html>")
    fmt.Fprint(w, "<body>")
}
func (r *RankServer) postload( w http.ResponseWriter, req *http.Request ) {
    fmt.Fprint(w, "</body>")
    fmt.Fprint(w, "</html>")
}

func (r *RankServer) qHandler( w http.ResponseWriter, req *http.Request ) {
    r.preload(w, req)
    defer r.postload(w, req)
    fmt.Fprint(w, "<pre>")
    defer fmt.Fprint(w, "</pre>")
    //fmt.Fprint( w, r.dumpData() )
    req.ParseForm()
    timestamp, ok := req.Form["t"]
    //log.Print(req.Form)
    if ! ok {
        r.checkData("")
        fmt.Fprint( w, r.latestData() )
    } else {
        //log.Print("showData", timestamp[0])
        r.checkData(timestamp[0])
        fmt.Fprint( w, r.showData(timestamp[0]) )
    }
}

func (r *RankServer) formatTimestamp (timestamp string) string {
    itime, _ := strconv.Atoi(timestamp)
    jst, _ := time.LoadLocation("Asia/Tokyo")
    t := time.Unix(int64(itime), 0).In(jst)
    st := t.Format(time.RFC3339)
    return st
}

func (r *RankServer) homeHandler( w http.ResponseWriter, req *http.Request ) {
    r.preload(w, req)
    defer r.postload(w, req)
    fmt.Fprintf(w, "<br>デレステイベントボーダー<br><br>")

    fmt.Fprintf(w, "<a href=\"log\">%s</a><br>\n", "過去ボーダー")
    fmt.Fprintf(w, "<a href=\"chart\">%s</a><br>\n", "グラフΒ")
    fmt.Fprintf(w, "<br>%s<br>\n", "最新ボーダー")
    r.checkData("")
    fmt.Fprint(w, "<pre>")
    defer fmt.Fprint(w, "</pre>")
    fmt.Fprint( w, r.latestData() )
}


func (r *RankServer) logHandler( w http.ResponseWriter, req *http.Request ) {
    r.updateTimestamp()
    r.preload(w, req)
    defer r.postload(w, req)
    fmt.Fprintf(w, "<br>デレステイベントボーダー<br><br>")
    fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "最新ボーダー")
    for _, timestamp := range r.list_timestamp {
        fmt.Fprintf(w, "<a href=\"q?t=%s\">%s</a><br>\n", timestamp, r.formatTimestamp(timestamp))
    }
}


func (r *RankServer) chartHandler( w http.ResponseWriter, req *http.Request ) {
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

func (r *RankServer) qchartHandler( w http.ResponseWriter, req *http.Request ) {
    r.checkData("")
    req.ParseForm()
    list_rank_str, ok := req.Form["rank"]
    //list_rank = []int
    var list_rank []int
    if ok {
        list_rank = make([]int, 0, len(list_rank_str))
        for _, v := range list_rank_str {
            n, _ := strconv.Atoi(v)
            list_rank = append(list_rank, n)
        }
    } else {
        list_rank = []int{60001, 120001}
    }
    r.preload_qchart(w, req, list_rank)
    defer r.postload(w, req)
    fmt.Fprint(w, `
    uses javascript library from <code>https://www.gstatic.com/charts/loader.js</code><br>`)
    fmt.Fprintf(w, "<a href=\"..\">%s</a><br>\n", "ホームページ")
    fmt.Fprint(w, `
    <table class="columns">
<tr><td><div id="myLineChart" style="border: 1px solid #ccc"/></td></tr>
<tr><td><div id="mySpeedChart" style="border: 1px solid #ccc"/></td></tr>
    </table>
    `)
}

func (r *RankServer) redirectHandler( w http.ResponseWriter, req *http.Request ) {
    fmt.Println("url is ", req.URL)
    req.URL.Host = r.hostname + ":4002"
    req.URL.Scheme = "https"
    fmt.Println("redirecting to ", req.URL)
    http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}

func main() {
    log.Print("RankServer running")
    r := MakeRankServer()
    r.run()
}
