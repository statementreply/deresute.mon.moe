package main
import (
    _ "crypto/aes"
    "crypto/cipher"
    "fmt"
    "rijndael_wrapper"
    "apiclient"
    "time"
    "math/rand"
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
