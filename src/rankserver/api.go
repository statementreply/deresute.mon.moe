package rankserver

import (
	"fmt"
	"math"
	"net/http"
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
	// generate json
	fmt.Fprint(w,
		"[\n",
		r.jsonData(rankingType, list_rank, r.fetchData_i, event),
		",\n",
		r.jsonData(rankingType, list_rank, r.getSpeed_i, event),
		"]\n",
	)
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
		if r.currentEvent.Type() == 1 {
			param.list_rank = []int{5001, 10001, 40001}
		} else if r.currentEvent.Type() == 3 {
			param.list_rank = []int{5001, 10001, 50001}
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
