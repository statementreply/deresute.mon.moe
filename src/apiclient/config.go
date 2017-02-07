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
	"io/ioutil"
)

type Config struct {
	// api, const
	SID_KEY       string `yaml:"SID_KEY"`
	VIEWER_ID_KEY string `yaml:"VIEWER_ID_KEY"`
	// api, global update
	ResVer string `yaml:"res_ver"`
	AppVer string `yaml:"app_ver"`
	// api, per user
	UDID     string `yaml:"udid"`
	User     uint32 `yaml:"user"`
	ViewerID uint32 `yaml:"viewer_id"`
	// audio, const
	HCA_KEY1 uint32 `yaml:"HCA_KEY1"`
	HCA_KEY2 uint32 `yaml:"HCA_KEY2"`
	// twitter, per user
	TwConsumerKey       string `yaml:"twitter_consumer_key"`
	TwConsumerSecret    string `yaml:"twitter_consumer_secret"`
	TwAccessToken       string `yaml:"twitter_access_token"`
	TwAccessTokenSecret string `yaml:"twitter_access_token_secret"`
	// twitter, internal
	TwDummy int `yaml:"twitter_dummy"`
}

func ParseConfig(filename string) *Config {
	var conf Config

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}
	err = yaml.Unmarshal(content, &conf)
	if err != nil {
		return nil
	}
	return &conf
}
