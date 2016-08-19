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

// rankserver templates
var rsTmpl = template.Must(template.ParseGlob(BASE + "/templates/*.html"))
var timestampFilter = regexp.MustCompile("^\\d+$")
var staticFilter = regexp.MustCompile("^/static")

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
		result.EventInfo += "</p>"
	}
	result.AvailableRank = [][]int{
		r.get_list_rank(r.latestTimestamp(), 0),
		r.get_list_rank(r.latestTimestamp(), 1),
	}
	return result
}

func (r *RankServer) homeHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "home.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

// mobile landscape optimized
func (r *RankServer) homeMHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "m.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qchartHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	for _, e := range r.resourceMgr.EventList {
		if r.isEventAvailable(e) {
			selected := false
			if tmplVar.event != nil {
				selected = tmplVar.event.Id() == e.Id()
			}
			tmplVar.EventAvailable = append(tmplVar.EventAvailable,
				&eventInfo{
					EventDetail: e,
					EventSelected: selected,
				})
		}
	}
	err := rsTmpl.ExecuteTemplate(w, "qchart.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
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

// distribution
func (r *RankServer) distHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	if tmplVar.Timestamp == "" {
		tmplVar.Timestamp = r.latestTimestamp()
	}
	tmplVar.RankingType = tmplVar.rankingType
	err := rsTmpl.ExecuteTemplate(w, "dist.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) logHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.UpdateTimestamp()
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)

	local_timestamp := r.GetListTimestamp()
	for _, timestamp := range local_timestamp {
		tmplVar.TimestampList = append(
			tmplVar.TimestampList,
			&aTag{
				Link: fmt.Sprintf("q?t=%s", timestamp),
				Text: ts.FormatTimestamp(timestamp),
			},
		)
	}

	err := rsTmpl.ExecuteTemplate(w, "log.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) isEventAvailable(e *resource_mgr.EventDetail) bool {
	// ranking information available after 2016-07
	if e.HasRanking() && e.EventEnd().After(time.Unix(1467552720, 0)) {
		return true
	} else {
		return false
	}
}

func (r *RankServer) eventHandler_new2(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.UpdateTimestamp()
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	formatter := ts.FormatTime
	for _, e := range r.resourceMgr.EventList {
		name := template.HTML(template.HTMLEscapeString(e.Name()))
		if r.isEventAvailable(e) {
			name_tmp := `<a href="qchart?event=` + strconv.Itoa(e.Id()) + `">` + string(name) + `</a>`
			name = template.HTML(name_tmp)
		}
		tmplVar.EventList = append(
			tmplVar.EventList,
			&eventInfo{
				EventLink:  name,
				EventStart: formatter(e.EventStart()),
				EventHalf:  formatter(e.SecondHalfStart()),
				EventEnd:   formatter(e.EventEnd()),
			},
		)
	}
	err := rsTmpl.ExecuteTemplate(w, "event.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

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
	r.init_req(w, req)
	req.URL.Host = r.hostname + ":4002"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}
