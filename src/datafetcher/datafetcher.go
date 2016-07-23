package datafetcher

import (
	"apiclient"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type DataFetcher struct {
	client         *apiclient.ApiClient
	key_point      [][2]int
	rank_cache_dir string
}

func NewDataFetcher(client *apiclient.ApiClient, key_point [][2]int, rank_cache_dir string) *DataFetcher {
	log.Println("NewDataFetcher()")
	df := new(DataFetcher)

	df.client = client
	//client.LoadCheck()
	df.key_point = key_point
	df.rank_cache_dir = rank_cache_dir

	log.Println(GetLocalTimestamp())
	log.Println(RoundTimestamp(time.Now()).String())
	return df
}

func (df *DataFetcher) Run() error {
	for _, key := range df.key_point {
		log.Println(key)
		err := df.GetCache(key[0], RankToPage(key[1]))
		if err != nil {
			//log.Fatal(err)
			return err
		}
	}
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

func (df *DataFetcher) GetCache(ranking_type int, page int) error {
	localtime := float64(time.Now().UnixNano()) / 1e9
	local_timestamp := GetLocalTimestamp()
	dirname := df.rank_cache_dir + local_timestamp + "/"
	path := dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
	if Exists(path) {
		// cache hit
		return nil
	} else {
		// cache miss
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	time.Sleep(11 * 100 * 1000 * 1000)
	ranking_list, servertime, err := df.GetPage(ranking_type, page)
	if err != nil {
		// FIXME
		return err
	}
	log.Printf("localtime: %f servertime: %d lag: %f\n", localtime, servertime, float64(servertime)-localtime)
	server_timestamp_i := RoundTimestamp(time.Unix(int64(servertime), 0)).Unix()
	server_timestamp := fmt.Sprintf("%d", server_timestamp_i)

	if server_timestamp != local_timestamp {
		log.Println("server:", server_timestamp, "local:", local_timestamp)
		dirname = df.rank_cache_dir + server_timestamp + "/"
		path = dirname + fmt.Sprintf("r%02d.%06d", ranking_type, page)
		if !Exists(dirname) {
			os.Mkdir(dirname, 0755)
		}
	}
	log.Println("write to path", path)
	lockfile := dirname + "lock"
	ioutil.WriteFile(lockfile, []byte(""), 0644)
	DumpToFile(ranking_list, path)
	os.Remove(lockfile)
	//DumpToStdout(ranking_list)
	//fmt.Println(ranking_list)
	return nil
}

func (df *DataFetcher) GetPage(ranking_type, page int) ([]interface{}, uint64, error) {
	var ranking_list []interface{}
	if !df.client.IsInitialized() {
		df.client.LoadCheck()
	}
	resp := df.client.GetAtaponRanking(ranking_type, page)
	servertime := resp["data_headers"].(map[interface{}]interface{})["servertime"].(uint64)
	err := df.client.ParseResultCode(resp)
	if err != nil {
		return nil, servertime, err
	}
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
