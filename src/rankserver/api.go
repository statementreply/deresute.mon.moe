package rankserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	ts "timestamp"
	"unicode/utf8"
)

func (r *RankServer) latestDataHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, r.latestData())
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

	event_id_str_list, ok := req.Form["event"] // checked Atoi
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

