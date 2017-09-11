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
	"crypto/cipher"
	"fmt"
	"github.com/ugorji/go/codec"
	"log"
	"math/rand"
	"rijndael_wrapper"
	"strconv"
)

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
	r_len, err := strconv.ParseInt(s[:4], 16, 16)
	if err != nil {
		log.Println("Unlolfuscate():", err)
		return ""
	}
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
	c, err := rijndael_wrapper.NewCipher(key)
	if err != nil {
		log.Println(err)
		return nil
	}
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
	c, err := rijndael_wrapper.NewCipher(key)
	if err != nil {
		log.Println(err)
		return nil
	}
	bm := cipher.NewCBCEncrypter(c, iv)
	dst := make([]byte, len(s_new))
	bm.CryptBlocks(dst, s_new)
	return dst
}

// with lock(!)
func (client *ApiClient) Set_res_ver(res_ver string) {
	client.lock.Lock()
	client.res_ver = res_ver
	client.lock.Unlock()
}

// with lock(!)
func (client *ApiClient) Get_res_ver() string {
	client.lock.RLock()
	val := client.res_ver
	client.lock.RUnlock()
	return val
}

func (client *ApiClient) GetResVer() string {
	return client.Get_res_ver()
}

func (client *ApiClient) SetResVer(res_ver string) {
	client.Set_res_ver(res_ver)
}

func (client *ApiClient) GetAppVer() string {
	client.lock.RLock()
	app_ver := client.app_ver
	client.lock.RUnlock()
	return app_ver
}

func (client *ApiClient) SetAppVer(app_ver string) {
	client.lock.Lock()
	client.app_ver = app_ver
	client.lock.Unlock()
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
