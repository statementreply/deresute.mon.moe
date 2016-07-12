package apiclient

import (
	// golang core libs
	"crypto/cipher"
	"crypto/md5"
	//crand "crypto/rand"
	"crypto/sha1"
	//"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	//"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
	// external libs
	// depends on rijndael by agl (embedded)
	"rijndael_wrapper"
	// msgpack/yaml/json libs
	// msgpack new spec only "gopkg.in/vmihailenco/msgpack.v2"
	// msgpack old spec      "github.com/ugorji/go-msgpack"
	// good updated msgpack lib (with a different API)
	// msgpack both specs supported
	"github.com/ugorji/go/codec"
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
	plain         string
}

func Lolfuscate(s string) string {
	var r string
	r = ""
	r += fmt.Sprintf("%04x", len(s))
	for i := 0; i < len(s); i++ {
		r += fmt.Sprintf("%02d", rand.Intn(100))
		r += string(s[i] + 10)
		r += fmt.Sprintf("%01d", rand.Intn(10))
	}
	r += fmt.Sprintf("%016d%016d", rand.Int63n(1e16), rand.Int63n(1e16))
	return r
}

func Unlolfuscate(s string) string {
	var r string
	r = ""
	r_len, _ := strconv.ParseInt(s[:4], 16, 16)
	//fmt.Println("rlen", int(r_len))
	for i := 6; (i < len(s)) && (len(r) < int(r_len)); i += 4 {
		r += string(s[i] - 10)
	}
	return r
}

func Decrypt_cbc(s, iv, key []byte) []byte {
	/*fmt.Println(hex.Dump(s))
	fmt.Println(hex.Dump(iv))
	fmt.Println(hex.Dump(key))*/
	s_len := len(s)
	s_new := s
	if s_len%32 != 0 {
		s_new = make([]byte, s_len+32-(s_len%32))
		copy(s_new, s)
	}
	c, _ := rijndael_wrapper.NewCipher(key)
	bm := cipher.NewCBCDecrypter(c, iv)
	dst := make([]byte, len(s_new))
	bm.CryptBlocks(dst, s_new)
	return dst
}

func Encrypt_cbc(s, iv, key []byte) []byte {
	s_len := len(s)
	s_new := s
	if s_len%32 != 0 {
		s_new = make([]byte, s_len+32-(s_len%32))
		copy(s_new, s)
	}
	c, _ := rijndael_wrapper.NewCipher(key)
	bm := cipher.NewCBCEncrypter(c, iv)
	dst := make([]byte, len(s_new))
	bm.CryptBlocks(dst, s_new)
	return dst
}

func NewApiClient(user, viewer_id int32, udid, res_ver string, VIEWER_ID_KEY, SID_KEY []byte) *ApiClient {
	client := new(ApiClient)
	client.user = user
	client.viewer_id = viewer_id
	client.viewer_id_str = fmt.Sprintf("%d", viewer_id)
	client.udid = udid
	client.msg_iv = []byte(strings.Replace(client.udid, "-", "", -1))
	client.res_ver = res_ver
	client.sid = ""
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
	var body string
	body = client.EncodeBody(args)
	// Request body finished

	// Prepare request header
	var sid string
	if client.sid != "" {
		sid = client.sid
	} else {
		sid = client.viewer_id_str + client.udid
	}
	param_tmp := sha1.Sum([]byte(client.udid + client.viewer_id_str + path + client.plain))
	sid_tmp := md5.Sum([]byte(sid + string(client.SID_KEY)))
	device_id_tmp := md5.Sum([]byte("Totally a real Android"))
	headers := map[string]string{
		"PARAM":           hex.EncodeToString(param_tmp[:]),
		"KEYCHAIN":        "",
		"USER_ID":         Lolfuscate(fmt.Sprintf("%d", client.user)),
		"CARRIER":         "google",
		"UDID":            Lolfuscate(client.udid),
		"APP_VER":         "2.0.3",
		"RES_VER":         client.res_ver,
		"IP_ADDRESS":      "127.0.0.1",
		"DEVICE_NAME":     "Nexus 42",
		"X-Unity-Version": "5.1.2f1",
		"SID":             hex.EncodeToString(sid_tmp[:]),
		"GRAPHICS_DEVICE_NAME": "3dfx Voodoo2 (TM)",
		"DEVICE_ID":            hex.EncodeToString(device_id_tmp[:]),
		"PLATFORM_OS_VERSION":  "Android OS 13.3.7 / API-42 (XYZZ1Y/74726f6c6c)",
		"DEVICE":               "2",
		"Content-Type":         "application/x-www-form-urlencoded", // lies
		"User-Agent":           "Dalvik/2.1.0 (Linux; U; Android 13.3.7; Nexus 42 Build/XYZZ1Y)",
		"Accept-Encoding":      "identity",
		"Connection":           "close",
	}
	// Request header ready

	// Prepare Request struct
	// req.body is ReadCloser
	req, _ := http.NewRequest("POST", BASE+path, ioutil.NopCloser(strings.NewReader(body)))
	for k := range headers {
		req.Header.Set(k, headers[k])
		// not needed
		//req.Header.Set(http.CanonicalHeaderKey(k), headers[k])
	}
	req.Close = true

	// Do request
	hclient := &http.Client{}
	resp, _ := hclient.Do(req)

	// Processing response
	resp_body, _ := ioutil.ReadAll(resp.Body)

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

func (client *ApiClient) Set_res_ver(res_ver string) {
	client.res_ver = res_ver
}

func MsgpackDecode(b []byte, v interface{}) {
	var bh codec.MsgpackHandle
	bh.RawToString = true
	//log.Fatal(fmt.Printf("%V\n%#v\n%t\n%T\n", bh, bh, bh, bh))
	dec := codec.NewDecoderBytes(b, &bh)
	err := dec.Decode(v)
	//log.Printf("msgpackDecode\n%s\n", hex.Dump(b))
	if err != nil {
		// FIXME: ignore error?
		log.Println("msgpack decode", err)
	}
}

func MsgpackEncode(v interface{}) []byte {
	var bh codec.MsgpackHandle
	// canonicalize map key order
	//bh.Canonical = true
	// server doesn't support str8
	bh.WriteExt = false

	var b []byte
	enc := codec.NewEncoderBytes(&b, &bh)
	err := enc.Encode(v)
	if err != nil {
		log.Fatal("msgpack encode", err)
	}
	return b
}

func Test1() {
	var args map[string]interface{}
	//var content map[string]interface{}
	var content2 map[string]interface{}
	args = make(map[string]interface{})
	fmt.Println("here")
	args["1"] = 2
	args["2"] = "string"
	args["c"] = map[string]int{"c92": 12}
	fmt.Println("here2")
	// old lib
	// don't use
	//mp, _ := msgpack.Marshal(args)
	//msgpack.Unmarshal(mp, &content, nil)
	//fmt.Println(args, content)

	// new lib
	mp2 := MsgpackEncode(args)
	MsgpackDecode(mp2, &content2)
	fmt.Println(args)
	fmt.Println(content2)
	//fmt.Println(mp)
	fmt.Println(mp2)
	return
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

func (client *ApiClient) GetPage(rankingType int, page int) map[string]interface{} {
	r1 := client.Call("/event/medley/ranking_list", map[string]interface{}{"ranking_type": rankingType, "page": page})
	return r1
}
