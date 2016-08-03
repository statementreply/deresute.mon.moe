package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"resource_mgr"
	//"time"
	"unicode/utf8"
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
	fmt.Println("master is", r.LoadMaster())
	r.ParseEvent()
	for _, e := range r.EventList {
		//fmt.Println(e.NoticeStart(), e.EventStart(), e.Name())
		fmt.Println(utf8.RuneCountInString(e.Name()), e.Name())
	}
	currentEvent := resource_mgr.FindCurrentEvent(r.EventList)
	if currentEvent != nil {
		fmt.Println(currentEvent.Name())
	}

	// download musicscores
	r.LoadMusic()

	fmt.Println(r.LoadMaster())

	fmt.Println("debug")
	//v/chara_271.acb|3428b3a012082796aeb14d8a0412e602|0|every|
	// "v/chara_271.acb"
	d, err := r.Fetch("dl/resources/High/Sound/Common/v/3428b3a012082796aeb14d8a0412e602")
	// l/song_1023.acb
	d, err = r.Fetch("dl/resources/High/Sound/Common/l/7440496164fa88f65518da9d63601d76")
	fmt.Println("fetched", d, err)
}
