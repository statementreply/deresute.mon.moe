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
	"fmt"
	"html/template"
	"net/http"
	"os"
	"regexp"
	"resource_mgr"
	"sort"
	"strconv"
	"strings"
	"time"
	ts "timestamp"
)

// rankserver templates
var rsTmpl = template.Must(template.ParseGlob(BASE + "/templates/*.html"))
var timestampFilter = regexp.MustCompile("^\\d+$")
var staticFilter = regexp.MustCompile("^/static")

func dumpHeader(header *http.Header) string {
	var result string
	var keys []string
	for k := range *header {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var kv []string
	for _, k := range keys {
		kv = append(kv, fmt.Sprintf("%v:%v", k, (*header)[k]))
	}
	result = strings.Join(kv, " ")
	result = "map[" + result + "]"
	return result
}

// log and parseform
func (r *RankServer) init_req(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	// req.Header is a map, with unordered keys
	r.logger.Printf("[INFO] %T <%s> \"%v\" %s <%s> %v %v %s %v\n", req, req.RemoteAddr, req.URL, req.Proto, req.Host, dumpHeader(&req.Header), req.Form, req.RequestURI, req.TLS)
	//fmt.Println(dumpHeader(&req.Header))
}

func (r *RankServer) generateDURL(param *qchartParam) string {
	u := "/d?"
	if param == nil {
		return u
	}
	u += "type=" + fmt.Sprintf("%d", param.RankingType) + "&"
	for _, rank := range param.list_rank {
		u += "rank=" + strconv.Itoa(rank) + "&"
	}
	if param.event == nil {
		return u
	}
	if param.Delta != 0 {
		u += "delta=" + fmt.Sprintf("%d", int64(param.Delta/time.Second))
		u += "&"
		//r.logger.Println("delta:", u)
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

// returns valid timestamp or ""
func (r *RankServer) parseParam_date(req *http.Request) int64 {
	timestamp, ok := req.Form["date"] // format checked
	if ok {
		if timestampFilter.MatchString(timestamp[0]) {
			n, err := strconv.ParseInt(timestamp[0], 10, 64)
			if err != nil {
				return 0
			}
			return n
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
	return 0
}

func (r *RankServer) parseParam_isfinal(req *http.Request) bool {
	isfinal, ok := req.Form["isfinal"]
	if ok && isfinal[0] == "1" {
		return true
	}
	return false
}

// returns valid timestamp or ""
func (r *RankServer) parseParam_time(req *http.Request) int64 {
	timestamp, ok := req.Form["time"] // format checked
	if ok {
		if timestampFilter.MatchString(timestamp[0]) {
			n, err := strconv.ParseInt(timestamp[0], 10, 64)
			if err != nil {
				return 0
			}
			return n
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
	return 0
}

func (r *RankServer) parseParam_delta(req *http.Request) int64 {
	delta, ok := req.Form["delta"] // format checked
	if ok {
		if timestampFilter.MatchString(delta[0]) {
			n, err := strconv.ParseInt(delta[0], 10, 64)
			if err != nil {
				return 0
			}
			//r.logger.Println("delta:", n)
			return n
		} else {
			r.logger.Println("bad req", req.Form)
		}
	}
	return 0
}

// returns list_rank or nil (list could be empty but not nil?)
// FIXME: limit list length to 20, avoid high cpu usage
func (r *RankServer) parseParam_rank(req *http.Request) []int {
	list_rank_str, ok := req.Form["rank"] // format checked split, strconv.Atoi
	var list_rank []int
	if ok {
		list_rank = make([]int, 0, 20)
		for _, v := range list_rank_str {
			// now v can contain more than one number
			//fmt.Println("str<"+v+">")
			subv := strings.Split(v, " ")
			for _, vv := range subv {
				n, err := strconv.Atoi(vv)
				if (err == nil) && (n >= 1) && (n <= 1000001) {
					list_rank = append(list_rank, n)
					if (len(list_rank) >= 20) {
						break
					}
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

// FIXME: parse multiple events
// return at most 10 event ids
func (r *RankServer) parseParam_events(req *http.Request) []*resource_mgr.EventDetail {
	var events []*resource_mgr.EventDetail
	event_id_str_list, ok := req.Form["event"] // checked Atoi
	if ok {
		for _, event_id_str := range event_id_str_list {
			// skip empty string
			if event_id_str != "" {
				event_id, err := strconv.Atoi(event_id_str)
				if err == nil {
					events = append(events, r.resourceMgr.FindEventById(event_id))
					if len(events) >= 10 {
						break
					}
				} else {
					r.logger.Println("bad event id", err, event_id_str)
				}
			}
		}
	}
	return events
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

// returns a string for ChartType
func (r *RankServer) parseParam_ctype(req *http.Request) string {
	// the default
	chartType := "myTWCChart"
	chartType_str_list, ok := req.Form["ctype"]
	if ok {
		chartType_str := chartType_str_list[0]
		if chartType_str == "myDistChart" {
			chartType = chartType_str
		}
	}
	return chartType
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
	result.Delta = time.Duration(r.parseParam_delta(req)) * time.Second
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
		result.EventTitle = result.event.LongName()
	}
	result.RankingType = r.parseParam_type(req)
	result.PrefillCheckedType = []template.HTMLAttr{"", ""}
	result.PrefillCheckedType[result.RankingType] = " checked"

	result.AChart = r.parseParam_achart(req)
	result.fancyChart = false
	result.PrefillAChart = ""
	if result.AChart == 1 {
		result.fancyChart = true
		result.PrefillAChart = " checked"
	}
	result.DURL = r.generateDURL(&result.qchartParam)
	result.DURL2 = req.URL.RawQuery
	// for debug
	//r.currentEvent = result.event
	if r.currentEvent != nil {
		result.EventInfo += "<p>"
		result.EventInfo += "イベント開催中："
		result.EventInfo += template.HTML(template.HTMLEscapeString(r.currentEvent.LongName()))
		if r.currentEvent.LoginBonusType() > 0 {
			result.EventInfo += "<br>ログインボーナスがあるので、イベントページにアクセスを忘れないように。"
		}
		result.EventInfo += "</p>"
	}
	result.AvailableRank = [][]int{
		r.get_list_rank(r.latestTimestamp(), 0),
		r.get_list_rank(r.latestTimestamp(), 1),
	}
	// common "/dist" "/qchart"
	for _, e := range r.resourceMgr.EventList {
		if r.isEventAvailable(e) {
			selected := false
			if result.event != nil {
				selected = result.event.Id() == e.Id()
			}
			result.EventAvailable = append(result.EventAvailable,
				&eventInfo{
					EventDetail:   e,
					EventSelected: selected,
				})
		}
	}
	// FIXME: hardcode
	result.TwitterCardURL = template.HTML("https://" + r.hostname + "/twc")
	tnow := time.Now()
	result.NowJST = tnow.In(ts.TZ()).Format(time.RFC3339)
	result.RemainingTime = ""
	result.RunningTime = ""
	if r.currentEvent != nil {
		remainingTime := r.currentEvent.EventEnd().Sub(tnow)
		remainingTimeStr := remainingTime.String()
		result.RemainingTime = remainingTimeStr
		result.RunningTime = tnow.Sub(r.currentEvent.EventStart()).String()
	}
	return result
}

func (r *RankServer) homeHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "home.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) twcHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	// parse div
	//tmplVar.ChartType = "myTWCChart"
	tmplVar.ChartType = r.parseParam_ctype(req)
	err := rsTmpl.ExecuteTemplate(w, "twitter_card.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) twcTestHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "twc_test.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

// mobile landscape optimized
func (r *RankServer) homeMHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "m.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qchartHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	tmplVar := r.getTmplVar(w, req)
	err := rsTmpl.ExecuteTemplate(w, "qchart.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) qHandler(w http.ResponseWriter, req *http.Request) {
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
func (r *RankServer) distHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	//if tmplVar.Timestamp == "" {
	//	tmplVar.Timestamp = r.latestTimestamp()
	//}
	if tmplVar.event == nil {
		r.logger.Println("nil event in distHandler")
		return
	}
	//tmplVar.RankingType = tmplVar.rankingType
	t_date := r.parseParam_date(req)
	t_time := r.parseParam_time(req)
	isFinal := r.parseParam_isfinal(req)
	t_offset := int64(9 * 3600)
	if (t_date > 0) && (t_time > 0) {
		//tmplVar.Timestamp = strconv.FormatInt(t_date+t_time-t_offset, 10)
	} else {
		//lt := r.latestTimestamp()
		// allow 2min to update
		lt := r.truncateTimestamp(time.Now().Add(-2 * time.Minute))
		t, err := strconv.ParseInt(lt, 10, 64)
		t += t_offset
		if err == nil {
			t_time = t % (3600 * 24)
			t_date = t - t_time
		}
	}
	if tmplVar.Timestamp == "" {
		tmplVar.Timestamp = strconv.FormatInt(t_date+t_time-t_offset, 10)
	}
	// override Timestamp
	if isFinal {
		// should be in range ResultStart ResultEnd
		// and in the current list of timestamps
		// FIXME when the final result is not yet available
		for _, t := range r.GetListTimestamp() {
			t0 := ts.TimestampToTime(t)
			if (!t0.Before(tmplVar.event.ResultStart())) && (!t0.After(tmplVar.event.ResultEnd())) {
				tmplVar.Timestamp = t
				break
			}
		}
		tmplVar.IsFinal = true
	}
	for i := 0; i < 24*4; i++ {
		tmplVar.ListTimeOfDay = append(tmplVar.ListTimeOfDay,
			&TimeOfSelector{
				// FIXME: fix time step hardcode
				Second:   int64(i*900 + 120),
				Text:     fmt.Sprintf("%02d:%02d", i/4, (i%4)*15+2),
				Selected: t_time == int64(i*900+120),
			})
	}
	// fill in event date range
	if tmplVar.event != nil {
		baseTime := tmplVar.event.EventStart().Truncate(time.Hour * 24)
		baseTime0 := baseTime
		for !baseTime.After(tmplVar.event.ResultStart()) {
			dayText := "day " + strconv.FormatInt(1+int64(baseTime.Sub(baseTime0)/(24*time.Hour)), 10) + ": " + baseTime.Format("2006 01-02")
			tmplVar.ListDate = append(tmplVar.ListDate,
				&TimeOfSelector{
					Second:   baseTime.Unix(),
					Text:     dayText,
					Selected: t_date == baseTime.Unix(),
				})
			baseTime = baseTime.Add(time.Hour * 24)
		}
	}
	tmplVar.FinalTime = tmplVar.event.ResultStart().Add(2 * time.Minute).Format("2006 01-02 15:04")
	err := rsTmpl.ExecuteTemplate(w, "dist.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

// dist_compare
// input: event ids
// output: [(timestamp, event_name)]
// timestamp = event final
func (r *RankServer) distCompareHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	tmplVar.TwitterCardURL = template.HTML("https://" + r.hostname + "/twc?ctype=myDistChart&arg=dist_compare&hlog=1")

	err := rsTmpl.ExecuteTemplate(w, "dist_compare.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) timeCompareHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	tmplVar.TwitterCardURL = template.HTML("https://" + r.hostname + "/twc?ctype=myDistChart&arg=time_compare&event=1022")

	err := rsTmpl.ExecuteTemplate(w, "time_compare.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func (r *RankServer) logHandler(w http.ResponseWriter, req *http.Request) {
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

func (r *RankServer) eventHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.UpdateTimestamp()
	r.CheckData()
	req.ParseForm()
	tmplVar := r.getTmplVar(w, req)
	formatter := ts.FormatTime
	n := len(r.resourceMgr.EventList)
	tmplVar.EventList = make([]*eventInfo, n)
	for i, e := range r.resourceMgr.EventList {
		name := template.HTML(template.HTMLEscapeString(e.Name()))
		if r.isEventAvailable(e) {
			name_tmp := `<a href="qchart?event=` + strconv.Itoa(e.Id()) + `">` + string(name) + `</a>`
			name = template.HTML(name_tmp)
		}
		// FIXME: visible change
		if e.Type() == 3 || e.Type() == 5 {
			name += template.HTML(template.HTMLEscapeString(" = " + e.MusicName()))
		}
		/*
			tmplVar.EventList = append(
				tmplVar.EventList,
				&eventInfo{
					EventLink:  name,
					EventStart: formatter(e.EventStart()),
					EventHalf:  formatter(e.SecondHalfStart()),
					EventEnd:   formatter(e.EventEnd()),
				},
			)
		*/
		tmplVar.EventList[n-i-1] =
			&eventInfo{
				EventLink:  name,
				EventStart: formatter(e.EventStart()),
				EventHalf:  formatter(e.SecondHalfStart()),
				EventEnd:   formatter(e.EventEnd()),
				EventDuration: e.EventEnd().Add(time.Second).Sub(e.EventStart()).String(),
			}
		//r.logger.Println("test FindMedleyTitle", r.resourceMgr.FindMedleyTitle(e.Id()))
	}
	err := rsTmpl.ExecuteTemplate(w, "event.html", tmplVar)
	if err != nil {
		r.logger.Println("html/template", err)
	}
}

func Exists(fileName string) bool {
	_, err := os.Stat(fileName)
	if err == nil {
		return true
	} else {
		if os.IsNotExist(err) {
			return false
		} else {
			return true
		}
	}
}

func IsDir(fileName string) bool {
	fi, err := os.Stat(fileName)
	if err == nil {
		return fi.IsDir()
	} else {
		return false
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
	//r.logger.Println("[INFO] servefile", filename)
	// block dir
	if Exists(filename) && (!IsDir(filename)) {
	} else {
		filename = "/dev/null"
	}
	http.ServeFile(w, req, filename)
}

func (r *RankServer) redirectHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	req.URL.Host = r.hostname + ":4002"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), http.StatusMovedPermanently)
}
