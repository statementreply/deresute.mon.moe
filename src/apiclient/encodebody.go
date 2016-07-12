package apiclient

import (
	crand "crypto/rand"
	"log"
	"math/big"
	"fmt"
	"encoding/base64"
	"strings"
)


func (client ApiClient) EncodeBody(args map[string]interface{}) string {
	// Prepare request body
	// vid_iv is \d{32}
	vid_iv_byte := make([]byte, 16)
	n, err := crand.Read(vid_iv_byte)
	if (n != 16) || (err != nil) {
		log.Fatal("rand err", n, err)
	}
	var vid_iv_big big.Int
	vid_iv_big.SetBytes(vid_iv_byte)
	vid_iv_string := fmt.Sprintf("%032d", &vid_iv_big)
	vid_iv := vid_iv_string[len(vid_iv_string)-32:]
	//log.Fatal(vid_iv, " ", len(vid_iv))

	args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))

	mp := MsgpackEncode(args)
	plain := base64.StdEncoding.EncodeToString(mp)

	key_tmp := make([]byte, 64)
	_, _ = crand.Read(key_tmp)
	key := []byte(base64.StdEncoding.EncodeToString(key_tmp))
	// trim to 32 bytes
	key = key[:32]

	msg_iv := []byte(strings.Replace(client.udid, "-", "", -1))
	body_tmp := Encrypt_cbc([]byte(plain), msg_iv, key)
	body := base64.StdEncoding.EncodeToString([]byte(string(body_tmp) + string(key)))
	// Request body finished
	return body
}

func (client ApiClient) EncodeBody2(args map[string]interface{}) string {
	var body string
	var vid_iv string
	// vid_iv is \d{32}
	vid_iv_byte := make([]byte, 16)
	n, err := crand.Read(vid_iv_byte)
	if (n != 16) || (err != nil) {
		log.Fatal("rand err", n, err)
	}
	var vid_iv_big big.Int
	vid_iv_big.SetBytes(vid_iv_byte)
	vid_iv_string := fmt.Sprintf("%032d", &vid_iv_big)
	vid_iv = vid_iv_string[len(vid_iv_string)-32:]
	//log.Fatal(vid_iv, " ", len(vid_iv))
	args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))
	mp := MsgpackEncode(args)
	client.plain = base64.StdEncoding.EncodeToString(mp)

	key_tmp := make([]byte, 64)
	_, _ = crand.Read(key_tmp)
	key := []byte(base64.StdEncoding.EncodeToString(key_tmp))
	// trim to 32 bytes
	key = key[:32]

	body_tmp := Encrypt_cbc([]byte(client.plain), client.msg_iv, key)
	body = base64.StdEncoding.EncodeToString([]byte(string(body_tmp) + string(key)))
	return body
}

func gen_vid_iv() string {
	var vid_iv string
	// vid_iv is \d{32}
	vid_iv_byte := make([]byte, 16)
	n, err := crand.Read(vid_iv_byte)
	if (n != 16) || (err != nil) {
		log.Fatal("rand err", n, err)
	}
	var vid_iv_big big.Int
	vid_iv_big.SetBytes(vid_iv_byte)
	vid_iv_string := fmt.Sprintf("%032d", &vid_iv_big)
	vid_iv = vid_iv_string[len(vid_iv_string)-32:]
	return vid_iv
}

func gen_key() []byte {
	var key []byte
	key_tmp := make([]byte, 64)
	_, _ = crand.Read(key_tmp)
	key = []byte(base64.StdEncoding.EncodeToString(key_tmp))
	// trim to 32 bytes
	key = key[:32]
	return key
}

func (client ApiClient) EncodeBody3(args map[string]interface{}) string {
	var body string
	vid_iv := gen_vid_iv()
	//log.Fatal(vid_iv, " ", len(vid_iv))
	args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))
	mp := MsgpackEncode(args)
	client.plain = base64.StdEncoding.EncodeToString(mp)

	key := gen_key()

	body_tmp := Encrypt_cbc([]byte(client.plain), client.msg_iv, key)
	body = base64.StdEncoding.EncodeToString([]byte(string(body_tmp) + string(key)))
	return body
}
