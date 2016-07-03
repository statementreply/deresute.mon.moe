package main
import (
    _ "crypto/aes"
    "crypto/cipher"
    "fmt"
    "rijndael"
)

func main() {
    fmt.Println("test")
    var key, data, iv [32]byte
    copy(key[0:32], []byte("9999111100002222abcdefghabcdefgh"))
    copy(data[0:32],[]byte("92938882919929001923918231234567"))
    copy(iv[0:32],  []byte("92938882919929009293888291992900"))

    fmt.Println(data)
    //dst := make([]byte, 12)
    fmt.Println(decrypt_cbc(data, iv, key))
    fmt.Println(encrypt_cbc(data, iv, key))
}

func decrypt_cbc(s, iv, key [32]byte) []byte {
    c0 := rijndael.NewCipher(&key)
    cbc := cipher.NewCBCDecrypter(c0, iv)
    cbc.CryptBlocks(s, s)
    return s
}


func encrypt_cbc(s, iv, key [32]byte) []byte {
    c0 := rijndael.NewCipher(&key)
    cbc := cipher.NewCBCEncrypter(c0, iv)
    cbc.CryptBlocks(s, s)
    return s
}
