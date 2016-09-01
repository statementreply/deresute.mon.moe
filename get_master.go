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
	for i, res_ver := range os.Args {
		if i == 0 {
			continue
		}
		r := resource_mgr.NewResourceMgr(res_ver, "data/resourcesbeta")
		fmt.Println(r.LoadMaster())
	}
}
