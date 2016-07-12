package apiclient

import (
	"encoding/base64"
	//"gopkg.in/vmihailenco/msgpack.v2"
	//"gopkg.in/yaml.v2"
	"strings"
	"log"
)

// Processing response (or request)
func DecodeBody(body []byte, msg_iv string) map[string]interface{} {
	var content map[string]interface{}
	// remove extra tabs
	body = []byte(strings.Replace(string(body), "\t", "", -1))
	body = []byte(strings.Replace(string(body), " ", "", -1))

	reply := make([]byte, base64.StdEncoding.DecodedLen(len(body)))
	n, _ := base64.StdEncoding.Decode(reply, body)
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
