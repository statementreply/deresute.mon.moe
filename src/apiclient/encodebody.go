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
//
// Ported from the Python implementation in deresute.me
//     <https://github.com/marcan/deresuteme>
//     Copyright 2016-2017 Hector Martin <marcan@marcan.st>
//     Licensed under the Apache License, Version 2.0

package apiclient

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	//"strings"
)

func gen_vid_iv() string {
	var vid_iv string
	// vid_iv is \d{32}
	vid_iv_byte := make([]byte, 16)
	n, err := rand.Read(vid_iv_byte)
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
	n, err := rand.Read(key_tmp)
	if (err != nil) || (n != 64) {
		log.Fatal("crypto/rand.Read", err)
	}
	key = []byte(base64.StdEncoding.EncodeToString(key_tmp))
	// trim to 32 bytes
	key = key[:32]
	return key
}

func (client *ApiClient) EncodeBody(args map[string]interface{}) (string, string) {
	// Prepare request body
	var body string
	vid_iv := gen_vid_iv()
	//log.Fatal(vid_iv, " ", len(vid_iv))
	args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))
	args["timezone"] = client.timezone
	mp := MsgpackEncode(args)
	plain_tmp := base64.StdEncoding.EncodeToString(mp)

	key := gen_key()

	body_tmp := Encrypt_cbc([]byte(plain_tmp), client.msg_iv, key)
	body = base64.StdEncoding.EncodeToString([]byte(string(body_tmp) + string(key)))
	// Request body finished
	return body, plain_tmp
}
