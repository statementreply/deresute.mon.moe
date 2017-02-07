// Copyright 2016 GUO Yixuan <culy.gyx@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
