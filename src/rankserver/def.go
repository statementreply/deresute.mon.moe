package rankserver

import (
	"apiclient"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"resource_mgr"
	"stoppableListener"
	"sync"
	"html/template"
	"time"
)

type qchartParam struct {
	rankingType int
	list_rank   []int
	event       *resource_mgr.EventDetail
	fancyChart  bool
}

type twitterParam struct {
	title_suffix string
	title_speed  string
	list_rank    []int
	map_rank     map[int]string
	rankingType  int
	interval     time.Duration
}

type tmplVar struct {
	// embed a qchartParam
	qchartParam
	EventInfo string
	Timestamp string
	DURL string
	AChart int
	PrefillEvent string
	PrefillRank string
	PrefillAChart template.HTMLAttr
	PrefillCheckedType []template.HTMLAttr
	AvailableRank [][]int
}

type RankServer struct {
	//    map[timestamp][rankingType][rank] = score
	// {"1467555420":   [{10: 2034} ,{30: 203021} ]  }
	list_timestamp []string                     // need mutex?
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
