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
	"gopkg.in/yaml.v2"
)

func SampleRanking() map[string]interface{} {
	var data map[string]interface{}
	var yamlStr = `data:
    - leader_card_info:
        card_id: 200184
        exp: 213600
        level: 90
        love: 600
        skill_level: 10
        step: 0
      rank: 1
      score: 126448
      user_info:
        comment: !!python/str 'ます'
        create_time: '2015-09-16 10:48:25'
        emblem_ex_value: 0
        emblem_id: 1720102
        fan: 1256407
        fan_type_1: 1424947
        fan_type_2: 1756923
        fan_type_3: 6983606
        last_login_time: '2016-07-21 09:28:16'        level: 300
        name: test
        producer_rank: 8
        viewer_id: 123000625
`
	yaml.Unmarshal([]byte(yamlStr), &data)
	return data
}
