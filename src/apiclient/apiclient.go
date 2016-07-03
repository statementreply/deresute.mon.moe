package apiclient
import (
    _ "rijndael_wrapper"
    "math/rand"
    "fmt"
    "strconv"
)

const BASE = "http://game.starlight-stage.jp"

type ApiClient struct {
    user int32
    viewer_id int32
    udid string
    sid string
    res_ver string
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


func NewApiClient(user, viewer_id int32, udid, res_ver string) *ApiClient {
    client := new(ApiClient)
    client.user = user
    client.viewer_id = viewer_id
    client.udid = udid
    client.res_ver = res_ver
    client.sid = ""
    return client
}


