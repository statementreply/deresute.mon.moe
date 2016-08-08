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
