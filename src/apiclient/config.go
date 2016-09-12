package apiclient

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	// api, const
	SID_KEY string `yaml:"SID_KEY"`
	VIEWER_ID_KEY string `yaml:"VIEWER_ID_KEY"`
	// api, global update
	ResVer string `yaml:"res_ver"`
	AppVer string `yaml:"app_ver"`
	// api, per user
	UDID string `yaml:"udid"`
	User uint32 `yaml:"user"`
	ViewerID uint32 `yaml:"viewer_id"`
	// audio, const
	HCA_KEY1 uint32 `yaml:"HCA_KEY1"`
	HCA_KEY2 uint32 `yaml:"HCA_KEY2"`
	// twitter, per user
	TwConsumerKey string `yaml:"twitter_consumer_key"`
	TwConsumerSecret string `yaml:"twitter_consumer_secret"`
	TwAccessToken string `yaml:"twitter_access_token"`
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
