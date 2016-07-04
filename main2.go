package main
import (
    _ "crypto/aes"
    "crypto/cipher"
    "fmt"
    "rijndael_wrapper"
    "apiclient"
    "time"
    "math/rand"
    "encoding/hex"
    "crypto/md5"
    "gopkg.in/yaml.v2"
    "io/ioutil"
)

func main() {
    fmt.Println("test")
    key := []byte("9999111100002222abcdefghabcdefgh")
    data := []byte("92938882919929001923918231234567")
    iv:=  []byte("92938882919929009293888291992900")

    fmt.Println(data)
    //dst := make([]byte, 12)
    fmt.Println(decrypt_cbc(data, iv, key))
    fmt.Println(encrypt_cbc(data, iv, key))

    rand.Seed(time.Now().Unix())
    fus := apiclient.Lolfuscate("1212930123")
    fmt.Println(fus)
    fmt.Println(apiclient.Unlolfuscate(fus))

    SECRET_FILE := "secret.yaml"
    secret, _ := ioutil.ReadFile(SECRET_FILE)
    //var secret_dict map[string]interface{}
    var secret_dict map[string]interface{}
    yaml.Unmarshal(secret, &secret_dict)
    fmt.Println(secret_dict)

    client := apiclient.NewApiClient(int32(secret_dict["user"].(int)), int32(secret_dict["viewer_id"].(int)),
         secret_dict["udid"].(string), secret_dict["res_ver"].(string), []byte(secret_dict["VIEWER_ID_KEY"].(string)), []byte(secret_dict["SID_KEY"].(string)))
    sum_tmp := md5.Sum([]byte("All your APIs are belong to us"))

    client.Call("/load/check",  map[string]interface{}{"campaign_data":"",
    "campaign_user": 171780,
    "campaign_sign": hex.EncodeToString(sum_tmp[:]),
    "app_type": 0,})
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
