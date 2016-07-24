package datafetcher

import (
	"apiclient"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"resource_mgr"
	"time"
)

var ErrNoEvent = errors.New("no event is running now")
var ErrEventType = errors.New("current event type has no ranking")
var ErrRankingNA = errors.New("current time is not in event/result period")

type DataFetcher struct {
	Client         *apiclient.ApiClient
	resourceMgr    *resource_mgr.ResourceMgr
	key_point      [][2]int
	rank_cache_dir string
}

func NewDataFetcher(client *apiclient.ApiClient, key_point [][2]int, rank_cache_dir, resource_cache_dir string) *DataFetcher {
	log.Println("NewDataFetcher()")
	df := new(DataFetcher)

	df.Client = client
	//client.LoadCheck()
	df.key_point = key_point
	df.rank_cache_dir = rank_cache_dir

	df.Client.LoadCheck()
	rv := client.Get_res_ver()
	df.resourceMgr = resource_mgr.NewResourceMgr(rv, resource_cache_dir)

	//log.Println(GetLocalTimestamp())
	//log.Println(RoundTimestamp(time.Now()).String())
	return df
}

func (df *DataFetcher) Run() error {
	// handle new res_ver
	df.Client.LoadCheck()
	rv := df.Client.Get_res_ver()
	df.resourceMgr.Set_res_ver(rv)

	df.resourceMgr.ParseEvent()
	currentEvent := df.resourceMgr.FindCurrentEvent()

	if currentEvent == nil {
		return ErrNoEvent
	}

	for _, key := range df.key_point {
		//log.Println("rankingtype:", key[0], "rank:", key[1])
		statusStr, err := df.GetCache(currentEvent, key[0], RankToPage(key[1]))
		if err != nil {
			//log.Fatal(err)
			return err
		}
		fmt.Print(statusStr) // progress bar
	}
	fmt.Print("\n")
	return nil
}

func RankToPage(rank int) int {
	var page int
	page = ((rank - 1) / 10) + 1
	return page
}

func DumpToStdout(v interface{}) {
	yy, _ := yaml.Marshal(v)
	log.Println(string(yy))
}

func DumpToFile(v interface{}, fileName string) {
	yy, _ := yaml.Marshal(v)
	ioutil.WriteFile(fileName, yy, 0644)
}

// string for hit/miss
func (df *DataFetcher) GetCache(currentEvent *resource_mgr.EventDetail, ranking_type int, page int) (string, error) {
	event_type := currentEvent.Type()
	//log.Println("current event type:", event_type)
	if !currentEvent.HasRanking() {
		return "", ErrEventType
	}
	if !currentEvent.RankingAvailable() {
		return "", ErrRankingNA
	}

	//localtime := float64(time.Now().UnixNano()) / 1e9 // for debug
	local_timestamp := GetLocalTimestamp()
	dirname := df.rank_cache_dir + local_timestamp + "/"
	path := dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
	if Exists(path) {
		// cache hit
		//log.Println("hit", local_timestamp, ranking_type, page)
		return "-", nil
	} else {
		// cache miss
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	time.Sleep(1020 * time.Millisecond)
	ranking_list, servertime, err := df.GetPage(event_type, ranking_type, page)
	if err != nil {
		return "", err
	}
	//log.Printf("localtime: %f servertime: %d lag: %f\n", localtime, servertime, float64(servertime)-localtime)

	server_timestamp_i := RoundTimestamp(time.Unix(int64(servertime), 0)).Unix()
	server_timestamp := fmt.Sprintf("%d", server_timestamp_i)

	if server_timestamp != local_timestamp {
		log.Println("{NOTICE} change to server:", server_timestamp, "local:", local_timestamp)
		dirname = df.rank_cache_dir + server_timestamp + "/"
		path = dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	//log.Println("write to path", path)
	lockfile := dirname + "lock"
	ioutil.WriteFile(lockfile, []byte(""), 0644)
	DumpToFile(ranking_list, path)
	os.Remove(lockfile)
	return "*", nil
}

func (df *DataFetcher) GetPage(event_type, ranking_type, page int) ([]interface{}, uint64, error) {
	var ranking_list []interface{}
	if !df.Client.IsInitialized() {
		df.Client.LoadCheck()
	}
	// deal with atapon/medley
	var resp map[string]interface{}
	if event_type == 1 {
		resp = df.Client.GetAtaponRanking(ranking_type, page)
	} else if event_type == 3 {
		resp = df.Client.GetMedleyRanking(ranking_type, page)
	} else {
		return nil, 0, ErrEventType
	}

	servertime := resp["data_headers"].(map[interface{}]interface{})["servertime"].(uint64)
	err := df.Client.ParseResultCode(resp)
	if err != nil {
		return nil, servertime, err
	}
	//log.Println("get", servertime, ranking_type, page)
	ranking_list = resp["data"].(map[interface{}]interface{})["ranking_list"].([]interface{})
	return ranking_list, servertime, err
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
