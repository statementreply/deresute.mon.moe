package main

import (
	"fmt"
	"os"
	"resource_mgr"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("res_ver needed")
		return
	}
	res_ver := os.Args[1]
	r := resource_mgr.NewResourceMgr(res_ver, "data/resourcesbeta")
	fmt.Println(r.LoadManifest())
}
