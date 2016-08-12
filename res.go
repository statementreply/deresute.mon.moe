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

	log.Println("res_ver", config["res_ver"])
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

	// chara_179_00_face_01.unity3d
	d, err = r.FetchLz4("dl/resources/High/AssetBundles/Android/286775f14f9a9331481535d72c5ede24")
	// v/card_300029.acb v/card_300030.acb v/chara_268.acb
	d, err = r.Fetch("dl/resources/High/Sound/Common/v/1d0415e9142dfc017c65268be1b851f4")
	fmt.Println(d, err)
	d, err = r.Fetch("dl/resources/High/Sound/Common/v/0f157ae68385aea380211cfb9328b971")
	fmt.Println(d, err)
	d, err = r.Fetch("dl/resources/High/Sound/Common/v/fd52db77fbb15bd90d0e5573219e12d7")
	fmt.Println(d, err)
}
