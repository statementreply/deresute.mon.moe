package rankserver

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"resource_mgr"
	"strconv"
	"strings"
	"time"
	ts "timestamp"
)

func (r *RankServer) init_req(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	r.logger.Printf("[INFO] %T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, req.Header, req.Form, req.RequestURI, req.TLS)
}

func (r *RankServer) generateDURL(param *qchartParam) string {
	u := "/d?"
	if param == nil {
		return u
	}
	u += "type=" + fmt.Sprintf("%d", param.rankingType) + "&"
	for _, rank := range param.list_rank {
		u += "rank=" + strconv.Itoa(rank) + "&"
	}
	if param.event == nil {
		return u
	}
	u += "event=" + fmt.Sprintf("%d", param.event.Id()) + "&"
	return u
}

// returns valid timestamp or ""
func (r *RankServer) parseParam_t(req *http.Request) string {
	timestamp, ok := req.Form["t"] // format checked
	if ok {
		if timestampFilter.MatchString(timestamp[0]) {
			return timestamp[0]
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
	return ""
}

// returns list_rank or nil (list could be empty but not nil?)
func (r *RankServer) parseParam_rank(req *http.Request) []int {
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
	}
	return list_rank
}

// returns valid event or nil
func (r *RankServer) parseParam_event(req *http.Request) *resource_mgr.EventDetail {
	var event *resource_mgr.EventDetail
	event_id_str_list, ok := req.Form["event"] // checked Atoi
	if ok {
		event_id_str := event_id_str_list[0]
		// skip empty string
		if event_id_str != "" {
			event_id, err := strconv.Atoi(event_id_str)
			if err == nil {
				event = r.resourceMgr.FindEventById(event_id)
			} else {
				r.logger.Println("bad event id", err, event_id_str)
			}
		}
	}
	// could be nil
	return event
}

// returns 0 or 1
func (r *RankServer) parseParam_type(req *http.Request) int {
	rankingType_str_list, ok := req.Form["type"] // checked Atoi
	if ok {
		rankingType_str := rankingType_str_list[0]
		rankingType_i, err := strconv.Atoi(rankingType_str)
		if err == nil {
			if rankingType_i > 0 {
				return 1
			}
		}
	}
	return 0
}

// returns 0 or 1
func (r *RankServer) parseParam_achart(req *http.Request) int {
	fancyChart_str_list, ok := req.Form["achart"] // ignored, len
	if ok {
		fancyChart_str := fancyChart_str_list[0]
		if len(fancyChart_str) > 0 {
			return 1
		}
	}
	return 0
}

// parse parameters
// available parameters
// - t:       single timestamp
// - rank:    multiple rank int
// - event:   single event_id
// - type:    single 0/1 pt/score
// - achart:  single 0/1 linechart/annotationchart
func (r *RankServer) getTmplVar(w http.ResponseWriter, req *http.Request) *tmplVar {
	result := new(tmplVar)
	req.ParseForm()
	result.Timestamp = r.parseParam_t(req)
	r.CheckData()

	result.list_rank = r.parseParam_rank(req)
	// new default value
	if (result.list_rank == nil) || (len(result.list_rank) == 0) {
		result.list_rank = []int{120001}
	}
	n_rank := []string{}
	for _, n := range result.list_rank {
		n_rank = append(n_rank, fmt.Sprintf("%d", n))
	}
	result.PrefillRank = strings.Join(n_rank, " ")

	result.event = r.parseParam_event(req)
	if result.event == nil {
		// default value is latest
		result.event = r.latestEvent
	}
	if result.event == nil {
		r.logger.Println("latestEvent is nil")
	}
	result.PrefillEvent = ""
	if result.event != nil {
		result.PrefillEvent = strconv.Itoa(result.event.Id())
		result.EventTitle = result.event.Name()
	}
	result.rankingType = r.parseParam_type(req)
	result.PrefillCheckedType = []template.HTMLAttr{"", ""}
	result.PrefillCheckedType[result.rankingType] = " checked"

	result.AChart = r.parseParam_achart(req)
	result.fancyChart = false
	result.PrefillAChart = ""
	if result.AChart == 1 {
		result.fancyChart = true
		result.PrefillAChart = " checked"
	}
	result.DURL = r.generateDURL(&result.qchartParam)
	// for debug
	//r.currentEvent = result.event
	if r.currentEvent != nil {
		result.EventInfo += "<p>"
		result.EventInfo += "イベント開催中："
		result.EventInfo += template.HTML(template.HTMLEscapeString(r.currentEvent.Name()))
		if r.currentEvent.LoginBonusType() > 0 {
			result.EventInfo += "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。"
		}
		result.EventInfo +="</p>"
	}
	result.AvailableRank = [][]int{
		r.get_list_rank(r.latestTimestamp(), 0),
		r.get_list_rank(r.latestTimestamp(), 1),
	}
	return result
}

// rankserver templates
var rsTmpl = template.Must(template.ParseGlob(BASE + "/templates/*.html"))

// now the script is totally static
func (r *RankServer) preload_html(w http.ResponseWriter, req *http.Request, param *qchartParam) {
	fancyChart := false
	if param != nil {
		fancyChart = param.fancyChart
	}
	r.init_req(w, req)
	err := rsTmpl.ExecuteTemplate(w, "preload.html", nil)
	if err != nil {
		r.logger.Println("html/template", err)
	}
	fmt.Fprint(w, `<body>
`)
	fmt.Fprint(w, `    <div data-role="page" data-dom-cache="false">
`)
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
	fmt.Fprint(w, `
    </div>
</body>
</html>
`)
}

var timestampFilter = regexp.MustCompile("^\\d+$")

func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	//fmt.Fprint( w, r.dumpData() )
	req.ParseForm()
	timestamp := r.parseParam_t(req)
	r.CheckData()
	if timestamp == "" {
		fmt.Fprint(w, r.latestData())
	} else {
		fmt.Fprint(w, r.showData(timestamp))
	}
}

func (r *RankServer) homeHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, &qchartParam{
		rankingType: 0,
		list_rank:   []int{120001},
		event:       r.latestEvent,
		fancyChart:  false,
	})
	fmt.Fprint(w, "\n")
	defer r.postload_html(w, req)
	fmt.Fprint(w, `        <div id="wrapper">
`)
	defer fmt.Fprint(w, "        </div>\n")
	fmt.Fprintf(w, "            <h2>デレステイベントボーダーbotβ+</h2>")
	fmt.Fprint(w, "\n")
	// for debug
	//r.currentEvent = r.latestEvent
	if r.currentEvent != nil {
		fmt.Fprintf(w, "            <p>")
		fmt.Fprintf(w, "イベント開催中：%s", r.currentEvent.Name())
		if r.currentEvent.LoginBonusType() > 0 {
			fmt.Fprintf(w, "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。")
		}
		fmt.Fprintf(w, "</p>\n")
	}
	fmt.Fprintf(w, `            <p>twitter bot：十五分毎にイベントptボーダーを更新し、一時間毎にトロフィーと称号ボーダーを更新します。
            <a href="https://twitter.com/deresuteborder0">@deresuteborder0</a></p>
`)

	fmt.Fprintf(w, "            <a href=\"event\">%s</a><br>\n", "過去のイベント (new)")
	fmt.Fprintf(w, "            <a href=\"log\">%s</a><br>\n", "過去のデータ")
	fmt.Fprintf(w, "            <a href=\"m\">%s</a><br>\n", "m-test")
	fmt.Fprint(w, "            <hr>")
	fmt.Fprintf(w, "<h3>%s</h3>\n", "12万位ボーダーグラフ")
	fmt.Fprintf(w, "            （<a href=\"qchart?rank=2001&rank=10001&rank=20001&rank=60001&rank=120001\">%s</a>）<br>\n", "他のボーダーはここ")
	fmt.Fprintf(w, "            （<a href=\"qchart?rank=501&rank=5001&rank=50001&rank=500001\">%s</a>）<br>\n", "イベント称号ボーダー")
	fmt.Fprint(w, r.chartSnippet())
	fmt.Fprint(w, "            <hr>\n")

	r.CheckData()
	// uses ajax
	fmt.Fprintf(w, "            <h3>%s</h3>\n", "最新ボーダー")
	fmt.Fprint(w, "            <pre id=\"latestdata\"></pre>\n")
}

func (r *RankServer) homeHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "home.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qchartHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "qchart.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	if tmplVar.Timestamp == "" {
		tmplVar.Data = r.latestData()
	} else {
		tmplVar.Data = r.showData(tmplVar.Timestamp)
	}
	err := rsTmpl.ExecuteTemplate(w, "q.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) logHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.UpdateTimestamp()
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)

	local_timestamp := r.GetListTimestamp()
	for _, timestamp := range local_timestamp {
		tmplVar.TimestampList = append(
			tmplVar.TimestampList,
			&aTag{
				Link: fmt.Sprintf("q2?t=%s", timestamp),
				Text: ts.FormatTimestamp(timestamp),
			},
		)
	}

	err := rsTmpl.ExecuteTemplate(w, "log.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) chartSnippet() string {
	return `
<div class="ui-grid-a ui-responsive">
<div class="ui-block-a" id="myLineChart">loading...</div>
<div class="ui-block-b" id="mySpeedChart">loading...</div>
</div>
`
}

// mobile landscape optimized
func (r *RankServer) homeMHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, &qchartParam{
		rankingType: 0,
		list_rank:   []int{120001},
		event:       r.latestEvent,
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

func (r *RankServer) qchartHandler(w http.ResponseWriter, req *http.Request) {
	r.CheckData()
	// parse parameters
	req.ParseForm()
	list_rank := r.parseParam_rank(req)
	if (list_rank == nil) || (len(list_rank) == 0) {
		list_rank = []int{60001, 120001}
	}

	event := r.parseParam_event(req)
	// default value is latest
	if event == nil {
		event = r.latestEvent
	}
	if event == nil {
		r.logger.Println("latestEvent is nil")
	}
	prefill_event := ""
	if event != nil {
		prefill_event = strconv.Itoa(event.Id())
	}
	prefill := ""
	{
		n_rank := []string{}
		for _, n := range list_rank {
			n_rank = append(n_rank, fmt.Sprintf("%d", n))
		}
		prefill = strings.Join(n_rank, " ")
	}
	rankingType := r.parseParam_type(req)
	checked_type := []string{"", ""}
	checked_type[rankingType] = " checked"

	achart := r.parseParam_achart(req)
	fancyChart := false
	fancyChart_checked := ""
	if achart == 1 {
		fancyChart = true
		fancyChart_checked = " checked"
	}

	// generate html
	r.preload_html(w, req, &qchartParam{
		rankingType: rankingType,
		list_rank:   list_rank,
		event:       event,
		fancyChart:  fancyChart,
	})
	defer r.postload_html(w, req)
	fmt.Fprintf(w, "        <p><a href=\"..\">%s</a></p>\n", "ホームページ")
	fmt.Fprintf(w, `        <div class="form">
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
	fmt.Fprintf(w, `        <div class="note">
            <p>表示できる順位<br>
            イベントpt：%d<br>
            ハイスコア：%d
            </p>
        </div>
`,
		r.get_list_rank(r.latestTimestamp(), 0),
		r.get_list_rank(r.latestTimestamp(), 1))
	fmt.Fprint(w, `        <div class="note"><p>javascript library from <code>https://www.gstatic.com/charts/loader.js</code></p>
        </div>`)
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

func (r *RankServer) redirectHandler(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = r.hostname + ":4002"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}
