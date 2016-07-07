package main
import (
    "fmt"
    "time"
    "os"
    "path"
    "math/rand"
    "io/ioutil"
    "encoding/hex"
    _ "crypto/aes"
    "crypto/md5"
    "crypto/cipher"
    "rijndael_wrapper"
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
    fmt.Println(secret_dict)

    client := apiclient.NewApiClient(
        int32(secret_dict["user"].(int)),
        int32(secret_dict["viewer_id"].(int)),
        secret_dict["udid"].(string),
        secret_dict["res_ver"].(string),
        []byte(secret_dict["VIEWER_ID_KEY"].(string)),
        []byte(secret_dict["SID_KEY"].(string)))
    sum_tmp := md5.Sum([]byte("All your APIs are belong to us"))

    fmt.Println(client.Call(
        "/load/check",
        map[string]interface{}{"campaign_data":"",
        "campaign_user": 171780,
        "campaign_sign": hex.EncodeToString(sum_tmp[:]),
        "app_type": 0,}))
}

func decrypt_cbc(s, iv, key []byte) []byte {
    c0, _ := rijndael_wrapper.NewCipher(key)
    cbc := cipher.NewCBCDecrypter(c0, iv)
    cbc.CryptBlocks(s, s)
    return s
}

func encrypt_cbc(s, iv, key []byte) []byte {
    c0, _ := rijndael_wrapper.NewCipher(key)
    cbc := cipher.NewCBCEncrypter(c0, iv)
    cbc.CryptBlocks(s, s)
    return s
}
