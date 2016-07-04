package apiclient
import (
    "rijndael_wrapper"
    "crypto/md5"
    "crypto/sha1"
    "crypto/cipher"
    "math/rand"
    crand "crypto/rand"
    "fmt"
    "strconv"
    "encoding/base64"
    "encoding/hex"
    "gopkg.in/vmihailenco/msgpack.v2"
    "strings"
    "net/http"
    "io/ioutil"
    "gopkg.in/yaml.v2"
)

const BASE = "http://game.starlight-stage.jp"

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
    vid_iv := fmt.Sprintf("%016d%016d", rand.Int63n(1e16), rand.Int63n(1e16))

    args["viewer_id"] = vid_iv + base64.StdEncoding.EncodeToString(Encrypt_cbc([]byte(client.viewer_id_str), []byte(vid_iv), client.VIEWER_ID_KEY))
    fmt.Println("args ", args)
    mp, _ := msgpack.Marshal(args)
    fmt.Println("mp ", string(mp))
    plain := base64.StdEncoding.EncodeToString(mp)
    //var key_tmp [64]byte
    key_tmp := make([]byte, 64)
    _, _ = crand.Read(key_tmp)
    key := []byte(base64.StdEncoding.EncodeToString(key_tmp))
    key = key[:32]
    msg_iv := []byte(strings.Replace(client.udid, "-", "", -1))
    fmt.Println("plain", plain)
    fmt.Println("key", string(key))
    fmt.Println("iv", string(msg_iv))
    body_tmp := Encrypt_cbc([]byte(plain), msg_iv, key)
    body := base64.StdEncoding.EncodeToString([]byte(string(body_tmp) + string(key)))
    fmt.Println("body \n", body)

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
        "User_Id": Lolfuscate(fmt.Sprintf("%d", client.user)),
        "CARRIER": "google",
        "UDID": Lolfuscate(client.udid),
        "App_Ver": "2.0.3",
        "Res_Ver": client.res_ver,
        "Ip_Address": "127.0.0.1",
        "Device_Name": "Nexus 42",
        "X-Unity-Version": "5.1.2f1",
        "SID": hex.EncodeToString(sid_tmp[:]),
        "Graphics_Device_Name": "3dfx Voodoo2 (TM)",
        "Device_Id": hex.EncodeToString(device_id_tmp[:]),
        "Platform_Os_Version": "Android OS 13.3.7 / API-42 (XYZZ1Y/74726f6c6c)",
        "DEVICE": "2",
        "Content-Type": "application/x-www-form-urlencoded", // lies
        "User-Agent": "Dalvik/2.1.0 (Linux; U; Android 13.3.7; Nexus 42 Build/XYZZ1Y)",
        "Accept-Encoding": "identity",
        "Connection": "close",
    }

    fmt.Println(BASE + path)
    yy, _ := yaml.Marshal(&headers)
    fmt.Printf("%v\n", string(yy))
    req, _ := http.NewRequest("POST", BASE + path, strings.NewReader(body))
    for k := range headers {
        req.Header.Set(http.CanonicalHeaderKey(k), headers[k])
        fmt.Println(http.CanonicalHeaderKey(k), headers[k])
    }
    req.Close = true
    hclient := &http.Client{};
    fmt.Printf("%v\n", req)
    //return map[string]interface{}{"1":2}
    resp, _ := hclient.Do(req)
    resp_body, _ := ioutil.ReadAll(resp.Body)
    //var reply []byte
    reply := make([]byte, base64.StdEncoding.DecodedLen(len(resp_body)))
    fmt.Println("resp_body ", string(resp_body))
    n, _ := base64.StdEncoding.Decode(reply, resp_body)
    // trim NULs
    reply = reply[:n]

    plain2 := Decrypt_cbc(reply[:len(reply)-32], msg_iv, reply[len(reply)-32:])
    fmt.Println("plain2", string(plain2))
    mp2 := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
    base64.StdEncoding.Decode(mp2, plain2)
    //var content map[string]interface{}
    var content interface{}
    msgpack.Unmarshal(mp2, &content)

    fmt.Println("content", content)
    return args
}
