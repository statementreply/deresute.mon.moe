package rankserver

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"
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

func (r *RankServer) getTmplVar(w http.ResponseWriter, req *http.Request) *tmplVar {
	req.ParseForm()
	result := new(tmplVar)

	timestamp, ok := req.Form["t"] // format checked
	if !ok {
		//r.CheckData("")
		//fmt.Fprint(w, r.latestData())
	} else {
		if timestampFilter.MatchString(timestamp[0]) {
			result.Timestamp = timestamp[0]
			//r.CheckData(timestamp[0])
			//fmt.Fprint(w, r.showData(timestamp[0]))
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
	// parse parameters
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
		// new default value
		list_rank = []int{120001}
	}

	event_id_str_list, ok := req.Form["event"] // checked Atoi
	// default value is latest
	event := r.latestEvent
	if event == nil {
		//event = r.latestEvent
		r.logger.Println("latestEvent is nil")
	}
	var prefill_event string = ""
	// this block output: prefill_event, event
	if ok {
		event_id_str := event_id_str_list[0]
		// skip empty string
		if event_id_str == "" {
			event = r.latestEvent
		} else {
			event_id, err := strconv.Atoi(event_id_str)
			if err == nil {
				prefill_event = event_id_str
				event = r.resourceMgr.FindEventById(event_id)
				if event == nil {
					event = r.latestEvent
				}
			} else {
				r.logger.Println("bad event id", err, event_id_str)
			}
		}
	}
	result.PrefillEvent = prefill_event
	var prefill string = "2001 10001 20001 60001 120001"
	{
		n_rank := []string{}
		for _, n := range list_rank {
			n_rank = append(n_rank, fmt.Sprintf("%d", n))
		}
		prefill = strings.Join(n_rank, " ")
	}
	result.PrefillRank = prefill

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
	result.PrefillAChart = fancyChart_checked

	param := &qchartParam{
		rankingType: rankingType,
		list_rank: list_rank,
		event: event,
		fancyChart: fancyChart,
	}
	result.DURL = r.generateDURL(param)
	if param.fancyChart {
		result.AChart = 1
	}

	if r.currentEvent != nil {
		result.EventInfo += "<p>"
		result.EventInfo += "イベント開催中：" + template.HTMLEscapeString(r.currentEvent.Name())
		if r.currentEvent.LoginBonusType() > 0 {
			result.EventInfo += "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。"
		}
		result.EventInfo +="</p>"
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
	fmt.Fprint(w, "</html>\n")
}

var timestampFilter = regexp.MustCompile("^\\d+$")

func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
	r.preload_html(w, req, nil)
	defer r.postload_html(w, req)
	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	//fmt.Fprint( w, r.dumpData() )
	req.ParseForm()
	timestamp, ok := req.Form["t"] // format checked
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
		event:       r.latestEvent,
		fancyChart:  false,
	})
	fmt.Fprint(w, "\n")
	defer r.postload_html(w, req)
	fmt.Fprint(w, `<div id="wrapper">`)
	defer fmt.Fprint(w, "</div>\n")
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
	fmt.Fprintf(w, "            <h3>%s</h3>\n", "最新ボーダー")
	fmt.Fprint(w, "            <pre id=\"latestdata\">")
	fmt.Fprint(w, "</pre>\n")
}

func (r *RankServer) homeHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.CheckData("")
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "home.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qchartHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.CheckData("")
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "qchart.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
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

	event_id_str_list, ok := req.Form["event"] // checked Atoi
	// default value is latest
	event := r.latestEvent
	if event == nil {
		//event = r.latestEvent
		r.logger.Println("latestEvent is nil")
	}
	var prefill_event string = ""
	// this block output: prefill_event, event
	if ok {
		event_id_str := event_id_str_list[0]
		// skip empty string
		if event_id_str == "" {
			event = r.latestEvent
		} else {
			event_id, err := strconv.Atoi(event_id_str)
			if err == nil {
				prefill_event = event_id_str
				event = r.resourceMgr.FindEventById(event_id)
				if event == nil {
					event = r.latestEvent
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

func (r *RankServer) redirectHandler(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = r.hostname + ":4002"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}
