package main
import (
    "fmt"
    "bufio"
    "net/http"
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
)


var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"
// 15min update interval
var INTERVAL int = 15 * 60

type RankServer struct {
    //data map[string]map[string]string
    data map[string][]map[int]int
    speed map[string][]map[int]float32
    //data_cache map[string][]map[int]bool
    // {"1467555420": 
    //    [{10: 2034} ,{30: 203021} ]
    //  }
    list_timestamp []string
}

func (r *RankServer) updateTimestamp() {
    dir, err := os.Open(RANK_CACHE_DIR)
    defer dir.Close()
    if err != nil {
        log.Fatal(err)
    }

    fi, _ := dir.Readdir(0)
    r.list_timestamp = make([]string, 0, len(fi))
    // sub: dir name 1467555420
    for _, sub := range fi {
        if sub.IsDir() {
            r.list_timestamp = append(r.list_timestamp, sub.Name())
        }
    }
    sort.Strings(r.list_timestamp)
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
        r.fetchData_3(timestamp, rankingType, rank)
    }
}

func (r *RankServer) getFilename(timestamp string, rankingType, rank int) string {
    subdirPath := RANK_CACHE_DIR + timestamp + "/"
    a := rankingType + 1
    b := int((rank - 1) / 10) + 1
    fileName := subdirPath + fmt.Sprintf("r%02d.%06d", a, b)

    return fileName
}

func (r *RankServer) fetchData_3(timestamp string, rankingType int, rank int) int {
    fileName := r.getFilename(timestamp, rankingType, rank)
    return r.fetchData_internal(timestamp, rankingType, rank, fileName)
}

func (r *RankServer) fetchData_internal(timestamp string, rankingType int, rank int, fileName string) int {
    _, ok := r.data[timestamp]
    //_, ok2 := r.data_cache[timestamp]
    if ! ok {
        r.data[timestamp] = make([]map[int]int, 2)
        r.data[timestamp][0] = make(map[int]int)
        r.data[timestamp][1] = make(map[int]int)
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
        r.data[timestamp][rankingType][rank] = score
    } else {
        //rank := r.FilenameToRank(fileName)
        r.data[timestamp][rankingType][rank] = 0
    }
    //}
    //r.data_cache[timestamp][rankingType][rank] = true
    return r.data[timestamp][rankingType][rank]
}

// speed per hour
func (r *RankServer) getSpeed(timestamp string, rankingType int, rank int) float32 {
    return 0.0
}

// deprecated
func (r *RankServer) ReadFile(fileName string) string {
    var content string
    content = ""
    fh, _ := os.Open(fileName)
    defer fh.Close()
    bfh := bufio.NewReader(fh)
    filter, _ := regexp.Compile("^\\s*(score|rank):")
    for {
        line, _ := bfh.ReadString('\n')
        if line == "" {
            break
        }
        if filter.MatchString(line) {
            content += line
            //log.Print(line)
        }
    }
    return content
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
    http.ListenAndServe(":4001", nil)
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
        score := r.fetchData_3(timestamp, rankingType, rank)

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
            score := r.fetchData_3(timestamp, rankingType, rank)
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
    log.Printf("%T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
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
    fmt.Fprint(w, "\nvar data_r = new google.visualization.DataTable(", r.rankData_list(0, []int{2001, 10001, 20001, 60001, 120001, 300001}), ")")


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
    // Instantiate and draw the chart.
    var chart = new google.visualization.LineChart(document.getElementById('myLineChart'));
    var chart_a = new google.visualization.AnnotationChart(document.getElementById('myAnnotationChart'));
    chart.draw(data_r, options);
    chart_a.draw(data_r, options_a);
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
    fmt.Fprintf(w, "<a href=\"chart\">%s</a><br>\n", "chart beta")
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
      <tr>
        <td><div id="myLineChart" style="border: 1px solid #ccc"/></td>
</tr>
<tr>
        <td><div id="myAnnotationChart"/></td>
      </tr>
    </table>
    `)
}


func MakeRankServer() *RankServer {
    r := &RankServer{}
    r.data = make(map[string][]map[int]int)
    //r.data_cache = make(map[string][]map[int]bool)
    //r.list_timestamp doesn't need initialization
    http.HandleFunc("/", r.homeHandler)
    http.HandleFunc("/q", r.qHandler)
    http.HandleFunc("/log", r.logHandler)
    http.HandleFunc("/chart", r.chartHandler)
    return r
}

func main() {
    r := MakeRankServer()
    r.run()
}
