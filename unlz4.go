package main

import (
	"fmt"
	"resource_mgr"
	"io/ioutil"
	"os"
)

func main() {
	t := resource_mgr.Unlz4(os.Args[0])
	fmt.Println(len(t))
	ioutil.WriteFile(os.Args[1], t, 0644)
}
