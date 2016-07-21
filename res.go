package main

import (
	"resource_mgr"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	//"time"
)

var CONFIG_FILE = "secret.yaml"

func main() {
	content, err := ioutil.ReadFile(CONFIG_FILE)
	if err != nil {
		log.Fatal(err)
	}
	var config map[string]string
	yaml.Unmarshal(content, &config)

	r := resource_mgr.NewResourceMgr(config["res_ver"], "data/resourcesbeta")
	r.LoadManifest()
	r.ParseEvent()
	for _, e := range r.EventList {
		//log.Println(e)
		log.Println(e.Name())
	}
}
