package apiclient

import (
	"encoding/base64"
	"fmt"
	//"gopkg.in/vmihailenco/msgpack.v2"
	"gopkg.in/yaml.v2"
	"strings"
)

func DecodeBody(body []byte, msg_iv string) {
	resp_body := body
	// remove extra tabs
	resp_body = []byte(strings.Replace(string(resp_body), "\t", "", -1))
	resp_body = []byte(strings.Replace(string(resp_body), " ", "", -1))

	reply := make([]byte, base64.StdEncoding.DecodedLen(len(resp_body)))
	n, _ := base64.StdEncoding.Decode(reply, resp_body)
	//print("written", n, "\n")
	//print("replylen", len(reply), "\n")
	reply = reply[:n]
	//fmt.Println("reply", reply)

	msg_iv = strings.Replace(msg_iv, "-", "", -1)
	//fmt.Println("key", string(reply[len(reply)-32:]))
	plain2 := Decrypt_cbc(reply[:len(reply)-32], []byte(msg_iv), reply[len(reply)-32:])
	//fmt.Println("plain2", string(plain2))

	mp := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
	n, _ = base64.StdEncoding.Decode(mp, plain2)
	mp = mp[:n]
	//fmt.Print("mp is\n", hex.Dump(mp))
	//var content map[string]interface{}
	var content interface{}
	MsgpackDecode(mp, &content)

	yy, _ := yaml.Marshal(content)
	_ = yy
	fmt.Printf("%#v\n", content)
	fmt.Println("content", string(yy))
}
