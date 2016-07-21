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

func (client *ApiClient) Set_res_ver(res_ver string) {
	client.res_ver = res_ver
}

func (client *ApiClient) Get_res_ver() string {
	return client.res_ver
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
