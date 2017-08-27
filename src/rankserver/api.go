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
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	res "resource_mgr"
	//"strconv"
	"strings"
	"time"
	ts "timestamp"
	"unicode/utf8"
)

func (r *RankServer) latestDataHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	fmt.Fprint(w, r.latestData())
}

// for "/d"
func (r *RankServer) dataHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	// parse parameters
	req.ParseForm()
	list_rank := r.parseParam_rank(req)
	if (list_rank == nil) || (len(list_rank) == 0) {
		list_rank = []int{60001, 120001}
	}

	event := r.parseParam_event(req)
	if event == nil {
		event = r.latestEvent
	}
	if event == nil {
		r.logger.Println("latestEvent is nil")
		return
	}
	rankingType := r.parseParam_type(req)
	delta := time.Duration(r.parseParam_delta(req)) * time.Second
	if delta == 0 {
		delta = INTERVAL
	}
	// generate json
	fmt.Fprint(w,
		"[\n",
		r.jsonData(rankingType, list_rank, r.fetchData_i, event, delta),
		",\n",
		r.jsonData(rankingType, list_rank, r.getSpeed_i, event, delta),
		"]\n",
	)
}

// for "/d2" version 2 data
// time change for events
// multiple events
// single rank
// event name
// time startday=0
func (r *RankServer) dataHandlerV2(w http.ResponseWriter, req *http.Request) {
	rankingType := 0
	//delta := INTERVAL
	result := make(map[string][][2]int)

	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	list_event := r.parseParam_events(req)
	var rank int
	{
		list_rank := r.parseParam_rank(req)
		if len(list_rank) >= 1 {
			rank = list_rank[0]
		} else {
			// error message?
			//return
			rank = 120001
		}
	}
	for _, event := range list_event {
		//fmt.Fprint(w, "[\n", r.jsonData(rankingType, []int{rank}, r.fetchData_i, event, delta), "]\n")
		//eventName, eventBorder := r.fetchEventBorder(event, rankingType, rank)
		eventName := event.ShortName()
		eventBorder := r.fetchEventBorder(event, rankingType, rank)
		result[eventName] = eventBorder
	}
	//fmt.Println("len of dataV2 is", len(result))
	b, err := json.Marshal(result)
	if err != nil {
		// report err
		return
	}
	//fmt.Fprintf(w, "123\n")
	w.Write(b)
}

func (r *RankServer) getDensity(timestamp string) []map[int]float32 {
	item := r.fetchDataSlice(timestamp)
	result := make([]map[int]float32, len(item))
	for i := 0; i < len(item); i++ {
		sorted_k := r.get_list_rank(timestamp, i)
		result[i] = make(map[int]float32)
		for j, k := range sorted_k {
			//cur_k = k
			var diff float32
			if j < len(sorted_k)-1 {
				next_k := sorted_k[j+1]
				diff = float32(next_k-k) / float32(item[i][k]-item[i][next_k])
			} else {
				diff = float32(10000.0) / float32(item[i][k])
			}
			if !math.IsInf(float64(diff), 1) {
				result[i][k] = diff
			}
		}
	}
	return result
}

func mapToJson(v interface{}, list_key_v interface{}) string {
	get_value := func(k int) interface{} {
		item, ok := v.(map[int]int)
		if ok {
			return item[k]
		} else {
			return v.(map[int]float32)[k]
		}
	}
	list_key := list_key_v.([]int)
	result := ""
	result += "[\n"
	needComma := false

	for _, k := range list_key {
		//v := item[k]
		v := get_value(k)
		if needComma {
			result += ","
		}
		result += fmt.Sprint(`[`, k, `,`, v, `]`)
		needComma = true
	}
	result += "]\n"
	return result
}

func (r *RankServer) distDataHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	timestamp := r.parseParam_t(req)
	if timestamp == "" {
		timestamp = r.latestTimestamp()
	}
	item := r.fetchDataSlice(timestamp)
	fmt.Fprint(w, "[\n")
	// len(item) == 2
	for i := 0; i < len(item); i++ {
		sorted_k := r.get_list_rank(timestamp, i)
		fmt.Fprint(w, mapToJson(item[i], sorted_k))
		fmt.Fprint(w, ",")
	}
	item2 := r.getDensity(timestamp)
	for i := 0; i < len(item2); i++ {
		sorted_k := r.get_list_rank(timestamp, i)
		fmt.Fprint(w, mapToJson(item2[i], sorted_k))
		if i < len(item2)-1 {
			fmt.Fprint(w, ",")
		}
	}
	fmt.Fprint(w, "]\n")
}

func (r *RankServer) distCompareDataHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	r.CheckData()
	req.ParseForm()
	events := r.parseParam_events(req)
	// list of [timestamp, event_name]
	var result [][]string
	for _, event := range events {
		eventTs := r.latestEventTimestamp(event)
		result = append(result, []string{eventTs, event.LongName()})
	}
	b, err := json.Marshal(result)
	if err != nil {
		fmt.Println(err)
		return
	}
	w.Write(b)
}

// js timezone bug
// date overriding
func (r *RankServer) eventDataHandler(w http.ResponseWriter, req *http.Request) {
	r.init_req(w, req)
	var list_day []eventDataRow
	for _, e := range r.resourceMgr.EventList {
		start := e.EventStart() //.Truncate(time.Hour * 24)
		end := e.EventEnd()     //.Truncate(time.Hour * 24)
		name := e.Name()
		if time.Now().Add(50 * 24 * time.Hour).Before(start) {
			continue
		}
		//fmt.Println("start:", start.Unix()/86400)
		step := time.Hour * 24
		offset := time.Hour * 9
		// first day in JST
		for mid := start.Add(offset).Truncate(step).Add(step).Add(-offset); // init
		mid.Before(end);                                                    // cond
		mid = mid.Add(step) {                                               // incr
			tipStr := ts.FormatDate(mid) + "\n" + name
			list_day = append(list_day, eventDataRow{
				T:       mid.Unix(),
				Status:  5,
				Tooltip: tipStr,
			})
			//fmt.Println("mid:", mid.Unix()/86400)
		}
		list_day = append(list_day, eventDataRow{
			T:       start.Unix(),
			Status:  0,
			Tooltip: ts.FormatDate(start) + "\n" + name,
		})
		list_day = append(list_day, eventDataRow{
			T:       end.Unix(),
			Status:  10,
			Tooltip: ts.FormatDate(end) + "\n" + name,
		})
		//fmt.Println("end:", end.Unix()/86400)
	}
	today_tooltip := "今日 " + ts.FormatDate(time.Now())
	// FIXME: cut at event_end
	if r.currentEvent != nil && !time.Now().After(r.currentEvent.EventEnd()) {
		today_tooltip += "\n" + r.currentEvent.Name()
	}
	list_day = append(list_day, eventDataRow{
		T:       time.Now().Unix(),
		Status:  15,
		Tooltip: today_tooltip,
	})
	game_start, err := time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Thu Sep 3 12:00:00 +0900 JST 2015")
	if err != nil {
		r.logger.Fatalln(err)
	}
	game_start2, err := time.Parse("Mon Jan 2 15:04:05 -0700 MST 2006", "Thu Sep 10 12:00:00 +0900 JST 2015")
	if err != nil {
		r.logger.Fatalln(err)
	}
	list_day = append(list_day, eventDataRow{
		T:       game_start.Unix(),
		Status:  0,
		Tooltip: "150903\nAndroid版配信開始",
	})
	list_day = append(list_day, eventDataRow{
		T:       game_start2.Unix(),
		Status:  0,
		Tooltip: "150910\niOS版配信開始",
	})
	b, err := json.Marshal(list_day)
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println(string(b))
	w.Write(b)
}

func (r *RankServer) twitterHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "",
		title_speed:  "",
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
	if (r.currentEvent != nil) && (r.currentEvent.Type() == 5) {
		fmt.Fprint(w, "EMPTY")
		return
	}
	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterEmblemHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "\n" + "イベント称号ボーダー",
		title_speed:  "（時速）",
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
	if (r.currentEvent != nil) && (r.currentEvent.Type() == 5) {
		fmt.Fprint(w, "EMPTY")
		return
	}
	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterTrophyHandler(w http.ResponseWriter, req *http.Request) {
	param := twitterParam{
		title_suffix: "\n" + "トロフィーボーダー",
		title_speed:  "（時速）",
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
		// FIXME move to resource_mgr.event
		if r.currentEvent.Type() == 1 {
			param.list_rank = []int{5001, 10001, 40001}
		} else if r.currentEvent.Type() == 3 {
			param.list_rank = []int{5001, 10001, 50001}
		} else if r.currentEvent.Type() == 5 {
			param.list_rank = []int{5001, 10001, 50001}
		}

		timestamp := r.latestTimestamp()
		t := ts.TimestampToTime(timestamp)
		// for atapon and groove, every hour, but every 15 min in the last hour
		// for live parade, every 15 min
		if (r.currentEvent.Type() != res.EventTour) &&
		   (!ts.IsWholeHour(timestamp)) &&
		   (r.currentEvent.EventEnd().Sub(t) >= time.Hour) {
			fmt.Fprint(w, "EMPTY")
			return
		}
	}

	r.twitterHandler_common(w, req, param)
}

func (r *RankServer) twitterHandler_common(w http.ResponseWriter, req *http.Request, param twitterParam) {
	r.init_req(w, req)
	var status string
	r.CheckData()
	timestamp := r.latestTimestamp()
	var title string

	timestamp_str := ts.FormatTimestamp_short(timestamp)
	var isFinal = false

	// exclude caravan/live-party
	if (r.currentEvent != nil) && (r.currentEvent.HasRanking()) {
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
		//if isFinal {
		//	r.logger.Println("isFinal debug", "timestamp", timestamp)
		//	r.logger.Println("isFinal debug", "timestamp_prev", timestamp_prev)
		//}

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
