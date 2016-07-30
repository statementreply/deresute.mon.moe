package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"resource_mgr"
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
	r.LoadMaster()
	r.ParseEvent()
	for _, e := range r.EventList {
		//log.Println(e)
		log.Println(e.Name())
	}
	currentEvent := resource_mgr.FindCurrentEvent(r.EventList)
	log.Println(currentEvent.Name())

	r.LoadMusic()

	fmt.Println(r.LoadMaster())

	fmt.Println("debug")
	//v/chara_271.acb|3428b3a012082796aeb14d8a0412e602|0|every| 
	// "v/chara_271.acb"
	d, err := r.Fetch("dl/resources/High/Sound/Common/v/3428b3a012082796aeb14d8a0412e602")
	fmt.Println("fetched", d, err)
}
