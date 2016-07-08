package main
import (
    "fmt"
    "time"
    "os"
    "strconv"
    "path"
    "log"
    "math/rand"
    "io/ioutil"
    "encoding/hex"
    //_ "crypto/aes"
    "crypto/md5"
    //"crypto/cipher"
    //"rijndael_wrapper"
    "apiclient"
    "gopkg.in/yaml.v2"
)

var SECRET_FILE string = "secret.yaml"
var BASE string = path.Dir(os.Args[0])
var RANK_CACHE_DIR string = BASE + "/data/rank/"

func main() {
    rand.Seed(time.Now().Unix())
    secret, _ := ioutil.ReadFile(SECRET_FILE)
    var secret_dict map[string]interface{}
    yaml.Unmarshal(secret, &secret_dict)
    //fmt.Println(secret_dict)

    client := apiclient.NewApiClient(
        int32(secret_dict["user"].(int)),
        int32(secret_dict["viewer_id"].(int)),
        secret_dict["udid"].(string),
        secret_dict["res_ver"].(string),
        []byte(secret_dict["VIEWER_ID_KEY"].(string)),
        []byte(secret_dict["SID_KEY"].(string)))
    sum_tmp := md5.Sum([]byte("All your APIs are belong to us"))
    args := map[string]interface{}{"campaign_data":"",
        "campaign_user": 171780,
        "campaign_sign": hex.EncodeToString(sum_tmp[:]),
        "app_type": 0,}

    check := client.Call("/load/check", args)
    log.Print(check)
    new_res_ver, ok := check["data_headers"].(map[interface{}]interface{})["required_res_ver"]
    if ok {
        s := new_res_ver.(string)
        client.Set_res_ver(s)
        fmt.Println("Update res_ver to ", s)
        time.Sleep(1.3e9)
        check := client.Call("/load/check", args)
        log.Print(check)
    }

    friend_id := 679923520
    if len(os.Args) > 1 {
        friend_id, _ = strconv.Atoi(os.Args[1])
    }
    data := client.Call("/profile/get_profile", map[string]interface{}{"friend_id": friend_id})
    yy, _ := yaml.Marshal(data)
    fmt.Println(string(yy))
}
