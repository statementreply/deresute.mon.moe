package main

import (
	"apiclient"
	"fmt"
	"io/ioutil"
	"os"
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
	content := apiclient.DecodeBody(body, msg_iv)
	fmt.Println(content)
}
