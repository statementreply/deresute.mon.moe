package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"resource_mgr"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("./unlz4 infile outfile")
	}
	t := resource_mgr.Unlz4(os.Args[1])
	fmt.Println(len(t))
	ioutil.WriteFile(os.Args[2], t, 0644)
}
