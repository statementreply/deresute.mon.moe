package apiclient

import (
	"fmt"
)

func Test1() {
	var args map[string]interface{}
	//var content map[string]interface{}
	var content2 map[string]interface{}
	args = make(map[string]interface{})
	fmt.Println("here")
	args["1"] = 2
	args["2"] = "string"
	args["c"] = map[string]int{"c92": 12}
	fmt.Println("here2")
	// old lib
	// don't use
	//mp, err := msgpack.Marshal(args)
	//msgpack.Unmarshal(mp, &content, nil)
	//fmt.Println(args, content)

	// new lib
	mp2 := MsgpackEncode(args)
	MsgpackDecode(mp2, &content2)
	fmt.Println(args)
	fmt.Println(content2)
	//fmt.Println(mp)
	fmt.Println(mp2)
	return
}
