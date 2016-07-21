package apiclient

import (
	// golang core libs
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
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

const BASE string = "http://game.starlight-stage.jp"

type ApiClient struct {
	user          int32
	viewer_id     int32
	viewer_id_str string
	udid          string
	sid           string
	res_ver       string
	VIEWER_ID_KEY []byte
	SID_KEY       []byte
	msg_iv        []byte
	// holds plaintext temporarily
	plain string
}

func NewApiClient(user, viewer_id int32, udid, res_ver string, VIEWER_ID_KEY, SID_KEY []byte) *ApiClient {
	rand.Seed(time.Now().UnixNano())
	client := new(ApiClient)
	client.user = user
	client.viewer_id = viewer_id
	client.viewer_id_str = fmt.Sprintf("%d", viewer_id)
	client.udid = udid
	client.msg_iv = []byte(strings.Replace(client.udid, "-", "", -1))
	client.res_ver = res_ver
	//client.sid = ""
	// initial sid
	client.sid = client.viewer_id_str + client.udid
	client.VIEWER_ID_KEY = VIEWER_ID_KEY
	client.SID_KEY = SID_KEY
	return client
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
		secret_dict["res_ver"].(string),
		[]byte(secret_dict["VIEWER_ID_KEY"].(string)),
		[]byte(secret_dict["SID_KEY"].(string)))
}

func (client *ApiClient) Call(path string, args map[string]interface{}) map[string]interface{} {
	// Prepare request body
	body := client.EncodeBody(args)
	// Request body finished

	// Prepare request header
	req := client.MakeRequest(path, body)

	// Do request
	hclient := &http.Client{}
	resp, _ := hclient.Do(req)

	// Processing response
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Read resp.Body", err)
	}

	//var content map[string]interface{}
	content := DecodeBody(resp_body, string(client.msg_iv))

	data_headers, ok := content["data_headers"]
	if ok {
		new_sid, ok := (data_headers.(map[interface{}]interface{}))["sid"]
		if ok && (new_sid != "") {
			//fmt.Println("get new sid", new_sid)
			client.sid = new_sid.(string)
		}
	} else {
		// FIXME
		log.Println("no data_headers in response")
	}
	return content
}

func (client *ApiClient) GetResultCode(content map[string]interface{}) int64 {
	var result_code int64
	data_headers, ok := content["data_headers"]
	if ok {
		result_code = data_headers.(map[interface{}]interface{})["result_code"].(int64)
	} else {
		// FIXME
	}
	return result_code
}

func (client *ApiClient) LoadCheck() {
	sum_tmp := md5.Sum([]byte("All your APIs are belong to us"))
	args := map[string]interface{}{"campaign_data": "",
		"campaign_user": 171780,
		"campaign_sign": hex.EncodeToString(sum_tmp[:]),
		"app_type":      0}

	check := client.Call("/load/check", args)
	log.Print(check)
	new_res_ver, ok := check["data_headers"].(map[interface{}]interface{})["required_res_ver"]
	if ok {
		s := new_res_ver.(string)
		client.Set_res_ver(s)
		fmt.Println("Update res_ver to ", s)
		time.Sleep(1.3e9) // nanosecond
		check := client.Call("/load/check", args)
		log.Print("check again ", check)
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

func (client *ApiClient) GetLiveDetailRanking(live_detail_id, page int) map[string]interface{} {
	return client.Call("/live/get_live_detail_ranking",
		map[string]interface{}{"live_detail_id": live_detail_id, "page": page})
}

//Req URL: game.starlight-stage.jp /live/get_live_detail_ranking 192.168.0.3->203.104.249.195 55234->80
//map[live_detail_id:162 page:1 viewer_id:28577727288451868527518831476546X6+XO8CAOsHM8aDp7/pvhM8RrXdP2ztPtyLaaUqegrU=]
