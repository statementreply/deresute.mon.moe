package apiclient

import (
	"encoding/base64"
	//"gopkg.in/vmihailenco/msgpack.v2"
	//"gopkg.in/yaml.v2"
	"strings"
	"log"
)

func DecodeBody(body []byte, msg_iv string) interface{} {
	var content interface{}
	resp_body := body
	// remove extra tabs
	resp_body = []byte(strings.Replace(string(resp_body), "\t", "", -1))
	resp_body = []byte(strings.Replace(string(resp_body), " ", "", -1))

	reply := make([]byte, base64.StdEncoding.DecodedLen(len(resp_body)))
	n, _ := base64.StdEncoding.Decode(reply, resp_body)
	// trim to n
	reply = reply[:n]

	msg_iv = strings.Replace(msg_iv, "-", "", -1)
	//fmt.Println("key", string(reply[len(reply)-32:]))
	log.Println("replylen", len(reply))
	// FIXME: short body, return nil?
	if len(reply) < 32 {
		return content
	}
	plain2 := Decrypt_cbc(reply[:len(reply)-32], []byte(msg_iv), reply[len(reply)-32:])

	mp := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
	n, _ = base64.StdEncoding.Decode(mp, plain2)
	mp = mp[:n]
	MsgpackDecode(mp, &content)

	return content
}
