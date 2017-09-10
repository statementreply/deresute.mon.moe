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

package apiclient

import (
	// golang core libs
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
	// external libs
	// depends on rijndael by agl (embedded)
	//"rijndael_wrapper"
	// msgpack/yaml/json libs
	// msgpack new spec only "gopkg.in/vmihailenco/msgpack.v2"
	// msgpack old spec      "github.com/ugorji/go-msgpack"
	// good updated msgpack lib (with a different API)
	// msgpack both specs supported
	//"github.com/ugorji/go/codec"
	"gopkg.in/yaml.v2"
)

// FIXME: android client starts to use https
const BASE string = "http://game.starlight-stage.jp"

var ErrSession = errors.New("session error, (need to restart session)")
var ErrResource = errors.New("need to update res_ver")
var ErrOveruse = errors.New("too many requests")
var ErrData = errors.New("data error str8?")
var ErrEventClose = errors.New("event closed")
var ErrUnknown = errors.New("unknown error")
var ErrDataHeaders = errors.New("no data_headers")
var ErrDataHeadersBad = errors.New("bad data_headers")
var ErrAppVer = errors.New("app_ver outdated")

type ApiClient struct {
	// constant after constructor
	app_ver       string
	user          int32
	viewer_id     int32
	viewer_id_str string
	udid          string
	msg_iv        []byte
	timezone      string
	VIEWER_ID_KEY []byte
	SID_KEY       []byte

	// need lock
	sid     string
	res_ver string
	// true: no need to call LoadCheck
	// false: need to call LoadCheck?
	initialized bool

	lock sync.RWMutex

	// for reuse, concurrency safe
	httpclient *http.Client
}

func NewApiClient(user, viewer_id int32, udid, app_ver, res_ver string, VIEWER_ID_KEY, SID_KEY []byte) *ApiClient {
	rand.Seed(time.Now().UnixNano())
	client := new(ApiClient)
	client.app_ver = app_ver
	client.user = user
	client.viewer_id = viewer_id
	client.viewer_id_str = fmt.Sprintf("%d", viewer_id)
	client.udid = udid
	client.timezone = "09:00:00" // version 2.1.0 new
	client.VIEWER_ID_KEY = VIEWER_ID_KEY
	client.SID_KEY = SID_KEY

	client.msg_iv = []byte(strings.Replace(client.udid, "-", "", -1))
	client.res_ver = res_ver
	//client.sid = ""
	//client.initialized = false
	client.Reset_sid()
	client.httpclient = &http.Client{Timeout: 20 * time.Second}
	return client
}

// initialize sid
// with lock (!)
func (client *ApiClient) Reset_sid() {
	client.lock.Lock()
	client.sid = client.viewer_id_str + client.udid
	// change to true?
	client.initialized = false
	client.lock.Unlock()
}

func NewApiClientFromConfig(configFile string) *ApiClient {
	secret, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal("read config file", err)
	}
	var secret_dict map[string]interface{}
	err = yaml.Unmarshal(secret, &secret_dict)
	if err != nil {
		log.Fatal("parse config file", err)
	}
	//fmt.Println(secret_dict)

	return NewApiClient(
		int32(secret_dict["user"].(int)),
		int32(secret_dict["viewer_id"].(int)),
		secret_dict["udid"].(string),
		secret_dict["app_ver"].(string),
		secret_dict["res_ver"].(string),
		[]byte(secret_dict["VIEWER_ID_KEY"].(string)),
		[]byte(secret_dict["SID_KEY"].(string)))
}

// with lock(!)
func (client *ApiClient) Call(path string, args map[string]interface{}) map[string]interface{} {
	// prevent concurrent calls
	client.lock.Lock()
	defer client.lock.Unlock()

	// Prepare request body
	body, plain_tmp := client.EncodeBody(args)
	// Request body finished

	// Prepare request header
	req := client.makeRequest(path, body, plain_tmp)

	// Do request
	//hclient := &http.Client{}
	resp, err := client.httpclient.Do(req)
	if err != nil {
		log.Println("http request error", err)
		return nil
	}

	// Processing response
	if resp.Body == nil {
		log.Println("resp.Body is nil")
		return nil
	}

	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Read resp.Body", err)
		return nil
	}
	resp.Body.Close()

	//var content map[string]interface{}
	content := DecodeBody(resp_body, string(client.msg_iv))

	data_headers, ok := content["data_headers"]
	if ok {
		new_sid, ok := (data_headers.(map[interface{}]interface{}))["sid"]
		if ok && (new_sid != "") {
			//fmt.Println("get new sid", new_sid)
			client.sid = new_sid.(string)
			client.initialized = true
		}
	} else {
		// FIXME: should not happen?
		log.Println("[ERROR] no data_headers in response")
	}
	return content
}

// NOTICE: result_code can be int64 or uint64
func (client *ApiClient) GetResultCode(content map[string]interface{}) (interface{}, error) {
	var result_code interface{}
	data_headers, ok := content["data_headers"]
	if ok {
		data_headers_map, ok := data_headers.(map[interface{}]interface{})
		if !ok {
			return int64(-1), ErrDataHeadersBad
		}
		result_code = data_headers_map["result_code"]
	} else {
		return int64(-1), ErrDataHeaders
	}
	return result_code, nil
}

func (client *ApiClient) ParseResultCode(content map[string]interface{}) error {
	result_code, err := client.GetResultCode(content)
	if err != nil {
		return err
	}
	switch r := result_code.(type) {
	case uint64:
		// good for now
		//log.Println("result_code is uint64", result_code)
	case int64:
		// convert to uint64
		if r != 1 {
			log.Println("result_code is not 1")
			log.Println("result_code is int64", result_code)
			result_code = interface{}(uint64(99999)) // ErrUnknown
		} else {
			result_code = interface{}(uint64(1))
		}
	default:
		log.Println("result_code is some other type?", result_code)
		result_code = interface{}(uint64(99999)) // ErrUnknown
	}
	switch result_code.(uint64) {
	case 1:
		return nil
	case 201: // session error
		return ErrSession
	case 204:
		return ErrAppVer
	case 13001, 11001:
		//ERROR_CODE_MEDLEY_CLOSE            //ERROR_CODE_ATAPON_CLOSE
		return ErrEventClose
	case 214:
		return ErrResource
	case 208:
		return ErrOveruse
	case 209:
		return ErrData
	default:
		log.Println("response body:", content)
		return ErrUnknown
	}
}

func (client *ApiClient) LoadCheck() {
	sum_tmp := md5.Sum([]byte("All your APIs are belong to us")) // some string...
	args := map[string]interface{}{
		"campaign_data": "",
		"campaign_user": 171780, // FIXME: some arbitrary uid...
		"campaign_sign": hex.EncodeToString(sum_tmp[:]),
		"app_type":      0,
	}

	check := client.Call("/load/check", args)
	if check == nil {
		log.Println("LoadCheck Failed")
		log.Print(check)
		return
	}
	log.Println("[INFO] debug load_check", check)
	// interface conversion / type assertion
	data_headers, ok := check["data_headers"].(map[interface{}]interface{})
	if !ok {
		log.Println("data_header type incorrect")
		log.Print(check)
		return
	}
	new_res_ver, ok := data_headers["required_res_ver"]
	if ok {
		s := new_res_ver.(string)
		client.Set_res_ver(s)
		fmt.Println("Update res_ver to", s)
		time.Sleep(1.3e9) // nanosecond
		check := client.Call("/load/check", args)
		log.Print("check again", check)
	}
}

func (client *ApiClient) GetProfile(friend_id int) map[string]interface{} {
	return client.Call("/profile/get_profile", map[string]interface{}{"friend_id": friend_id})
}

func (client *ApiClient) GetMedleyRanking(rankingType int, page int) map[string]interface{} {
	r1 := client.Call("/event/medley/ranking_list", map[string]interface{}{"ranking_type": rankingType, "page": page})
	return r1
}

func (client *ApiClient) GetAtaponRanking(rankingType int, page int) map[string]interface{} {
	r1 := client.Call("/event/atapon/ranking_list", map[string]interface{}{"ranking_type": rankingType, "page": page})
	return r1
}

func (client *ApiClient) GetTourRanking(rankingType int, page int) map[string]interface{} {
	r1 := client.Call("/event/tour/ranking_list", map[string]interface{}{"ranking_type": rankingType, "page": page})
	return r1
}

func (client *ApiClient) GetLiveDetailRanking(live_detail_id, page int) map[string]interface{} {
	return client.Call("/live/get_live_detail_ranking",
		map[string]interface{}{"live_detail_id": live_detail_id, "page": page})
}

//Req URL: game.starlight-stage.jp /live/get_live_detail_ranking 192.168.0.3->203.104.249.195 55234->80
//map[live_detail_id:162 page:1 viewer_id:28577727288451868527518831476546X6+XO8CAOsHM8aDp7/pvhM8RrXdP2ztPtyLaaUqegrU=]

// with lock(!)
func (client *ApiClient) IsInitialized() bool {
	client.lock.RLock()
	ret := client.initialized
	client.lock.RUnlock()
	return ret
}
