package rankserver

import (
	"apiclient"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"resource_mgr"
	"stoppableListener"
	"sync"
	"time"
)

type qchartParam struct {
	rankingType int
	list_rank   []int
	event       *resource_mgr.EventDetail
	fancyChart  bool
	Delta       time.Duration
}

type twitterParam struct {
	title_suffix string
	title_speed  string
	list_rank    []int
	map_rank     map[int]string
	rankingType  int
	interval     time.Duration
}

type aTag struct {
	Link string
	Text string
}

// extension of EventDetail type
type eventInfo struct {
	*resource_mgr.EventDetail
	EventLink     template.HTML
	EventStart    string
	EventHalf     string
	EventEnd      string
	EventSelected bool
}

type TimeOfSelector struct {
	Second   int64
	Text     string
	Selected bool
}

type tmplVar struct {
	// embed a qchartParam
	qchartParam
	// for homepage currentEvent
	EventInfo template.HTML
	// for others, selected by event=
	EventTitle         string
	Timestamp          string
	DURL               string
	AChart             int
	PrefillEvent       string
	PrefillRank        string
	PrefillAChart      template.HTMLAttr
	PrefillCheckedType []template.HTMLAttr
	AvailableRank      [][]int
	// for "/qchart"
	EventAvailable []*eventInfo
	// for "/q"
	Data string
	// for "/log"
	TimestampList []*aTag
	// for "/event"
	EventList []*eventInfo
	// for "/dist"
	RankingType   int
	ListTimeOfDay []*TimeOfSelector
	ListDate      []*TimeOfSelector
	// for twitter card
	TwitterCardURL string
}

type eventDataRow struct {
	T		int64  `json:"t"`
	Status	int	   `json:"status"`
	Tooltip	string `json:"tooltip"`
}

type RankServer struct {
	//    map[timestamp][rankingType][rank] = score
	// {"1467555420":   [{10: 2034} ,{30: 203021} ]  }
	list_timestamp []string // need mutex?
	// for both read and write
	mux_timestamp sync.RWMutex
	// sql
	rankDB       string
	db           *sql.DB
	logger       *log.Logger
	keyFile      string
	certFile     string
	plainServer  *http.Server
	tlsServer    *http.Server
	slHTTP       *stoppableListener.StoppableListener
	slTLS        *stoppableListener.StoppableListener
	hostname     string
	resourceMgr  *resource_mgr.ResourceMgr
	currentEvent *resource_mgr.EventDetail
	latestEvent  *resource_mgr.EventDetail
	client       *apiclient.ApiClient
	lastCheck    time.Time
	config       map[string]string
}
