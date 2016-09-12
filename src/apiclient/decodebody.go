package apiclient

import (
	"encoding/base64"
	"encoding/hex"
	//"gopkg.in/vmihailenco/msgpack.v2"
	//"gopkg.in/yaml.v2"
	"bytes"
	"log"
	"runtime/debug"
	"strings"
)

// Processing response (or request)
func DecodeBody(body []byte, msg_iv string) map[string]interface{} {
	var content map[string]interface{}
	// NOTE: remove extra tabs/spaces for base64
	body = bytes.Replace(body, []byte("\t"), nil, -1)
	body = bytes.Replace(body, []byte(" "), nil, -1)

	reply := make([]byte, base64.StdEncoding.DecodedLen(len(body)))
	n, err := base64.StdEncoding.Decode(reply, body)
	if err != nil {
		log.Println("base64 Decode", hex.Dump(body), err)
	}
	// trim to n
	reply = reply[:n]

	msg_iv = strings.Replace(msg_iv, "-", "", -1)
	//fmt.Println("key", string(reply[len(reply)-32:]))
	//log.Println("replylen", len(reply))

	// NOTE: short body, return nil
	if len(reply) < 32 {
		return content
	}
	plain2 := Decrypt_cbc(reply[:len(reply)-32], []byte(msg_iv), reply[len(reply)-32:])
	// NOTE: trim NULs at the end of plain2
	plain2 = bytes.Replace(plain2, []byte("\000"), nil, -1)

	mp := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
	n, err = base64.StdEncoding.Decode(mp, plain2)
	if err != nil {
		// too long
		//log.Println(hex.Dump(plain2))
		debug.PrintStack()
		log.Println("base64 Decode", err)
	}
	mp = mp[:n]
	MsgpackDecode(mp, &content)

	return content
}
