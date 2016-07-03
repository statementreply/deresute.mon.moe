package rijndael_wrapper
import (
    "rijndael"
)

//const BlockSize = 32


func NewCipher(key []byte) (Cipher_wrapper, error) {
    var key32 [32]byte
    copy(key32[0:32], key)
    c := rijndael.NewCipher(&key32)
    //c_wrapper := new(Cipher_wrapper)
    var c_wrapper Cipher_wrapper
    c_wrapper.c = *c
    return c_wrapper, nil
}

type Cipher_wrapper struct {
    c rijndael.Cipher
}

func (c Cipher_wrapper) BlockSize() int {
    return 32
}

func (c Cipher_wrapper) Decrypt(dst, src []byte) {
    var src32, dst32 [32]byte
    copy(src32[:], src)
    c.c.Decrypt(&dst32, &src32)
    copy(dst[0:32], dst32[:])
}

func (c Cipher_wrapper) Encrypt(dst, src []byte) {
    var src32, dst32 [32]byte
    copy(src32[:], src)
    c.c.Encrypt(&dst32, &src32)
    copy(dst[0:32], dst32[:])
}
