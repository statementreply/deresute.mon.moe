package apiclient
import (
    // golang core libs
    "fmt"
    "strconv"
    "strings"
    //"os"
    "log"
    "io/ioutil"
    "math/rand"
    "math/big"
    crand "crypto/rand"
    "crypto/md5"
    "crypto/sha1"
    "crypto/cipher"
    "encoding/base64"
    "encoding/hex"
    "net/http"
    // external libs
    // depends on rijndael by agl (embedded)
    "rijndael_wrapper"
    // msgpack/yaml/json libs
    // buggy "gopkg.in/vmihailenco/msgpack.v2"
    // good, deprecated, msgpack "github.com/ugorji/go-msgpack"
    // good updated msgpack lib (with a different API)
    "github.com/ugorji/go/codec"
    //"gopkg.in/yaml.v2"
)

const BASE string = "http://game.starlight-stage.jp"

type ApiClient struct {
    user int32
    viewer_id int32
    viewer_id_str string
    udid string
    sid string
    res_ver string
    VIEWER_ID_KEY []byte
    SID_KEY []byte
}

func Lolfuscate(s string) string {
    var r string
    r = ""
    r += fmt.Sprintf("%04x", len(s))
    for i := 0; i < len(s); i++ {
        r += fmt.Sprintf("%02d", rand.Intn(100))
        r += string(s[i]+10)
        r += fmt.Sprintf("%01d", rand.Intn(10))
    }
    r += fmt.Sprintf("%016d%016d", rand.Int63n(1e16), rand.Int63n(1e16))
    return r
}

func Unlolfuscate(s string) string {
    var r string
    r = ""
    r_len, _ := strconv.ParseInt(s[:4], 16, 16)
    fmt.Println("rlen", int(r_len))
    for i := 6; (i < len(s)) && (len(r) < int(r_len)); i += 4 {
        r += string(s[i]-10)
    }
    return r
}

func Decrypt_cbc(s, iv, key []byte) []byte {
    s_len := len(s)
    s_new := s
    if s_len % 32 != 0 {
        s_new = make([]byte, s_len + 32 - (s_len%32))
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
    if s_len % 32 != 0 {
        s_new = make([]byte, s_len + 32 - (s_len%32))
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
    client.res_ver = res_ver
    client.sid = ""
    client.VIEWER_ID_KEY = VIEWER_ID_KEY
    fmt.Println(len(VIEWER_ID_KEY))
    client.SID_KEY = SID_KEY
    return client
}

func (client *ApiClient) Call(path string, args map[string]interface{}) map[string]interface{} {
    // Prepare request body
    // vid_iv is \d{32}
    vid_iv_byte := make([]byte, 16)
    n, err := crand.Read(vid_iv_byte)
    if (n != 16) || (err != nil) {
        log.Fatal(n, err)
    }
    var vid_iv_big big.Int
    vid_iv_big.SetBytes(vid_iv_byte)
    vid_iv_string := fmt.Sprintf("%032d", &vid_iv_big)
    vid_iv := vid_iv_string[len(vid_iv_string)-32:]
    //log.Fatal(vid_iv, " ", len(vid_iv))

    args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))

    mp := msgpackEncode(args)
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

    // Prepare request header
    var sid string
    if client.sid != "" {
        sid = client.sid
    } else {
        sid = client.viewer_id_str + client.udid
    }
    param_tmp := sha1.Sum([]byte(client.udid + client.viewer_id_str + path + plain))
    sid_tmp := md5.Sum([]byte(sid + string(client.SID_KEY)))
    device_id_tmp := md5.Sum([]byte("Totally a real Android"))
    headers := map[string]string{
        "PARAM": hex.EncodeToString(param_tmp[:]),
        "KEYCHAIN": "",
        "USER_ID": Lolfuscate(fmt.Sprintf("%d", client.user)),
        "CARRIER": "google",
        "UDID": Lolfuscate(client.udid),
        "APP_VER": "2.0.3",
        "RES_VER": client.res_ver,
        "IP_ADDRESS": "127.0.0.1",
        "DEVICE_NAME": "Nexus 42",
        "X-Unity-Version": "5.1.2f1",
        "SID": hex.EncodeToString(sid_tmp[:]),
        "GRAPHICS_DEVICE_NAME": "3dfx Voodoo2 (TM)",
        "DEVICE_ID": hex.EncodeToString(device_id_tmp[:]),
        "PLATFORM_OS_VERSION": "Android OS 13.3.7 / API-42 (XYZZ1Y/74726f6c6c)",
        "DEVICE": "2",
        "Content-Type": "application/x-www-form-urlencoded", // lies
        "User-Agent": "Dalvik/2.1.0 (Linux; U; Android 13.3.7; Nexus 42 Build/XYZZ1Y)",
        "Accept-Encoding": "identity",
        "Connection": "close",
    }
    // Request header ready

    // Prepare Request struct
    // req.body is ReadCloser
    req, _ := http.NewRequest("POST", BASE + path, ioutil.NopCloser(strings.NewReader(body)))
    for k := range headers {
        req.Header.Set(k, headers[k])
        // not needed
        //req.Header.Set(http.CanonicalHeaderKey(k), headers[k])
    }
    req.Close = true

    // Do request
    hclient := &http.Client{};
    resp, _ := hclient.Do(req)

    // Processing response
    resp_body, _ := ioutil.ReadAll(resp.Body)
    reply := make([]byte, base64.StdEncoding.DecodedLen(len(resp_body)))
    n, _ = base64.StdEncoding.Decode(reply, resp_body)

    // trim NULs
    reply = reply[:n]

    plain2 := Decrypt_cbc(reply[:len(reply)-32], msg_iv, reply[len(reply)-32:])
    mp2 := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
    base64.StdEncoding.Decode(mp2, plain2)
    var content map[string]interface{}
    msgpackDecode(mp2, &content)
    data_headers, ok := content["data_headers"]
    if ok {
        new_sid, ok := (data_headers.(map[interface{}]interface{}))["sid"]
        if ok && (new_sid != "") {
            //fmt.Println("get new sid", new_sid)
            client.sid = string(new_sid.([]byte))
        }
    } else {
        log.Fatal("no data_headers in response")
    }
    return content
}


func (client *ApiClient) Set_res_ver(res_ver string) {
    client.res_ver = res_ver
}

func msgpackDecode(b []byte, v interface{}) {
    var bh codec.MsgpackHandle
    dec := codec.NewDecoderBytes(b, &bh)
    err := dec.Decode(v)
    if err != nil {
        log.Fatal(err)
    }
}

func msgpackEncode(v interface{}) []byte {
    //codec.EncodeOptions{Canonical: true}
    //codec.BasicHandle{EncodeOptions: codec.EncodeOptions{Canonical: true}}
    var bh codec.MsgpackHandle
    // canonicalize map key order
    bh.Canonical = true

    // useless
    //bh.RawToString = true
    //bh.SliceType = reflect.TypeOf([]byte(nil))
    //bh.SliceType = reflect.TypeOf(string(""))
    //bh.MapType = reflect.TypeOf(map[string]string(nil))

    var b []byte
    enc := codec.NewEncoderBytes(&b, &bh)
    err := enc.Encode(v)
    if err != nil {
        log.Fatal(err)
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
    mp2 := msgpackEncode(args)
    msgpackDecode(mp2, &content2)
    fmt.Println(args)
    fmt.Println(content2)
    //fmt.Println(mp)
    fmt.Println(mp2)
}
