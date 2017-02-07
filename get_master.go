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
