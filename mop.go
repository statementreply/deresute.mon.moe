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

package main

import (
	"apiclient"
	"log"
	"time"
)

var SECRET_FILE = "secret.yaml"

func main() {
	client := apiclient.NewApiClientFromConfig(SECRET_FILE)
	client.LoadCheck()
	time.Sleep(500 * time.Millisecond)

	args := map[string]interface{}{
		"invalid_param": 19,
	}
	val := client.Call("/producer_rank/index", args)
	time.Sleep(500 * time.Millisecond)
	log.Printf("args %#v\n", args)
	log.Printf("top %v\n\n", val)

	// param: page, back_mo_flag
	// valuetype Stage.ProducerFanRanking/eProducerRankingType: 0, 1, 2
	// -1 => back_mo_flag
	args = map[string]interface{}{
		"page":         int32(4),
		"back_mo_flag": int32(0), // -1? 0 or 1
	}
	val = client.Call("/producer_rank/mo_p_ranking_list", args)
	log.Printf("args %v\n", args)
	time.Sleep(500 * time.Millisecond)
	log.Printf("rank %#v\n", val)

	args = map[string]interface{}{}
	val = client.Call("/producer_rank/mo_p_rank_data", args)
	log.Printf("args %#v\n", args)
	log.Printf("info %v\n\n", val)
}

/* new api
-       IL_xxxx:  ldc.i4.s 0x5b
+       IL_xxxx:  ldc.i4.s 0x5e
        IL_xxxx:  ldstr "emblem/edit"
		        IL_xxxx:  callvirt instance void class [mscorlib]System.Collections.Generic.Dictionary`2<valuetype ApiType/Type, string>::Add(!0, !1)
				        IL_xxxx:  ldloc.0
						-       IL_xxxx:  ldc.i4.s 0x5c
						+       IL_xxxx:  ldc.i4.s 0x5f
						+       IL_xxxx:  ldstr "producer_rank/index"
						+       IL_xxxx:  callvirt instance void class [mscorlib]System.Collections.Generic.Dictionary`2<valuetype ApiType/Type, string>::Add(!0, !1)
						+       IL_xxxx:  ldloc.0
						+       IL_xxxx:  ldc.i4.s 0x60
						+       IL_xxxx:  ldstr "producer_rank/mo_p_ranking_list"
						+       IL_xxxx:  callvirt instance void class [mscorlib]System.Collections.Generic.Dictionary`2<valuetype ApiType/Type, string>::Add(!0, !1)
						+       IL_xxxx:  ldloc.0
						+       IL_xxxx:  ldc.i4.s 0x61
						+       IL_xxxx:  ldstr "producer_rank/mo_p_rank_data"
						+       IL_xxxx:  callvirt instance void class [mscorlib]System.Collections.Generic.Dictionary`2<valuetype ApiType/Type, string>::Add(!0, !1)
						+       IL_xxxx:  ldloc.0
						+       IL_xxxx:  ldc.i4.s 0x62

*/
