package main
import (
    "fmt"
    "os"
    "io/ioutil"
    "encoding/base64"
    "apiclient"
    "gopkg.in/vmihailenco/msgpack.v2"
    "strings"
)

func main() {
    var body []byte
    msg_iv := os.Args[1]
    if len(os.Args) >= 3 {
        fmt.Println(os.Args)
        body, _ = ioutil.ReadFile(os.Args[2])
    } else {
        body, _ = ioutil.ReadAll(os.Stdin)
    }
    fmt.Println(body)
    resp_body := body

    //var reply []byte
    reply := make([]byte, base64.StdEncoding.DecodedLen(len(resp_body)))
    fmt.Println("resp_body ", string(resp_body))
    n, _ := base64.StdEncoding.Decode(reply, resp_body)
    print("written", n, "\n")
    print("replylen", len(reply), "\n")
    reply = reply[:n]
    //fmt.Println("reply", string(reply))

    msg_iv = strings.Replace(msg_iv, "-", "", -1)
    fmt.Println("msg_iv ", msg_iv)
    fmt.Println("len", len(msg_iv))
    fmt.Println("key", string(reply[len(reply)-32-1:]))
    plain2 := apiclient.Decrypt_cbc(reply[:len(reply)-32-1], []byte(msg_iv), reply[len(reply)-32-1:])
    fmt.Println("plain2", string(plain2))
    mp := make([]byte, base64.StdEncoding.DecodedLen(len(plain2)))
    base64.StdEncoding.Decode(mp, plain2)
    fmt.Println("mp", mp)
    //var content map[string]interface{}
    var content interface{}
    msgpack.Unmarshal(mp, &content)

    fmt.Println("content", content)

}
